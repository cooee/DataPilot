package orchestrator

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	analyticsCLI "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/cli"
	analyticsdto "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
	"github.com/Caknoooo/go-gin-clean-starter/database/entities"
)

var dateRE = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// RulePlanner 基于关键词 + 指标词典的规则型 NL→语义层编排（v1.1 默认）。
type RulePlanner struct {
	metrics []entities.MetaMetricDict
}

func NewRulePlanner(metrics []entities.MetaMetricDict) *RulePlanner {
	return &RulePlanner{metrics: metrics}
}

// Plan 将自然语言问题编译为 QueryPlan。
func (p *RulePlanner) Plan(question string, intentHint string, dateOverride string) (*QueryPlan, error) {
	qOrig := strings.TrimSpace(question)
	qLower := strings.ToLower(qOrig)
	if qOrig == "" && intentHint == "" {
		return nil, fmt.Errorf("question 不能为空")
	}

	reportDate := p.resolveDate(qOrig, dateOverride)
	from, to := reportDate, reportDate

	intent := intentHint
	if intent == "" {
		intent = p.detectIntent(qLower)
	}

	switch intent {
	case IntentAnomalyList:
		return p.planAnomaly(from, to)
	case IntentMetricQA:
		return p.planMetricQA(qLower, qOrig, from, to)
	default:
		return nil, fmt.Errorf("无法理解问题意图，请尝试：「某日付费免费对比」「某日异常产品」")
	}
}

func (p *RulePlanner) detectIntent(q string) string {
	if containsAny(q, "异常", "离群", "告警", "anomaly", "outlier", "3σ", "3sigma", "sigma") {
		return IntentAnomalyList
	}
	return IntentMetricQA
}

func (p *RulePlanner) planAnomaly(from, to string) (*QueryPlan, error) {
	fn, ok := analyticsCLI.Presets["anomaly-detection"]
	if !ok {
		return nil, fmt.Errorf("preset anomaly-detection 未注册")
	}
	req, err := fn(from, to)
	if err != nil {
		return nil, err
	}
	return &QueryPlan{
		Intent:         IntentAnomalyList,
		Preset:         "anomaly-detection",
		Query:          req,
		Interpretation: fmt.Sprintf("列举 %s 指标超出基线（±3σ 或环比≥30%%）的产品", from),
		Confidence:     1,
		Source:         "rule",
	}, nil
}

func (p *RulePlanner) planMetricQA(qLower, qOrig, from, to string) (*QueryPlan, error) {
	preset := "product-type-compare"

	switch {
	case containsAny(qLower, "健康", "health", "评分"):
		preset = "product-health"
	case containsAny(qLower, "小组", "team", "团队"):
		preset = "team-performance"
		// team-performance 默认近 7 日
		t, _ := time.Parse("2006-01-02", to)
		from = t.AddDate(0, 0, -6).Format("2006-01-02")
	case containsAny(qLower, "趋势", "走势", "trend", "多天", "近7", "近七"):
		preset = "product-trend"
		t, _ := time.Parse("2006-01-02", to)
		from = t.AddDate(0, 0, -6).Format("2006-01-02")
	case containsAny(qLower, "明细", "排行", "top", "充值排序", "产品列表"):
		preset = "product-detail"
	case containsAny(qLower, "对比", "compare", "vs", "付费", "免费", "paid", "free"):
		preset = "product-type-compare"
	default:
		if p.mentionsMetric(qLower, qOrig) {
			preset = "product-type-compare"
		}
	}

	fn, ok := analyticsCLI.Presets[preset]
	if !ok {
		return nil, fmt.Errorf("preset %s 未注册", preset)
	}
	req, err := fn(from, to)
	if err != nil {
		return nil, err
	}

	// 按问题中出现的指标名收窄 metrics（可选）
	if narrowed := p.narrowMetrics(qOrig, req); len(narrowed) > 0 {
		req.Metrics = narrowed
	}

	return &QueryPlan{
		Intent:         IntentMetricQA,
		Preset:         preset,
		Query:          req,
		Interpretation: fmt.Sprintf("查询 %s~%s，场景=%s", from, to, preset),
		Confidence:     1,
		Source:         "rule",
	}, nil
}

func (p *RulePlanner) mentionsMetric(qLower, qOrig string) bool {
	for _, m := range p.metrics {
		if m.MetricNameZH != "" && strings.Contains(qOrig, m.MetricNameZH) {
			return true
		}
		if m.MetricKey != "" && strings.Contains(qLower, m.MetricKey) {
			return true
		}
	}
	return containsAny(qOrig, "日活", "充值", "留存", "新增") || containsAny(qLower, "dau", "arppu")
}

func (p *RulePlanner) narrowMetrics(qOrig string, req *analyticsdto.QueryRequest) []string {
	var keys []string
	for _, m := range p.metrics {
		zh := m.MetricNameZH
		if zh != "" && strings.Contains(qOrig, zh) {
			keys = append(keys, m.MetricKey)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(req.Metrics))
	for _, k := range req.Metrics {
		allowed[k] = struct{}{}
	}
	var out []string
	for _, k := range keys {
		if _, ok := allowed[k]; ok {
			out = append(out, k)
		}
	}
	return out
}

func (p *RulePlanner) resolveDate(q, override string) string {
	if override != "" {
		return override
	}
	if m := dateRE.FindString(q); m != "" {
		return m
	}
	now := time.Now()
	switch {
	case strings.Contains(q, "昨天"), strings.Contains(q, "昨日"):
		return now.AddDate(0, 0, -1).Format("2006-01-02")
	case strings.Contains(q, "今天"), strings.Contains(q, "今日"):
		return now.Format("2006-01-02")
	default:
		return now.AddDate(0, 0, -1).Format("2006-01-02")
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
