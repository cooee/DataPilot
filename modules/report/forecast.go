package report

import (
	"fmt"
	"sort"
	"time"
)

const forecastHistoryDays = 30 // 预测回溯窗口（含报告日）

var paidForecastMetrics = []forecastMetricDef{
	{Key: "new_user_total_cnt", Label: "新增用户", Pick: func(p TrendPoint) float64 { return p.NewUsers }},
	{Key: "new_paying_user_cnt", Label: "新增付费用户", Pick: func(p TrendPoint) float64 { return p.NewPaying }},
	{Key: "recharge_total_amt", Label: "充值金额", Pick: func(p TrendPoint) float64 { return p.Recharge }},
	{Key: "arppu", Label: "ARPPU", Pick: func(p TrendPoint) float64 { return p.ARPPU }},
}

var freeForecastMetrics = []forecastMetricDef{
	{Key: "dau", Label: "日活 DAU", Pick: func(p TrendPoint) float64 { return p.DAU }},
	{Key: "retention_d7_ratio", Label: "7日留存率", Pick: func(p TrendPoint) float64 { return p.RetentionD7 }},
	{Key: "new_user_total_cnt", Label: "新增用户", Pick: func(p TrendPoint) float64 { return p.NewUsers }},
}

// siteForecastMetrics 站点产品仅预测日导量新增。
var siteForecastMetrics = []forecastMetricDef{
	{Key: "lead_new_cnt", Label: "日导量新增", Pick: func(p TrendPoint) float64 { return p.LeadNewCnt }},
}

type forecastMetricDef struct {
	Key   string
	Label string
	Pick  func(TrendPoint) float64
}

type datedValue struct {
	date string
	v    float64
}

// BuildForecast7d 兼容旧逻辑。
func BuildForecast7d(history []TrendPoint, reportDate string) []ForecastPoint {
	byType := map[string][]TrendPoint{}
	for _, h := range history {
		byType[h.ProductType] = append(byType[h.ProductType], h)
	}
	base, _ := time.Parse("2006-01-02", reportDate)
	var out []ForecastPoint
	for pt, pts := range byType {
		pts = prepTrendPoints(pts)
		dauS, dauM, _ := buildForecastSeries(pts, func(p TrendPoint) float64 { return p.DAU })
		recS, recM, _ := buildForecastSeries(pts, func(p TrendPoint) float64 { return p.Recharge })
		method := dauM
		if recM != "" {
			method = recM
		}
		for i := 1; i <= 7; i++ {
			d := base.AddDate(0, 0, i).Format("2006-01-02")
			out = append(out, ForecastPoint{
				Date: d, ProductType: pt,
				DAU: extrapolate(dauS, i), Recharge: extrapolate(recS, i), Method: method,
			})
		}
	}
	sortForecastPoints(out)
	return out
}

// BuildMetricForecasts 按产品线、按指标独立选择回归样本并预测 7 日。
func BuildMetricForecasts(history []TrendPoint, reportDate, productType string, defs []forecastMetricDef) []MetricForecast {
	pts := prepTrendPoints(filterTrend(history, productType))
	base, _ := time.Parse("2006-01-02", reportDate)

	var out []MetricForecast
	for _, def := range defs {
		series, method, sampleDays := buildForecastSeries(pts, def.Pick)
		detail := methodDetailCN(method, sampleDays)
		for i := 1; i <= 7; i++ {
			val := extrapolate(series, i)
			if method == "carry_forward" && len(series) > 0 {
				val = series[len(series)-1]
			}
			out = append(out, MetricForecast{
				Date:         base.AddDate(0, 0, i).Format("2006-01-02"),
				MetricKey:    def.Key,
				MetricLabel:  def.Label,
				Value:        val,
				Method:       method,
				SampleDays:   sampleDays,
				MethodDetail: detail,
			})
		}
	}
	return out
}

