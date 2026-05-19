package service

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/dto"
	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/mapper"
	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/repository"
)

const (
	// defaultBatchSize 每批写入 ClickHouse 的行数。
	defaultBatchSize = 500
	// maxDirtyRatio 超过此比例的解析错误行则中止整批写入（数据质量阈值）。
	maxDirtyRatio = 0.05
)

// ODSLoader 把 Google Sheet CSV 整批写入 ODS 层。
type ODSLoader struct {
	fetcher   *CSVFetcher
	odsRepo   *repository.ODSRepo
	batchSize int
}

func NewODSLoader(fetcher *CSVFetcher, odsRepo *repository.ODSRepo) *ODSLoader {
	return &ODSLoader{
		fetcher:   fetcher,
		odsRepo:   odsRepo,
		batchSize: defaultBatchSize,
	}
}

// LoadResult 同步结果摘要。
type LoadResult struct {
	Total    int
	Inserted int
	Skipped  int
	Warnings []dto.ParseWarning
}

// Load 拉取 spreadsheetID/gid 对应的 Sheet，解析并写入 ODS。
// source 用于 _source 字段，可传空（自动从 spreadsheetID 推断）。
func (l *ODSLoader) Load(ctx context.Context, spreadsheetID string, gid int, source string) (*LoadResult, error) {
	if source == "" {
		source = BuildSource(spreadsheetID, gid)
	}

	rows, err := l.fetcher.FetchCSV(spreadsheetID, gid)
	if err != nil {
		return nil, fmt.Errorf("fetch csv: %w", err)
	}
	if len(rows) == 0 {
		return &LoadResult{}, nil
	}

	// 校验表头
	if err := validateHeaders(rows[0]); err != nil {
		return nil, err
	}

	dataRows := rows[1:]
	result := &LoadResult{Total: len(dataRows)}

	var (
		batch    []dto.ODSRow
		errCount int
	)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := l.odsRepo.BatchInsert(ctx, batch); err != nil {
			return fmt.Errorf("batch insert: %w", err)
		}
		result.Inserted += len(batch)
		batch = batch[:0]
		return nil
	}

	for i, row := range dataRows {
		rowNo := i + 1
		odsRow, warns, err := mapper.MapRow(row, rowNo, source)
		if err != nil {
			errCount++
			log.Printf("[ods_loader] skip row %d: %v", rowNo, err)
			result.Skipped++
			continue
		}
		result.Warnings = append(result.Warnings, warns...)
		batch = append(batch, odsRow)

		if len(batch) >= l.batchSize {
			// 脏数据比例检查（超过阈值则中止）
			if dirtyRatio(errCount, rowNo) > maxDirtyRatio {
				return nil, fmt.Errorf("dirty ratio %.1f%% exceeds threshold %.1f%%; abort at row %d",
					dirtyRatio(errCount, rowNo)*100, maxDirtyRatio*100, rowNo)
			}
			if err := flush(); err != nil {
				return nil, err
			}
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return result, nil
}

// validateHeaders 校验 CSV 第一行与 ExpectedHeaders 完全匹配。
func validateHeaders(header []string) error {
	if len(header) < len(mapper.ExpectedHeaders) {
		return fmt.Errorf("header has %d cols, need %d", len(header), len(mapper.ExpectedHeaders))
	}
	var mismatches []string
	for i, expected := range mapper.ExpectedHeaders {
		if strings.TrimSpace(header[i]) != expected {
			mismatches = append(mismatches, fmt.Sprintf("col[%d]: got %q, want %q", i, header[i], expected))
		}
	}
	if len(mismatches) > 0 {
		return fmt.Errorf("header mismatch:\n  %s", strings.Join(mismatches, "\n  "))
	}
	return nil
}

func dirtyRatio(errCount, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(errCount) / float64(total)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BuildSource 生成 _source 字段默认值（供测试和 CLI 复用）。
func BuildSource(spreadsheetID string, gid int) string {
	return fmt.Sprintf("gsheet:%s:gid%d", spreadsheetID[:min(8, len(spreadsheetID))], gid)
}

// --- 仅供测试的导出包装 ---

// ValidateHeadersExported 导出 validateHeaders 供包外测试。
func ValidateHeadersExported(header []string) error { return validateHeaders(header) }

// DirtyRatioExported 导出 dirtyRatio 供包外测试。
func DirtyRatioExported(errCount, total int) float64 { return dirtyRatio(errCount, total) }

// MinExported 导出 min 供包外测试。
func MinExported(a, b int) int { return min(a, b) }
