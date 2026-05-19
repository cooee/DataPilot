package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/dto"
)

type ODSRepo struct {
	conn driver.Conn
}

func NewODSRepo(conn driver.Conn) *ODSRepo {
	return &ODSRepo{conn: conn}
}

const odsInsertSQL = "INSERT INTO ods_product_daily_report (" +
	"`日期`,`产品名称`,`产品编号`,`小组`,`部门`," +
	"`日活`,`日活环比`,`总充值`,`总充值环比`," +
	"`自然充值`,`自然充值环比`,`渠道充值`,`渠道充值环比`," +
	"`内导充值`,`内导充值环比`,`总新增`,`总新增环比`," +
	"`自然新增`,`自然新增环比`,`渠道新增`,`渠道新增环比`," +
	"`内导新增`,`内导新增环比`,`新充人数`,`老充人数`," +
	"`新用户充值`,`老用户充值`,`充值单数`,`付款成功率`," +
	"`次留率`,`3留率`,`7留率`," +
	"`下载页访问数`,`下载页点击数`,`落地页下载率`," +
	"`总转化`,`新充转化`,`老充转化`,`老用户日活`,`ARPPU`," +
	"_source,_src_row_no,_ingested_at)"

// BatchInsert 把 rows 用 PrepareBatch 批量写入 ods_product_daily_report
func (r *ODSRepo) BatchInsert(ctx context.Context, rows []dto.ODSRow) error {
	if len(rows) == 0 {
		return nil
	}
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	batch, err := r.conn.PrepareBatch(ctx2, odsInsertSQL)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, row := range rows {
		if err := batch.Append(
			row.Date, row.ProductName, row.ProductCode, row.Team, row.BusinessUnit,
			row.DAU, row.DAUChain,
			row.RechargeTotal, row.RechargeTotalChain,
			row.RechargeOrganic, row.RechargeOrganicChain,
			row.RechargeChannel, row.RechargeChannelChain,
			row.RechargeInternal, row.RechargeInternalChain,
			row.NewUserTotal, row.NewUserTotalChain,
			row.NewUserOrganic, row.NewUserOrganicChain,
			row.NewUserChannel, row.NewUserChannelChain,
			row.NewUserInternal, row.NewUserInternalChain,
			row.NewPayingUserCnt, row.OldPayingUserCnt,
			row.NewPayingUserAmt, row.OldPayingUserAmt,
			row.RechargeOrderCnt, row.PaymentSuccessRatio,
			row.RetentionD1, row.RetentionD3, row.RetentionD7,
			row.LandingPageVisit, row.LandingPageClick, row.LandingPageDownload,
			row.ConversionTotal, row.ConversionNewPaying, row.ConversionOldPaying,
			row.OldUserDAU, row.ARPPU,
			row.Source, row.SrcRowNo, row.IngestedAt,
		); err != nil {
			return fmt.Errorf("append row %d: %w", row.SrcRowNo, err)
		}
	}
	return batch.Send()
}