// prepTrendPoints 按日期去重、排序，保留每日最后一条。
func prepTrendPoints(pts []TrendPoint) []TrendPoint {
	byDate := map[string]TrendPoint{}
	for _, p := range pts {
		if p.Date == "" {
			continue
		}
		byDate[p.Date] = p
	}
	out := make([]TrendPoint, 0, len(byDate))
	for _, p := range byDate {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	if len(out) > forecastHistoryDays {
		out = out[len(out)-forecastHistoryDays:]
	}
	return out
}

// buildForecastSeries 为单指标构造回归样本：优先近 7 日，其次全样本，不足 2 日则 carry_forward。
func buildForecastSeries(pts []TrendPoint, pick func(TrendPoint) float64) (series []float64, method string, sampleDays int) {
	var dv []datedValue
	for _, p := range pts {
		dv = append(dv, datedValue{date: p.Date, v: pick(p)})
	}
	sort.Slice(dv, func(i, j int) bool { return dv[i].date < dv[j].date })

	if len(dv) == 0 {
		return nil, "no_data", 0
	}

	// 优先使用最近 7 个自然日样本做线性回归
	if len(dv) >= 7 {
		tail := dv[len(dv)-7:]
		return valuesOnly(tail), "linear_regression_7d", 7
	}
	if len(dv) >= 2 {
		return valuesOnly(dv), "linear_regression", len(dv)
	}
	return []float64{dv[0].v}, "carry_forward", 1
}

func valuesOnly(dv []datedValue) []float64 {
	out := make([]float64, len(dv))
	for i, d := range dv {
		out[i] = d.v
	}
	return out
}

func methodDetailCN(method string, sampleDays int) string {
	switch method {
	case "linear_regression_7d":
		return fmt.Sprintf("近%d日线性回归（一元最小二乘外推）", sampleDays)
	case "linear_regression":
		return fmt.Sprintf("全样本%d日线性回归（历史不足7日）", sampleDays)
	case "carry_forward":
		return fmt.Sprintf("单日外推（仅%d日历史，无法回归；沿用最近观测值）", sampleDays)
	default:
		return "无历史样本"
	}
}

func filterTrend(history []TrendPoint, productType string) []TrendPoint {
	var pts []TrendPoint
	for _, h := range history {
		if h.ProductType == productType {
			pts = append(pts, h)
		}
	}
	return pts
}

func extrapolate(series []float64, daysAhead int) float64 {
	if len(series) == 0 {
		return 0
	}
	if len(series) == 1 {
		return series[0]
	}
	n := float64(len(series))
	var sumX, sumY, sumXY, sumXX float64
	for i, y := range series {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	denom := n*sumXX - sumX*sumX
	if denom == 0 {
		return series[len(series)-1]
	}
	b := (n*sumXY - sumX*sumY) / denom
	a := (sumY - b*sumX) / n
	predicted := a + b*(n-1+float64(daysAhead))
	// 比率类指标（留存）限制在 [0,1]
	if predicted < 0 && series[len(series)-1] <= 1 {
		return 0
	}
	if predicted > 1 && series[len(series)-1] <= 1 {
		return 1
	}
	return predicted
}

func sortForecastPoints(out []ForecastPoint) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date == out[j].Date {
			return out[i].ProductType < out[j].ProductType
		}
		return out[i].Date < out[j].Date
	})
}

func pctChange(current, previous float64) float64 {
	if previous == 0 {
		return 0
	}
	return (current - previous) / previous * 100
}

func BuildInsights(p DailyReportPayload) []string {
	var ins []string
	if !p.DQOK {
		ins = append(ins, "数据质量检查未全部通过，请优先处理 DQ 失败项后再做经营决策。")
	}
	for _, seg := range p.Segments {
		if len(seg.Anomalies) > 0 {
			ins = append(ins, seg.Label+"：报告日存在 "+itoa(len(seg.Anomalies))+" 项指标异常，详见对应页签。")
		}
	}
	if len(ins) == 0 {
		ins = append(ins, "报告日整体波动在可控范围，建议结合各产品线页签中的预测与 Top 产品进一步复盘。")
	}
	return ins
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
