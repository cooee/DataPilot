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

const defaultSiteGID = 553168897

// SiteODSLoader 拉取站点产品 Sheet 写入 ods_site_product_daily。
type SiteODSLoader struct {
	fetcher   *CSVFetcher
	siteRepo  *repository.SiteODSRepo
	batchSize int
}

func NewSiteODSLoader(fetcher *CSVFetcher, siteRepo *repository.SiteODSRepo) *SiteODSLoader {
	return &SiteODSLoader{
		fetcher:   fetcher,
		siteRepo:  siteRepo,
		batchSize: defaultBatchSize,
	}
}

// DefaultSiteGID 站点产品 tab gid。
func DefaultSiteGID() int { return defaultSiteGID }

// Load 同步 gid=553168897（或传入 gid）的站点产品 Sheet。
func (l *SiteODSLoader) Load(ctx context.Context, spreadsheetID string, gid int, source string) (*LoadResult, error) {
	if gid == 0 {
		gid = defaultSiteGID
	}
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
	if err := validateSiteHeaders(rows[0]); err != nil {
		return nil, err
	}

	dataRows := rows[1:]
	result := &LoadResult{Total: len(dataRows)}

	var batch []dto.SiteODSRow
	errCount := 0

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := l.siteRepo.BatchInsert(ctx, batch); err != nil {
			return fmt.Errorf("batch insert: %w", err)
		}
		result.Inserted += len(batch)
		batch = batch[:0]
		return nil
	}

	for i, row := range dataRows {
		rowNo := i + 1
		odsRow, warns, err := mapper.MapSiteRow(row, rowNo, source)
		if err != nil {
			errCount++
			log.Printf("[site_ods_loader] skip row %d: %v", rowNo, err)
			result.Skipped++
			continue
		}
		result.Warnings = append(result.Warnings, warns...)
		batch = append(batch, odsRow)

		if len(batch) >= l.batchSize {
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

func validateSiteHeaders(header []string) error {
	if len(header) < len(mapper.ExpectedSiteHeaders) {
		return fmt.Errorf("header has %d cols, need %d", len(header), len(mapper.ExpectedSiteHeaders))
	}
	var mismatches []string
	for i, expected := range mapper.ExpectedSiteHeaders {
		if strings.TrimSpace(header[i]) != expected {
			mismatches = append(mismatches, fmt.Sprintf("col[%d]: got %q, want %q", i, header[i], expected))
		}
	}
	if len(mismatches) > 0 {
		return fmt.Errorf("site header mismatch:\n  %s", strings.Join(mismatches, "\n  "))
	}
	return nil
}
