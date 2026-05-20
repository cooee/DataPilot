package service

import (
	"fmt"
	"strings"

	analyticsdto "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
)

func buildAnswer(preset string, result *analyticsdto.QueryResult, reportDate string) string {
	if result == nil || len(result.Rows) == 0 {
		return fmt.Sprintf("%s 未查询到数据。", reportDate)
	}

	switch preset {
	case "product-type-compare":
		return buildProductTypeCompareAnswer(result, reportDate)
	case "anomaly-detection":
		return buildAnomalyAnswer(result, reportDate)
	case "product-health":
		return buildHealthAnswer(result)
	default:
		return fmt.Sprintf("共 %d 条结果（preset=%s）。", len(result.Rows), preset)
	}
}

func buildProductTypeCompareAnswer(result *analyticsdto.QueryResult, date string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("【%s 付费 vs 免费】\n", date))
	for _, row := range result.Rows {
		pt, _ := row["product_type"].(string)
		dau := formatNum(row["dau"])
		recharge := formatNum(row["recharge_total_amt"])
		retention := formatNum(row["retention_d7_ratio"])
		b.WriteString(fmt.Sprintf("- %s：日活 %s，充值 %s，7留 %s\n", pt, dau, recharge, retention))
	}
	return strings.TrimSpace(b.String())
}

func buildAnomalyAnswer(result *analyticsdto.QueryResult, date string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("【%s 异常产品】共 %d 条\n", date, len(result.Rows)))
	for i, row := range result.Rows {
		if i >= 10 {
			b.WriteString(fmt.Sprintf("… 另有 %d 条\n", len(result.Rows)-10))
			break
		}
		code, _ := row["product_code"].(string)
		name, _ := row["product_name"].(string)
		metric, _ := row["metric"].(string)
		dir, _ := row["anomaly_direction"].(string)
		val := formatNum(row["value"])
		z := formatNum(row["z_score"])
		b.WriteString(fmt.Sprintf("- %s %s | %s=%s (%s, z=%s)\n", code, name, metric, val, dir, z))
	}
	return strings.TrimSpace(b.String())
}

func buildHealthAnswer(result *analyticsdto.QueryResult) string {
	var b strings.Builder
	b.WriteString("【产品健康度 Top】\n")
	for i, row := range result.Rows {
		if i >= 5 {
			break
		}
		name, _ := row["product_name"].(string)
		score := formatNum(row["health_score"])
		b.WriteString(fmt.Sprintf("- %s 健康分 %s\n", name, score))
	}
	return strings.TrimSpace(b.String())
}

func formatNum(v any) string {
	if v == nil {
		return "-"
	}
	switch n := v.(type) {
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", n), "0"), ".")
	case int, int64:
		return fmt.Sprintf("%v", n)
	default:
		return fmt.Sprint(v)
	}
}
