package report

import (
	"context"
	"fmt"
	"time"

	analyticsCLI "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/cli"
	analyticsdto "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
	analyticsSvc "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/service"
	gsheetSvc "github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/service"
)

type CollectInput struct {
	ReportDate string
	SyncFrom   time.Time
	SyncTo     time.Time
	DQOK       bool
	DQSummary  string
}

// Collect 拉取日报所需的全部 Analytics 数据集。
func Collect(ctx context.Context, analytics analyticsSvc.AnalyticsService, in CollectInput) (*DailyReportPayload, error) {
	reportDate := in.ReportDate
	rd, err := time.Parse("2006-01-02", reportDate)
	if err != nil {
		return nil, fmt.Errorf("invalid report date: %w", err)
	}
	// 预测默认回溯 30 日（含报告日），不足 7 日时降级为全样本/单日外推
	trendFrom := rd.AddDate(0, 0, -(forecastHistoryDays - 1)).Format("2006-01-02")
	trendTo := reportDate
	teamFrom := rd.AddDate(0, 0, -6).Format("2006-01-02")

	payload := &DailyReportPayload{
		ReportDate:  reportDate,
		SyncFrom:    in.SyncFrom.Format("2006-01-02"),
		SyncTo:      in.SyncTo.Format("2006-01-02"),
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		DQOK:        in.DQOK,
		DQSummary:   in.DQSummary,
	}

	queries := []struct {
		preset string
		from   string
		to     string
		apply  func(*DailyReportPayload, *analyticsdto.QueryResult)
	}{
		{"product-type-compare", reportDate, reportDate, applyCompare},
		{"team-performance", teamFrom, trendTo, applyTeam},
		{"product-health", "", "", applyHealth},
		{"anomaly-detection", reportDate, reportDate, applyAnomaly},
	}

	for _, q := range queries {
		fn, ok := analyticsCLI.Presets[q.preset]
		if !ok {
			return nil, fmt.Errorf("preset %s not found", q.preset)
		}
		req, err := fn(q.from, q.to)
		if err != nil {
			return nil, err
		}
		res, _, err := analytics.Query(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", q.preset, err)
		}
		q.apply(payload, res)
	}

	// 扩展趋势（含新增/ARPPU，供分类型预测）
	trendReq := &analyticsdto.QueryRequest{
		Dataset: "dws_product_daily",
		Metrics: []string{
			"dau", "recharge_total_amt", "retention_d7_ratio", "arppu",
			"new_user_total_cnt", "new_paying_user_cnt",
		},
		Dimensions: []string{"date", "product_type"},
		DateRange:  &analyticsdto.DateRange{From: trendFrom, To: trendTo},
		OrderBy:    []analyticsdto.OrderBy{{Field: "date", Desc: false}},
		Limit:      500,
	}
	if res, _, err := analytics.Query(ctx, trendReq); err != nil {
		return nil, fmt.Errorf("query product-trend-extended: %w", err)
	} else {
		applyTrend(payload, res)
	}

	detailReq := &analyticsdto.QueryRequest{
		Dataset: "dws_product_daily",
		Metrics: []string{
			"dau", "recharge_total_amt", "retention_d7_ratio", "arppu",
			"new_user_total_cnt", "new_paying_user_cnt",
		},
		Dimensions: []string{"date", "product_code", "product_name", "team", "product_type"},
		DateRange:  &analyticsdto.DateRange{From: reportDate, To: reportDate},
		OrderBy:    []analyticsdto.OrderBy{{Field: "recharge_total_amt", Desc: true}},
		Limit:      300,
	}
	if res, _, err := analytics.Query(ctx, detailReq); err != nil {
		return nil, fmt.Errorf("query product-detail-extended: %w", err)
	} else {
		applyProductDaily(payload, res)
	}

	payload.Forecast7d = BuildForecast7d(payload.TrendHistory, reportDate)
	payload.Segments = BuildSegments(payload)
	payload.Insights = BuildInsights(*payload)
	return payload, nil
}

