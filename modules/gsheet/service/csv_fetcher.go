package service

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CSVFetcher 负责从公开 Google Sheet 拉取 CSV 内容（无需凭证）。
// Sheet 需设置为「知道链接的任何人 - 查看者」可访问。
type CSVFetcher struct {
	client *http.Client
}

func NewCSVFetcher() *CSVFetcher {
	c := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			// 拒绝跳转到 Google 登录页
			if strings.HasPrefix(req.URL.Host, "accounts.google.") {
				return fmt.Errorf("redirected to login (%s): sheet is not publicly accessible", req.URL)
			}
			return nil
		},
	}
	return &CSVFetcher{client: c}
}

// FetchCSV 下载并解析指定 spreadsheetID + gid 的 CSV，返回所有行（含表头）。
func (f *CSVFetcher) FetchCSV(spreadsheetID string, gid int) ([][]string, error) {
	url := fmt.Sprintf(
		"https://docs.google.com/spreadsheets/d/%s/export?format=csv&gid=%d",
		spreadsheetID, gid,
	)
	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch csv (id=%s gid=%d): %w", spreadsheetID, gid, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "text/csv") && !strings.HasPrefix(ct, "application/csv") {
		return nil, fmt.Errorf("unexpected Content-Type %q (final=%s); sheet may not be public",
			ct, resp.Request.URL)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) // strip UTF-8 BOM

	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = -1 // 容忍各行列数不一致
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv parse: %w", err)
	}
	return rows, nil
}
