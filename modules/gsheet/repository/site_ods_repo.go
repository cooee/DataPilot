package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/dto"
)

type SiteODSRepo struct {
	conn driver.Conn
}

func NewSiteODSRepo(conn driver.Conn) *SiteODSRepo {
	return &SiteODSRepo{conn: conn}
}

const siteODSInsertSQL = "INSERT INTO ods_site_product_daily (" +
	"`日期`,`产品名称`,`产品编号`,`小组`,`部门`," +
	"`日活跃数`,`日活环比`,`日导量新增`,`日导量新增环比`,`日导量充值`,`日导量充值环比`," +
	"product_type,_source,_src_row_no,_ingested_at)"

func (r *SiteODSRepo) BatchInsert(ctx context.Context, rows []dto.SiteODSRow) error {
	if len(rows) == 0 {
		return nil
	}
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	batch, err := r.conn.PrepareBatch(ctx2, siteODSInsertSQL)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, row := range rows {
		if err := batch.Append(
			row.Date, row.ProductName, row.ProductCode, row.Team, row.BusinessUnit,
			row.DAU, row.DAUChain, row.LeadNewCnt, row.LeadNewChain,
			row.LeadRechargeAmt, row.LeadRechargeChain,
			row.ProductType, row.Source, row.SrcRowNo, row.IngestedAt,
		); err != nil {
			return fmt.Errorf("append row %d: %w", row.SrcRowNo, err)
		}
	}
	return batch.Send()
}