func applyCompare(p *DailyReportPayload, res *analyticsdto.QueryResult) {
	for _, row := range res.Rows {
		p.Compare = append(p.Compare, TypeCompareRow{
			ProductType: str(row["product_type"]),
			DAU:         num(row["dau"]),
			Recharge:    num(row["recharge_total_amt"]),
			RetentionD7: num(row["retention_d7_ratio"]),
			ARPPU:       num(row["arppu"]),
			NewUsers:    num(row["new_user_total_cnt"]),
			NewPaying:   num(row["new_paying_user_cnt"]),
		})
	}
}

func applyTeam(p *DailyReportPayload, res *analyticsdto.QueryResult) {
	for _, row := range res.Rows {
		p.TeamDaily = append(p.TeamDaily, TeamRow{
			Date:        str(row["date"]),
			Team:        str(row["team"]),
			DAU:         num(row["dau_sum"]),
			Recharge:    num(row["recharge_total_amt_sum"]),
			RetentionD7: num(row["retention_d7_ratio_wavg"]),
		})
	}
}

func applyHealth(p *DailyReportPayload, res *analyticsdto.QueryResult) {
	for _, row := range res.Rows {
		p.HealthTop = append(p.HealthTop, HealthRow{
			ProductCode: str(row["product_code"]),
			ProductName: str(row["product_name"]),
			Team:        str(row["team"]),
			HealthScore: num(row["health_score"]),
			AvgDAU:      num(row["avg_dau_30d"]),
			Recharge30d: num(row["recharge_total_30d"]),
		})
	}
}

func applyProductDaily(p *DailyReportPayload, res *analyticsdto.QueryResult) {
	for _, row := range res.Rows {
		p.ProductDaily = append(p.ProductDaily, ProductDayRow{
			ProductCode: str(row["product_code"]),
			ProductName: str(row["product_name"]),
			Team:        str(row["team"]),
			ProductType: str(row["product_type"]),
			DAU:         num(row["dau"]),
			Recharge:    num(row["recharge_total_amt"]),
			RetentionD7: num(row["retention_d7_ratio"]),
			ARPPU:       num(row["arppu"]),
			NewUsers:    num(row["new_user_total_cnt"]),
			NewPaying:   num(row["new_paying_user_cnt"]),
		})
	}
}

func applyAnomaly(p *DailyReportPayload, res *analyticsdto.QueryResult) {
	for _, row := range res.Rows {
		p.Anomalies = append(p.Anomalies, AnomalyRow{
			ProductCode: str(row["product_code"]),
			ProductName: str(row["product_name"]),
			Team:        str(row["team"]),
			ProductType: str(row["product_type"]),
			Metric:      str(row["metric"]),
			Value:       num(row["value"]),
			ZScore:      num(row["z_score"]),
			Direction:   str(row["anomaly_direction"]),
		})
	}
}

func applyTrend(p *DailyReportPayload, res *analyticsdto.QueryResult) {
	p.TrendHistory = nil
	for _, row := range res.Rows {
		p.TrendHistory = append(p.TrendHistory, TrendPoint{
			Date:        str(row["date"]),
			ProductType: str(row["product_type"]),
			DAU:         num(row["dau"]),
			Recharge:    num(row["recharge_total_amt"]),
			RetentionD7: num(row["retention_d7_ratio"]),
			ARPPU:       num(row["arppu"]),
			NewUsers:    num(row["new_user_total_cnt"]),
			NewPaying:   num(row["new_paying_user_cnt"]),
		})
	}
}

func DQSummaryFromReport(r *gsheetSvc.DQReport) (bool, string) {
	if r == nil {
		return true, "DQ OK"
	}
	return r.OK, gsheetSvc.SummaryLine(r)
}

func str(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	case uint32:
		return float64(n)
	default:
		return 0
	}
}
