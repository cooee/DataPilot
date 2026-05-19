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
