package report

import (
	"fmt"
	"sort"
)

// BuildSegments 构建付费/免费分栏分析数据。
func BuildSegments(p *DailyReportPayload) []SegmentReport {
	paid := buildOneSegment(p, "paid", "付费产品", "增长与变现：新增、新增付费、充值、ARPPU 为核心观测与预测对象。")
	free := buildOneSegment(p, "free", "免费产品", "规模与粘性：日活、7日留存、新增规模为观测与预测核心。")
	return []SegmentReport{paid, free}
}

func buildOneSegment(p *DailyReportPayload, key, label, focus string) SegmentReport {
	snap := snapshotFor(p, key)
	recent := recentTrend(p.TrendHistory, key, 7)
	forecasts := BuildMetricForecasts(p.TrendHistory, p.ReportDate, key, forecastDefs(key))
	anomalies := filterAnomalies(p.Anomalies, key)
	top := topProducts(p.ProductDaily, key, sortKey(key), 10)
	notes := buildAnalystNotes(key, label, snap, recent, forecasts, anomalies)
	risk := riskLevel(len(anomalies), snap)

	return SegmentReport{
		Key: key, Label: label, Focus: focus,
		Snapshot: snap, TrendRecent: recent, Forecasts: forecasts,
		Anomalies: anomalies, TopProducts: top,
		AnalystNotes: notes, RiskLevel: risk,
	}
}

func forecastDefs(key string) []forecastMetricDef {
	if key == "paid" {
		return paidForecastMetrics
	}
	return freeForecastMetrics
}

func snapshotFor(p *DailyReportPayload, productType string) SegmentSnapshot {
	var snap SegmentSnapshot
	for _, c := range p.Compare {
		if c.ProductType == productType {
			snap = SegmentSnapshot{
				DAU: c.DAU, Recharge: c.Recharge, RetentionD7: c.RetentionD7,
				ARPPU: c.ARPPU, NewUsers: c.NewUsers, NewPaying: c.NewPaying,
			}
			break
		}
	}
	pts := filterTrend(p.TrendHistory, productType)
	if len(pts) >= 2 {
		sort.Slice(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })
		last := pts[len(pts)-1]
		prev := pts[len(pts)-2]
		snap.DAUWoW = pctChange(last.DAU, prev.DAU)
		snap.RechargeWoW = pctChange(last.Recharge, prev.Recharge)
		snap.RetainWoW = pctChange(last.RetentionD7, prev.RetentionD7)
	}
	return snap
}

func recentTrend(history []TrendPoint, productType string, n int) []TrendPoint {
	pts := filterTrend(history, productType)
	sort.Slice(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })
	if len(pts) > n {
		pts = pts[len(pts)-n:]
	}
	return pts
}

func filterAnomalies(rows []AnomalyRow, productType string) []AnomalyRow {
	var out []AnomalyRow
	for _, r := range rows {
		if r.ProductType == productType {
			out = append(out, r)
		}
	}
	return out
}

func sortKey(productType string) string {
	if productType == "paid" {
		return "recharge"
	}
	return "dau"
}

func topProducts(rows []ProductDayRow, productType, by string, n int) []ProductDayRow {
	var filtered []ProductDayRow
	for _, r := range rows {
		if r.ProductType == productType {
			filtered = append(filtered, r)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if by == "recharge" {
			return filtered[i].Recharge > filtered[j].Recharge
		}
		return filtered[i].DAU > filtered[j].DAU
	})
	if len(filtered) > n {
		filtered = filtered[:n]
	}
	return filtered
}

func riskLevel(anomalyCount int, snap SegmentSnapshot) string {
	if anomalyCount >= 5 {
		return "high"
	}
	if anomalyCount >= 2 {
		return "medium"
	}
	if snap.DAUWoW < -15 || snap.RechargeWoW < -20 {
		return "medium"
	}
	return "low"
}

func buildAnalystNotes(key, label string, snap SegmentSnapshot, recent []TrendPoint, fc []MetricForecast, anomalies []AnomalyRow) []string {
	var notes []string
	if key == "paid" {
		notes = append(notes, formatPaidSummary(snap))
		notes = append(notes, forecastNarrativePaid(fc)...)
	} else {
		notes = append(notes, formatFreeSummary(snap))
		notes = append(notes, forecastNarrativeFree(fc)...)
	}
	if len(recent) >= 3 {
		notes = append(notes, trendMomentum(key, recent))
	}
	if len(anomalies) > 0 {
		notes = append(notes, label+"报告日共 "+itoa(len(anomalies))+" 个产品指标触发异常规则，建议优先排查 Z-Score 最高的 3 项并对照投放/版本变更。")
	} else {
		notes = append(notes, label+"核心指标未触发异常检测，短期以趋势跟踪与预测区间规划为主。")
	}
	notes = append(notes, forecastDiagnosticsNote(fc)...)
	return notes
}

