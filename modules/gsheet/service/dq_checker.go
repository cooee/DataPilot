package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// DQResult 单项数据质量检查结果。
type DQResult struct {
	Rule    string
	Passed  bool
	Detail  string
	FailCnt uint64
}

// DQReport 一次完整 DQ 检查的汇总报告。
type DQReport struct {
	DateFrom time.Time
	DateTo   time.Time
	Results  []DQResult
	OK       bool
}

// DQChecker 在 ODS 层执行数据质量规则检查。
type DQChecker struct {
	conn driver.Conn
}

func NewDQChecker(conn driver.Conn) *DQChecker {
	return &DQChecker{conn: conn}
}

// Check 对指定日期范围的 ODS 数据做全量 DQ 检查。
func (c *DQChecker) Check(ctx context.Context, dateFrom, dateTo time.Time) (*DQReport, error) {
	report := &DQReport{DateFrom: dateFrom, DateTo: dateTo, OK: true}

	checks := []func(context.Context, time.Time, time.Time) (DQResult, error){
		c.checkDuplicates,
		c.checkNegativeDAU,
		c.checkNegativeRecharge,
		c.checkOutlierDAUChain,
	}

	for _, fn := range checks {
		r, err := fn(ctx, dateFrom, dateTo)
		if err != nil {
			return nil, err
		}
		if !r.Passed {
			report.OK = false
			log.Printf("[dq] FAIL  %s: %s (cnt=%d)", r.Rule, r.Detail, r.FailCnt)
		} else {
			log.Printf("[dq] PASS  %s", r.Rule)
		}
		report.Results = append(report.Results, r)
	}
	return report, nil
}

// PrintReport 把报告打印到标准日志。
func PrintReport(r *DQReport) {
	log.Printf("[dq] ===== DQ Report [%s ~ %s] =====",
		r.DateFrom.Format("2006-01-02"), r.DateTo.Format("2006-01-02"))
	for _, res := range r.Results {
		status := "PASS"
		if !res.Passed {
			status = "FAIL"
		}
		log.Printf("[dq] [%s] %s  %s", status, res.Rule, res.Detail)
	}
	if r.OK {
		log.Printf("[dq] Overall: ALL PASS")
	} else {
		log.Printf("[dq] Overall: HAS FAILURES")
	}
}

// SummaryLine 返回一行可嵌入日志的 DQ 摘要。
func SummaryLine(r *DQReport) string {
	var fails []string
	for _, res := range r.Results {
		if !res.Passed {
			fails = append(fails, res.Rule)
		}
	}
	if len(fails) == 0 {
		return "DQ OK"
	}
	return "DQ FAIL: " + strings.Join(fails, ", ")
}

// --- 具体规则 ---

func (c *DQChecker) checkDuplicates(ctx context.Context, from, to time.Time) (DQResult, error) {
	rule := "duplicate (date, product_code, source)"
	var cnt uint64
	err := c.conn.QueryRow(ctx, `
		SELECT count()
		FROM (
			SELECT `+"`日期`, `产品编号`, `_source`"+`, count() AS n
			FROM ods_product_daily_report FINAL
			WHERE `+"`日期`"+` BETWEEN {from:Date} AND {to:Date}
			GROUP BY `+"`日期`, `产品编号`, `_source`"+`
			HAVING n > 1
		)`,
		driver.NamedValue{Name: "from", Value: from},
		driver.NamedValue{Name: "to", Value: to},
	).Scan(&cnt)
	if err != nil {
		return DQResult{}, fmt.Errorf("dq duplicate: %w", err)
	}
	passed := cnt == 0
	detail := "no duplicates"
	if !passed {
		detail = fmt.Sprintf("%d duplicate groups", cnt)
	}
	return DQResult{Rule: rule, Passed: passed, Detail: detail, FailCnt: cnt}, nil
}

func (c *DQChecker) checkNegativeDAU(ctx context.Context, from, to time.Time) (DQResult, error) {
	return c.checkNegativeField(ctx, from, to, "`日活`", "negative DAU")
}

func (c *DQChecker) checkNegativeRecharge(ctx context.Context, from, to time.Time) (DQResult, error) {
	return c.checkNegativeField(ctx, from, to, "`总充值`", "negative recharge_total")
}

func (c *DQChecker) checkNegativeField(ctx context.Context, from, to time.Time, col, ruleName string) (DQResult, error) {
	var cnt uint64
	q := "SELECT count() FROM ods_product_daily_report FINAL " +
		"WHERE `日期` BETWEEN {from:Date} AND {to:Date} AND toFloat64(" + col + ") < 0"
	err := c.conn.QueryRow(ctx, q,
		driver.NamedValue{Name: "from", Value: from},
		driver.NamedValue{Name: "to", Value: to},
	).Scan(&cnt)
	if err != nil {
		return DQResult{}, fmt.Errorf("dq %s: %w", ruleName, err)
	}
	passed := cnt == 0
	detail := "ok"
	if !passed {
		detail = fmt.Sprintf("%d rows with %s", cnt, ruleName)
	}
	return DQResult{Rule: ruleName, Passed: passed, Detail: detail, FailCnt: cnt}, nil
}

// checkOutlierDAUChain 检测日活环比偏离均值超过 3σ 的行（仅警告，不中断流程）。
func (c *DQChecker) checkOutlierDAUChain(ctx context.Context, from, to time.Time) (DQResult, error) {
	rule := "outlier DAU chain-ratio (|val-mean| > 3sigma, warn only)"
	var cnt uint64
	err := c.conn.QueryRow(ctx, `
		WITH stats AS (
			SELECT avg(`+"`日活环比`"+`) AS mu, stddevPop(`+"`日活环比`"+`) AS sigma
			FROM ods_product_daily_report FINAL
			WHERE `+"`日期`"+` BETWEEN {from:Date} AND {to:Date}
		)
		SELECT count()
		FROM ods_product_daily_report FINAL, stats
		WHERE `+"`日期`"+` BETWEEN {from:Date} AND {to:Date}
		  AND abs(`+"`日活环比`"+` - mu) > 3 * sigma`,
		driver.NamedValue{Name: "from", Value: from},
		driver.NamedValue{Name: "to", Value: to},
	).Scan(&cnt)
	if err != nil {
		return DQResult{}, fmt.Errorf("dq outlier: %w", err)
	}
	detail := "no outliers"
	if cnt > 0 {
		detail = fmt.Sprintf("%d rows exceed 3sigma (warning only)", cnt)
	}
	return DQResult{Rule: rule, Passed: true, Detail: detail, FailCnt: cnt}, nil
}
