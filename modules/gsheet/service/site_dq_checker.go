package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// SiteDQChecker 对 ods_site_product_daily 执行 DQ 检查。
type SiteDQChecker struct {
	conn driver.Conn
}

func NewSiteDQChecker(conn driver.Conn) *SiteDQChecker {
	return &SiteDQChecker{conn: conn}
}

// Check 站点产品 ODS 数据质量检查。
func (c *SiteDQChecker) Check(ctx context.Context, dateFrom, dateTo time.Time) (*DQReport, error) {
	report := &DQReport{DateFrom: dateFrom, DateTo: dateTo, OK: true}
	dateFrom = dateFrom.UTC().Truncate(24 * time.Hour)
	dateTo = dateTo.UTC().Truncate(24 * time.Hour)

	checks := []func(context.Context, time.Time, time.Time) (DQResult, error){
		c.checkDuplicates,
		c.checkNegativeDAU,
		c.checkNegativeLeadNew,
		c.checkNegativeLeadRecharge,
		c.checkOutlierDAUChain,
	}

	for _, fn := range checks {
		r, err := fn(ctx, dateFrom, dateTo)
		if err != nil {
			return nil, err
		}
		if !r.Passed {
			report.OK = false
		}
		report.Results = append(report.Results, r)
	}
	return report, nil
}

func (c *SiteDQChecker) checkDuplicates(ctx context.Context, from, to time.Time) (DQResult, error) {
	rule := "duplicate (date, product_code, source)"
	var cnt uint64
	q := fmt.Sprintf(`
		SELECT count()
		FROM (
			SELECT `+"`日期`, `产品编号`, `_source`"+`, count() AS n
			FROM ods_site_product_daily FINAL
			WHERE `+"`日期`"+` BETWEEN '%s' AND '%s'
			GROUP BY `+"`日期`, `产品编号`, `_source`"+`
			HAVING n > 1
		)`, from.Format("2006-01-02"), to.Format("2006-01-02"))
	err := c.conn.QueryRow(ctx, q).Scan(&cnt)
	if err != nil {
		return DQResult{}, fmt.Errorf("site dq duplicate: %w", err)
	}
	detail := "no duplicates"
	if cnt > 0 {
		detail = fmt.Sprintf("%d duplicate groups", cnt)
	}
	return DQResult{Rule: rule, Passed: cnt == 0, Detail: detail, FailCnt: cnt}, nil
}

func (c *SiteDQChecker) checkNegativeDAU(ctx context.Context, from, to time.Time) (DQResult, error) {
	return c.checkNegativeField(ctx, from, to, "`日活跃数`", "negative DAU (日活跃数)")
}

func (c *SiteDQChecker) checkNegativeLeadNew(ctx context.Context, from, to time.Time) (DQResult, error) {
	return c.checkNegativeField(ctx, from, to, "`日导量新增`", "negative lead_new")
}

func (c *SiteDQChecker) checkNegativeLeadRecharge(ctx context.Context, from, to time.Time) (DQResult, error) {
	return c.checkNegativeField(ctx, from, to, "`日导量充值`", "negative lead_recharge")
}

func (c *SiteDQChecker) checkNegativeField(ctx context.Context, from, to time.Time, col, ruleName string) (DQResult, error) {
	var cnt uint64
	q := fmt.Sprintf("SELECT count() FROM ods_site_product_daily FINAL "+
		"WHERE `日期` BETWEEN '%s' AND '%s' AND toFloat64(%s) < 0",
		from.Format("2006-01-02"), to.Format("2006-01-02"), col)
	err := c.conn.QueryRow(ctx, q).Scan(&cnt)
	if err != nil {
		return DQResult{}, fmt.Errorf("site dq %s: %w", ruleName, err)
	}
	detail := "ok"
	if cnt > 0 {
		detail = fmt.Sprintf("%d rows with %s", cnt, ruleName)
	}
	return DQResult{Rule: ruleName, Passed: cnt == 0, Detail: detail, FailCnt: cnt}, nil
}

func (c *SiteDQChecker) checkOutlierDAUChain(ctx context.Context, from, to time.Time) (DQResult, error) {
	rule := "outlier DAU chain-ratio (|val-mean| > 3sigma, warn only)"
	var cnt uint64
	q := fmt.Sprintf(`
		WITH stats AS (
			SELECT avg(`+"`日活环比`"+`) AS mu, stddevPop(`+"`日活环比`"+`) AS sigma
			FROM ods_site_product_daily FINAL
			WHERE `+"`日期`"+` BETWEEN '%s' AND '%s'
		)
		SELECT count()
		FROM ods_site_product_daily FINAL, stats
		WHERE `+"`日期`"+` BETWEEN '%s' AND '%s'
		  AND abs(`+"`日活环比`"+` - mu) > 3 * sigma`,
		from.Format("2006-01-02"), to.Format("2006-01-02"),
		from.Format("2006-01-02"), to.Format("2006-01-02"))
	err := c.conn.QueryRow(ctx, q).Scan(&cnt)
	if err != nil {
		return DQResult{}, fmt.Errorf("site dq outlier: %w", err)
	}
	detail := "no outliers"
	if cnt > 0 {
		detail = fmt.Sprintf("%d rows exceed 3sigma (warning only)", cnt)
	}
	return DQResult{Rule: rule, Passed: true, Detail: detail, FailCnt: cnt}, nil
}
