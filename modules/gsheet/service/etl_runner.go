package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/repository"
)

const etlDir = "database/clickhouse/etl"

// ETLRunner 按顺序执行 ODS→DIM→DWD→DWS 的 ClickHouse 内 ETL。
type ETLRunner struct {
	etlRepo *repository.EtlRepo
}

func NewETLRunner(etlRepo *repository.EtlRepo) *ETLRunner {
	return &ETLRunner{etlRepo: etlRepo}
}

// ETLParams ETL 执行参数。
type ETLParams struct {
	DateFrom time.Time // 起始日期（含）
	DateTo   time.Time // 结束日期（含）
}

// RunAll 依次执行：ODS→DIM、ODS→DWD、DWD→DWS(product/team/bu)
func (r *ETLRunner) RunAll(ctx context.Context, p ETLParams) error {
	steps := []struct {
		name      string
		sqlFile   string
		dropTables []string // MergeTree 表需要先 DROP PARTITION
	}{
		{
			name:    "ODS → DIM",
			sqlFile: etlDir + "/01_ods_to_dim.sql",
		},
		{
			name:    "ODS → DWD",
			sqlFile: etlDir + "/02_ods_to_dwd.sql",
		},
		{
			name:       "DWD → DWS product",
			sqlFile:    etlDir + "/03_dwd_to_dws_product.sql",
			dropTables: []string{"dws_product_daily"},
		},
		{
			name:       "DWD → DWS team",
			sqlFile:    etlDir + "/04_dwd_to_dws_team.sql",
			dropTables: []string{"dws_team_daily"},
		},
		{
			name:       "DWD → DWS bu",
			sqlFile:    etlDir + "/05_dwd_to_dws_bu.sql",
			dropTables: []string{"dws_bu_daily"},
		},
	}

	params := map[string]interface{}{
		"date_from": p.DateFrom,
		"date_to":   p.DateTo,
	}

	partitions := monthPartitions(p.DateFrom, p.DateTo)

	for _, step := range steps {
		log.Printf("[etl] step: %s", step.name)

		// DROP PARTITION（MergeTree 重算幂等）
		for _, tbl := range step.dropTables {
			for _, part := range partitions {
				if err := r.etlRepo.DropPartition(ctx, tbl, part); err != nil {
					return fmt.Errorf("[etl] drop partition %s/%s: %w", tbl, part, err)
				}
				log.Printf("[etl]   dropped partition %s from %s", part, tbl)
			}
		}

		if err := r.etlRepo.ExecETL(ctx, step.sqlFile, params); err != nil {
			return fmt.Errorf("[etl] %s: %w", step.name, err)
		}
		log.Printf("[etl] ✓ %s done", step.name)
	}
	return nil
}

// monthPartitions 返回 [dateFrom, dateTo] 区间内涉及的所有月份分区键（格式 "YYYYMM"）。
func monthPartitions(from, to time.Time) []string {
	var parts []string
	cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC)
	for !cur.After(end) {
		parts = append(parts, fmt.Sprintf("%d%02d", cur.Year(), cur.Month()))
		cur = cur.AddDate(0, 1, 0)
	}
	return parts
}

// buildETLArgs 把 map[string]interface{} 转为 clickhouse NamedValue 切片。
func buildETLArgs(params map[string]interface{}) []driver.NamedValue {
	args := make([]driver.NamedValue, 0, len(params))
	for k, v := range params {
		args = append(args, driver.NamedValue{Name: k, Value: v})
	}
	return args
}

// MonthPartitionsExported 导出 monthPartitions 供包外测试。
func MonthPartitionsExported(from, to time.Time) []string { return monthPartitions(from, to) }