func formatPaidSummary(s SegmentSnapshot) string {
	return "付费盘快照：日活 " + fmtFloat(s.DAU, 0) + "，新增 " + fmtFloat(s.NewUsers, 0) +
		"，新增付费 " + fmtFloat(s.NewPaying, 0) + "，充值 " + fmtFloat(s.Recharge, 2) +
		"，ARPPU " + fmtFloat(s.ARPPU, 2) + "；充值环比 " + fmtPct(s.RechargeWoW) + "。"
}

func formatFreeSummary(s SegmentSnapshot) string {
	return "免费盘快照：日活 " + fmtFloat(s.DAU, 0) + "，新增 " + fmtFloat(s.NewUsers, 0) +
		"，7日留存 " + fmtPctRatio(s.RetentionD7) + "；日活环比 " + fmtPct(s.DAUWoW) +
		"，留存环比 " + fmtPct(s.RetainWoW) + "。"
}

func forecastNarrativePaid(fc []MetricForecast) []string {
	byKey := lastForecastByMetric(fc)
	var notes []string
	if v, ok := byKey["recharge_total_amt"]; ok {
		notes = append(notes, "充值预测（7日后）："+fmtFloat(v, 2)+"，若低于报告日水平需提前检查付费转化漏斗。")
	}
	if v, ok := byKey["new_paying_user_cnt"]; ok {
		notes = append(notes, "新增付费用户预测（7日后）："+fmtFloat(v, 0)+"，与获客投放节奏对齐评估。")
	}
	return notes
}

func forecastNarrativeFree(fc []MetricForecast) []string {
	byKey := lastForecastByMetric(fc)
	var notes []string
	if v, ok := byKey["dau"]; ok {
		notes = append(notes, "日活预测（7日后）："+fmtFloat(v, 0)+"，关注内容/活动排期是否支撑规模目标。")
	}
	if v, ok := byKey["retention_d7_ratio"]; ok {
		notes = append(notes, "7日留存预测（7日后）："+fmtPctRatio(v)+"，留存下滑时需排查新用户质量与首日体验。")
	}
	return notes
}

func lastForecastByMetric(fc []MetricForecast) map[string]float64 {
	m := map[string]float64{}
	for _, row := range fc {
		if row.Date > "" {
			m[row.MetricKey] = row.Value
		}
	}
	// 取每个 metric 最后一天
	latest := map[string]MetricForecast{}
	for _, row := range fc {
		if prev, ok := latest[row.MetricKey]; !ok || row.Date > prev.Date {
			latest[row.MetricKey] = row
		}
	}
	out := map[string]float64{}
	for k, row := range latest {
		out[k] = row.Value
	}
	return out
}

func trendMomentum(key string, recent []TrendPoint) string {
	if len(recent) < 2 {
		return ""
	}
	first, last := recent[0], recent[len(recent)-1]
	if key == "paid" {
		chg := pctChange(last.Recharge, first.Recharge)
		return "近 " + itoa(len(recent)) + " 日充值趋势：" + trendWord(chg) + "（区间变化 " + fmtPct(chg) + "）。"
	}
	chg := pctChange(last.DAU, first.DAU)
	return "近 " + itoa(len(recent)) + " 日日活趋势：" + trendWord(chg) + "（区间变化 " + fmtPct(chg) + "）。"
}

func forecastDiagnosticsNote(forecasts []MetricForecast) []string {
	if len(forecasts) == 0 {
		return []string{"预测：无历史趋势数据，请补录 DWS 多日数据后重跑。"}
	}
	byKey := map[string]MetricForecast{}
	for _, f := range forecasts {
		byKey[f.MetricKey] = f
	}
	var notes []string
	notes = append(notes, "预测机制：优先取近7个自然日样本做一元线性回归；历史2~6日则用全样本回归；仅1日时 Carry Forward（数值不变）。大促/节假日需人工校正。")
	for _, k := range []string{"recharge_total_amt", "dau", "new_paying_user_cnt", "retention_d7_ratio"} {
		if f, ok := byKey[k]; ok {
			notes = append(notes, fmt.Sprintf("- %s：%s（样本%d天）", f.MetricLabel, f.MethodDetail, f.SampleDays))
		}
	}
	return notes
}

func trendWord(pct float64) string {
	switch {
	case pct > 5:
		return "上行"
	case pct < -5:
		return "下行"
	default:
		return "震荡"
	}
}

func fmtFloat(v float64, dec int) string {
	if dec == 0 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.*f", dec, v)
}

func fmtPct(v float64) string {
	return fmt.Sprintf("%+.1f%%", v)
}

func fmtPctRatio(v float64) string {
	return fmt.Sprintf("%.1f%%", v*100)
}
