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
	case "site-product-summary", "site-product-trend":
		return buildSiteSummaryAnswer(result, reportDate, preset)
	case "site-product-detail":
		return buildSiteDetailAnswer(result, reportDate)
	case "site-team-summary":
		return buildSiteTeamAnswer(result, reportDate)
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

func buildSiteSummaryAnswer(result *analyticsdto.QueryResult, date, preset string) string {
	title := fmt.Sprintf("【%s 站点产品汇总】", date)
	if preset == "site-product-trend" {
		title = fmt.Sprintf("【%s~%s 站点产品走势】", firstDate(result), lastDate(result))
	}
	var b strings.Builder
	b.WriteString(title + "\n")
	for _, row := range result.Rows {
		d := formatDate(row["date"])
		dau := formatNum(row["dau"])
		leadNew := formatNum(row["lead_new_cnt"])
		leadRecharge := formatNum(row["lead_recharge_amt"])
		if d != "" && d != "-" {
			b.WriteString(fmt.Sprintf("- %s：日活跃 %s，导量新增 %s，导量充值 %s\n", d, dau, leadNew, leadRecharge))
		} else {
			b.WriteString(fmt.Sprintf("- 日活跃 %s，导量新增 %s，导量充值 %s\n", dau, leadNew, leadRecharge))
		}
	}
	return strings.TrimSpace(b.String())
}

func buildSiteDetailAnswer(result *analyticsdto.QueryResult, date string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("【%s 站点产品明细】共 %d 个产品\n", date, len(result.Rows)))
	for i, row := range result.Rows {
		if i >= 10 {
			b.WriteString(fmt.Sprintf("… 另有 %d 个产品\n", len(result.Rows)-10))
			break
		}
		name, _ := row["product_name"].(string)
		code, _ := row["product_code"].(string)
		team, _ := row["team"].(string)
		dau := formatNum(row["dau"])
		leadNew := formatNum(row["lead_new_cnt"])
		leadRecharge := formatNum(row["lead_recharge_amt"])
		b.WriteString(fmt.Sprintf("- %s %s（%s）日活跃 %s，导量新增 %s，导量充值 %s\n",
			code, name, team, dau, leadNew, leadRecharge))
	}
	return strings.TrimSpace(b.String())
}

func buildSiteTeamAnswer(result *analyticsdto.QueryResult, date string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("【%s 站点产品·小组汇总】\n", date))
	for _, row := range result.Rows {
		team, _ := row["team"].(string)
		if team == "" {
			team = "(未分组)"
		}
		dau := formatNum(row["dau"])
		leadNew := formatNum(row["lead_new_cnt"])
		leadRecharge := formatNum(row["lead_recharge_amt"])
		b.WriteString(fmt.Sprintf("- %s：日活跃 %s，导量新增 %s，导量充值 %s\n", team, dau, leadNew, leadRecharge))
	}
	return strings.TrimSpace(b.String())
}

func firstDate(result *analyticsdto.QueryResult) string {
	if len(result.Rows) == 0 {
		return "-"
	}
	return formatDate(result.Rows[0]["date"])
}

func lastDate(result *analyticsdto.QueryResult) string {
	if len(result.Rows) == 0 {
		return "-"
	}
	return formatDate(result.Rows[len(result.Rows)-1]["date"])
}

func formatDate(v any) string {
	if v == nil {
		return "-"
	}
	switch d := v.(type) {
	case string:
		return d
	default:
		return fmt.Sprint(v)
	}
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
