package cli

import (
	"fmt"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
)

// Preset 内置常用查询场景（对应 docs/cli.md 验证命令）。
var Presets = map[string]func(dateFrom, dateTo string) (*dto.QueryRequest, error){
	"product-type-compare": presetProductTypeCompare,
	"product-detail":       presetProductDetail,
	"product-trend":        presetProductTrend,
	"team-performance":     presetTeamPerformance,
	"product-health":       presetProductHealth,
	"anomaly-detection":    presetAnomalyDetection,
	"anomaly-watch":        presetAnomalyWatch,
	"anomaly-baseline":     presetAnomalyBaseline,
	// 站点产品（dws_site_product_daily，与 paid/free 独立表）
	"site-product-summary": presetSiteProductSummary,
	"site-product-detail":  presetSiteProductDetail,
	"site-product-trend":   presetSiteProductTrend,
	"site-team-summary":    presetSiteTeamSummary,
}

func presetProductTypeCompare(from, to string) (*dto.QueryRequest, error) {
	from, to, err := resolveDateRange(from, to, true)
	if err != nil {
		return nil, err
	}
	return &dto.QueryRequest{
		Dataset:    "dws_product_daily",
		Metrics:    []string{"dau", "recharge_total_amt", "retention_d7_ratio", "arppu", "new_user_total_cnt", "new_paying_user_cnt"},
		Dimensions: []string{"product_type"},
		DateRange:  &dto.DateRange{From: from, To: to},
		Limit:      100,
	}, nil
}

func presetProductDetail(from, to string) (*dto.QueryRequest, error) {
	from, to, err := resolveDateRange(from, to, true)
	if err != nil {
		return nil, err
	}
	return &dto.QueryRequest{
		Dataset: "dws_product_daily",
		Metrics: []string{"dau", "recharge_total_amt", "retention_d7_ratio", "arppu"},
		Dimensions: []string{
			"date", "product_type", "product_code", "product_name", "team",
		},
		DateRange: &dto.DateRange{From: from, To: to},
		OrderBy:   []dto.OrderBy{{Field: "recharge_total_amt", Desc: true}},
		Limit:     200,
	}, nil
}

func presetProductTrend(from, to string) (*dto.QueryRequest, error) {
	from, to, err := resolveDateRange(from, to, false)
	if err != nil {
		return nil, err
	}
	return &dto.QueryRequest{
		Dataset:    "dws_product_daily",
		Metrics:    []string{"dau", "recharge_total_amt", "retention_d7_ratio"},
		Dimensions: []string{"date", "product_type"},
		DateRange:  &dto.DateRange{From: from, To: to},
		OrderBy:    []dto.OrderBy{{Field: "date", Desc: false}},
		Limit:      500,
	}, nil
}

func presetTeamPerformance(from, to string) (*dto.QueryRequest, error) {
	from, to, err := resolveDateRange(from, to, false)
	if err != nil {
		return nil, err
	}
	return &dto.QueryRequest{
		Dataset: "ads_team_performance_daily",
		Metrics: []string{"dau_sum", "recharge_total_amt_sum", "retention_d7_ratio_wavg"},
		Dimensions: []string{"date", "team"},
		DateRange:  &dto.DateRange{From: from, To: to},
		OrderBy:    []dto.OrderBy{{Field: "date", Desc: true}},
		Limit:      100,
	}, nil
}

func presetProductHealth(_from, _to string) (*dto.QueryRequest, error) {
	return &dto.QueryRequest{
		Dataset: "ads_product_health_overview",
		Metrics: []string{"avg_dau_30d", "recharge_total_30d", "health_score"},
		Dimensions: []string{
			"product_code", "product_name", "team",
		},
		OrderBy: []dto.OrderBy{{Field: "health_score", Desc: true}},
		Limit:   20,
	}, nil
}

// presetAnomalyDetection 报告日超出 ±3σ 的产品指标异常（需至少 2 天历史，std>0）。
func presetAnomalyDetection(from, to string) (*dto.QueryRequest, error) {
	from, to, err := resolveDateRange(from, to, true)
	if err != nil {
		return nil, err
	}
	return &dto.QueryRequest{
		Dataset: "ads_product_anomaly_daily",
		Metrics: []string{"value", "baseline_mean", "baseline_std", "baseline_cnt", "upper_3sigma", "lower_3sigma", "z_score"},
		Dimensions: []string{
			"date", "product_code", "product_name", "team", "metric", "anomaly_direction",
		},
		DateRange: &dto.DateRange{From: from, To: to},
		Filters: []dto.Filter{
			{Field: "is_anomaly", Operator: "eq", Value: 1},
		},
		OrderBy: []dto.OrderBy{{Field: "z_score", Desc: true}},
		Limit:   100,
	}, nil
}

