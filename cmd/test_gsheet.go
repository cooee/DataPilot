package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/pkg/constants"
	"github.com/samber/do"
	"google.golang.org/api/sheets/v4"
)

// 默认链接，可通过命令行第二个参数覆盖（支持完整 URL 或纯 ID）
//
//	go run cmd/main.go --test:gsheet
//	go run cmd/main.go --test:gsheet 1sJZBMAHBa3QtmSKQ8oLGaiC_w-8kloGJizRNHnbQ-jw
//	go run cmd/main.go --test:gsheet "https://docs.google.com/spreadsheets/d/<id>/edit"
const defaultTestSpreadsheetID = "1sJZBMAHBa3QtmSKQ8oLGaiC_w-8kloGJizRNHnbQ-jw"

func runTestGSheet(injector *do.Injector) {
	spreadsheetID := parseTestSpreadsheetID(os.Args)

	svc := do.MustInvokeNamed[*sheets.Service](injector, constants.GoogleSheetsService)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	meta, err := svc.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
	if err != nil {
		log.Fatalf("[test:gsheet] failed to fetch spreadsheet meta (id=%s): %v", spreadsheetID, err)
	}
	if len(meta.Sheets) == 0 {
		log.Fatalf("[test:gsheet] spreadsheet %s has no sheets", spreadsheetID)
	}

	first := meta.Sheets[0]
	title := first.Properties.Title
	gp := first.Properties.GridProperties

	log.Printf("[test:gsheet] spreadsheet: %q (id=%s)", meta.Properties.Title, spreadsheetID)
	log.Printf("[test:gsheet] sub-sheets: %d, picking the first one", len(meta.Sheets))
	log.Printf("[test:gsheet] first sheet: %q  rows=%d cols=%d", title, gp.RowCount, gp.ColumnCount)

	// 用单引号包住 title，避免空格/中文/破折号导致解析失败
	rangeA1 := fmt.Sprintf("'%s'", title)
	resp, err := svc.Spreadsheets.Values.
		Get(spreadsheetID, rangeA1).
		Context(ctx).Do()
	if err != nil {
		log.Fatalf("[test:gsheet] failed to read values: %v", err)
	}

	log.Printf("[test:gsheet] got %d rows", len(resp.Values))
	if len(resp.Values) == 0 {
		return
	}

	header := resp.Values[0]
	log.Printf("[test:gsheet] header (%d cols): %v", len(header), header)

	// 打印前 5 行数据
	dataRows := resp.Values[1:]
	preview := 5
	if len(dataRows) < preview {
		preview = len(dataRows)
	}
	for i := 0; i < preview; i++ {
		log.Printf("[test:gsheet] row[%d]: %v", i+1, dataRows[i])
	}
	if len(dataRows) > preview {
		log.Printf("[test:gsheet] ... (%d more rows omitted)", len(dataRows)-preview)
	}
}

// parseTestSpreadsheetID 从 os.Args 中找到 --test:gsheet 后面紧跟的参数；
// 如果没传则返回默认 ID。同时支持把完整 URL 解析成 ID。
func parseTestSpreadsheetID(args []string) string {
	for i := 1; i < len(args); i++ {
		if args[i] != "--test:gsheet" {
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			return extractSpreadsheetID(args[i+1])
		}
	}
	return defaultTestSpreadsheetID
}

func extractSpreadsheetID(s string) string {
	const marker = "/spreadsheets/d/"
	if idx := strings.Index(s, marker); idx >= 0 {
		rest := s[idx+len(marker):]
		if slash := strings.IndexAny(rest, "/?#"); slash >= 0 {
			rest = rest[:slash]
		}
		return rest
	}
	return s
}

// runTestGSheetPublic 不走 Google API，直接拉 docs.google.com 的 CSV 导出 URL，
// 只对「任何知道链接的人 - 查看者」的公开 Sheet 有效，零凭证依赖。
//
//	go run ./cmd --test:gsheet:public
//	go run ./cmd --test:gsheet:public "https://docs.google.com/spreadsheets/d/<id>/edit?gid=123"
func runTestGSheetPublic() {
	spreadsheetID := parseTestSpreadsheetID(os.Args)
	gid := parseTestGid(os.Args)

	csvURL := fmt.Sprintf(
		"https://docs.google.com/spreadsheets/d/%s/export?format=csv&gid=%d",
		spreadsheetID, gid,
	)
	log.Printf("[test:gsheet:public] GET %s", csvURL)

	// 公开 sheet 的正常下载流程是：
	//   docs.google.com/.../export  --307-->  *.googleusercontent.com/export/...  --200-->  text/csv
	// 而未公开时会被 302 到 accounts.google.com/ServiceLogin。
	// 所以策略是：允许跟随到 google 自家域名，但禁止跟随到登录页。
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			host := req.URL.Host
			if strings.HasPrefix(host, "accounts.google.") {
				return fmt.Errorf("redirected to login page (%s); sheet is not publicly accessible", req.URL)
			}
			return nil
		},
	}

	resp, err := client.Get(csvURL)
	if err != nil {
		log.Fatalf("[test:gsheet:public] sheet 可能不是公开可访问的，请把共享权限改为「知道链接的任何人 - 查看者」: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		log.Fatalf("[test:gsheet:public] http %d: %s", resp.StatusCode, string(body))
	}

	// 再次兜底：如果最终落到 HTML 页（而不是 CSV），说明走的不是导出链路
	if ct := resp.Header.Get("Content-Type"); ct != "" &&
		!strings.HasPrefix(ct, "text/csv") &&
		!strings.HasPrefix(ct, "application/csv") {
		log.Fatalf("[test:gsheet:public] unexpected Content-Type %q (final URL=%s)；通常意味着 sheet 没公开",
			ct, resp.Request.URL)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("[test:gsheet:public] read body: %v", err)
	}
	// 去掉 Google 偶尔加的 UTF-8 BOM
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})

	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1 // 容忍不同行的列数差异（尾部空列）
	rows, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("[test:gsheet:public] csv parse error: %v", err)
	}

	log.Printf("[test:gsheet:public] got %d rows from gid=%d", len(rows), gid)
	if len(rows) == 0 {
		return
	}

	header := rows[0]
	log.Printf("[test:gsheet:public] header (%d cols): %v", len(header), header)

	dataRows := rows[1:]
	preview := 5
	if len(dataRows) < preview {
		preview = len(dataRows)
	}
	for i := 0; i < preview; i++ {
		log.Printf("[test:gsheet:public] row[%d]: %v", i+1, dataRows[i])
	}
	if len(dataRows) > preview {
		log.Printf("[test:gsheet:public] ... (%d more rows omitted)", len(dataRows)-preview)
	}
}

// parseTestGid 从命令行参数中带的 URL 里抓 gid，没有则返回 0
func parseTestGid(args []string) int {
	for i := 1; i < len(args); i++ {
		if args[i] != "--test:gsheet" && args[i] != "--test:gsheet:public" {
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			return extractGid(args[i+1])
		}
	}
	return 0
}

var gidRegexp = regexp.MustCompile(`[?#&]gid=(\d+)`)

func extractGid(s string) int {
	if m := gidRegexp.FindStringSubmatch(s); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}
