package service

import (
	"context"
	"fmt"
	"log"
)

const siteETLDir = "database/clickhouse/etl/site"

// RunSiteAll 执行站点产品 ODS→DIM→DWD→DWS ETL。
func (r *ETLRunner) RunSiteAll(ctx context.Context, p ETLParams) error {
	steps := []struct {
		name       string
		sqlFile    string
		dropTables []string
	}{
		{name: "site ODS → DIM", sqlFile: siteETLDir + "/01_ods_to_dim.sql"},
		{name: "site ODS → DWD", sqlFile: siteETLDir + "/02_ods_to_dwd.sql"},
		{
			name:       "site DWD → DWS",
			sqlFile:    siteETLDir + "/03_dwd_to_dws.sql",
			dropTables: []string{"dws_site_product_daily"},
		},
	}

	params := map[string]interface{}{
		"date_from": p.DateFrom,
		"date_to":   p.DateTo,
	}
	partitions := monthPartitions(p.DateFrom, p.DateTo)

	for _, step := range steps {
		log.Printf("[etl:site] step: %s", step.name)
		for _, tbl := range step.dropTables {
			for _, part := range partitions {
				if err := r.etlRepo.DropPartition(ctx, tbl, part); err != nil {
					return fmt.Errorf("[etl:site] drop partition %s/%s: %w", tbl, part, err)
				}
				log.Printf("[etl:site]   dropped partition %s from %s", part, tbl)
			}
		}
		if err := r.etlRepo.ExecETL(ctx, step.sqlFile, params); err != nil {
			return fmt.Errorf("[etl:site] %s: %w", step.name, err)
		}
		log.Printf("[etl:site] ✓ %s done", step.name)
	}
	return nil
}