// presetAnomalyWatch 报告日偏离度 Top N（含 insufficient_baseline，便于历史不足时排查）。
func presetAnomalyWatch(from, to string) (*dto.QueryRequest, error) {
	from, to, err := resolveDateRange(from, to, true)
	if err != nil {
		return nil, err
	}
	return &dto.QueryRequest{
		Dataset: "ads_product_anomaly_daily",
		Metrics: []string{"value", "baseline_mean", "baseline_std", "z_score", "is_anomaly"},
		Dimensions: []string{
			"date", "product_code", "product_name", "team", "metric", "anomaly_direction",
		},
		DateRange: &dto.DateRange{From: from, To: to},
		OrderBy:   []dto.OrderBy{{Field: "z_score", Desc: true}},
		Limit:     30,
	}, nil
}

// presetAnomalyBaseline 查看产品 30 日基线区间（不含当日值）。
func presetAnomalyBaseline(_from, _to string) (*dto.QueryRequest, error) {
	return &dto.QueryRequest{
		Dataset:    "ads_anomaly_baseline",
		Metrics:    []string{"mean", "std", "upper_3sigma", "lower_3sigma"},
		Dimensions: []string{"product_code", "metric"},
		OrderBy:    []dto.OrderBy{{Field: "product_code", Desc: false}},
		Limit:      200,
	}, nil
}

func presetSiteProductSummary(from, to string) (*dto.QueryRequest, error) {
	from, to, err := resolveDateRange(from, to, true)
	if err != nil {
		return nil, err
	}
	return &dto.QueryRequest{
		Dataset:    "dws_site_product_daily",
		Metrics:    []string{"dau", "lead_new_cnt", "lead_recharge_amt"},
		Dimensions: []string{"date"},
		DateRange:  &dto.DateRange{From: from, To: to},
		Limit:      10,
	}, nil
}

func presetSiteProductDetail(from, to string) (*dto.QueryRequest, error) {
	from, to, err := resolveDateRange(from, to, true)
	if err != nil {
		return nil, err
	}
	return &dto.QueryRequest{
		Dataset: "dws_site_product_daily",
		Metrics: []string{"dau", "dau_chain_ratio", "lead_new_cnt", "lead_recharge_amt"},
		Dimensions: []string{
			"date", "product_code", "product_name", "team", "business_unit",
		},
		DateRange: &dto.DateRange{From: from, To: to},
		OrderBy:   []dto.OrderBy{{Field: "lead_recharge_amt", Desc: true}},
		Limit:     200,
	}, nil
}

func presetSiteProductTrend(from, to string) (*dto.QueryRequest, error) {
	from, to, err := resolveDateRange(from, to, false)
	if err != nil {
		return nil, err
	}
	return &dto.QueryRequest{
		Dataset:    "dws_site_product_daily",
		Metrics:    []string{"dau", "lead_new_cnt", "lead_recharge_amt"},
		Dimensions: []string{"date"},
		DateRange:  &dto.DateRange{From: from, To: to},
		OrderBy:    []dto.OrderBy{{Field: "date", Desc: false}},
		Limit:      500,
	}, nil
}

func presetSiteTeamSummary(from, to string) (*dto.QueryRequest, error) {
	from, to, err := resolveDateRange(from, to, true)
	if err != nil {
		return nil, err
	}
	return &dto.QueryRequest{
		Dataset:    "dws_site_product_daily",
		Metrics:    []string{"dau", "lead_new_cnt", "lead_recharge_amt"},
		Dimensions: []string{"date", "team"},
		DateRange:  &dto.DateRange{From: from, To: to},
		OrderBy:    []dto.OrderBy{{Field: "lead_recharge_amt", Desc: true}},
		Limit:      50,
	}, nil
}

// resolveDateRange singleDay=true 时 from/to 相同则只用一天。
func resolveDateRange(from, to string, singleDay bool) (string, string, error) {
	now := time.Now().Format("2006-01-02")
	if from == "" && to == "" {
		if singleDay {
			yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			return yesterday, yesterday, nil
		}
		// 默认近 7 天
		return time.Now().AddDate(0, 0, -7).Format("2006-01-02"), now, nil
	}
	if from == "" {
		from = to
	}
	if to == "" {
		to = from
	}
	if _, err := time.Parse("2006-01-02", from); err != nil {
		return "", "", fmt.Errorf("无效 -from: %w", err)
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		return "", "", fmt.Errorf("无效 -to: %w", err)
	}
	return from, to, nil
}
