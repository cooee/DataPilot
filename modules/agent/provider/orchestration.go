package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	analyticsdto "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/catalog"
	"github.com/Caknoooo/go-gin-clean-starter/database/entities"
)

// PlanRequest Agent NL 编排输入。
type PlanRequest struct {
	Question string
	Date     string
	Intent   string // metric_qa | anomaly_list | 空=自动
	Presets  []string
	Glossary string // meta_metric_dict 摘要
	SkillID  string // 注入的 skill name（验收用）
	SkillText string // SKILL.md 正文（不含 frontmatter）
}

// PlanResult LLM 输出的查询计划（JSON 反序列化）。
type PlanResult struct {
	Intent         string                     `json:"intent"`
	Preset         string                     `json:"preset"`
	Interpretation string                     `json:"interpretation"`
	Query          *analyticsdto.QueryRequest `json:"query"`
}

// Orchestrator 使用 LLMProvider.Chat 完成 NL→语义层 Query 编排。
type Orchestrator struct {
	llm LLMProvider
}

func NewOrchestrator(llm LLMProvider) *Orchestrator {
	return &Orchestrator{llm: llm}
}

func (o *Orchestrator) Plan(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	if o.llm == nil {
		return nil, ErrLLMNotConfigured
	}
	messages := []ChatMessage{
		{Role: "system", Content: buildOrchestrationSystemPrompt(req.Presets, req.Glossary, req.SkillID, req.SkillText)},
		{Role: "user", Content: buildOrchestrationUserPrompt(req)},
	}
	resp, err := o.llm.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}
	return parsePlanJSON(resp.Content)
}

func buildOrchestrationSystemPrompt(presets []string, glossary, skillID, skillText string) string {
	var b strings.Builder
	b.WriteString(`你是 DataPilot 数据分析 Agent，负责将用户自然语言转为语义层 JSON 查询计划。
只输出一个 JSON 对象，不要 markdown 代码块，不要额外说明。

JSON 结构：
{
  "intent": "metric_qa 或 anomaly_list",
  "preset": "预设名（优先使用）",
  "interpretation": "一句话说明",
  "query": {
    "dataset": "数据集名",
    "metrics": ["指标列"],
    "dimensions": ["维度列"],
    "filters": [{"field":"x","operator":"eq","value":"y"}],
    "date_range": {"from":"YYYY-MM-DD","to":"YYYY-MM-DD"},
    "order_by": [{"field":"x","desc":true}],
    "limit": 100
  }
}

规则：
- 必须填写 preset（从下列列表原样复制，不要用中文别名）
- query.dataset 必须使用下列英文表名之一，且与 preset 一致；若已填 preset，可省略 query 各字段
- 可用 preset：` + strings.Join(presets, ", ") + `
- 可用 dataset：` + strings.Join(datasetNames(), ", ") + `
- 异常/离群/告警 → intent=anomaly_list, preset=anomaly-detection
- 付费免费对比 → preset=product-type-compare（dataset=dws_product_daily）
- 日期格式 YYYY-MM-DD
`)
	if glossary != "" {
		b.WriteString("\n指标词典：\n")
		b.WriteString(glossary)
	}
	if strings.TrimSpace(skillText) != "" {
		b.WriteString("\n\n## 运营 Skill（id=")
		b.WriteString(skillID)
		b.WriteString("）\n")
		b.WriteString("下列为项目 Skill 工作流与 CLI 约定；选择 preset 与 interpretation 时须与此一致。\n\n")
		b.WriteString(skillText)
	}
	return b.String()
}

func buildOrchestrationUserPrompt(req PlanRequest) string {
	var b strings.Builder
	b.WriteString("用户问题：")
	b.WriteString(req.Question)
	if req.Date != "" {
		b.WriteString("\n报告日：")
		b.WriteString(req.Date)
	}
	if req.Intent != "" {
		b.WriteString("\n意图提示：")
		b.WriteString(req.Intent)
	}
	if req.Date == "" {
		b.WriteString("\n若未指定日期，默认昨天：")
		b.WriteString(time.Now().AddDate(0, 0, -1).Format("2006-01-02"))
	}
	return b.String()
}

func parsePlanJSON(raw string) (*PlanResult, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var plan PlanResult
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return nil, fmt.Errorf("解析 LLM JSON: %w; raw=%s", err, truncate(raw, 200))
	}
	if plan.Preset == "" && plan.Query == nil {
		return nil, fmt.Errorf("LLM 返回无效计划：缺少 preset 与 query")
	}
	return &plan, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// BuildGlossary 从 meta_metric_dict 生成简短词典文本。
func BuildGlossary(metrics []entities.MetaMetricDict, maxItems int) string {
	if maxItems <= 0 {
		maxItems = 40
	}
	var lines []string
	for i, m := range metrics {
		if i >= maxItems {
			lines = append(lines, "...")
			break
		}
		lines = append(lines, fmt.Sprintf("- %s (%s): %s", m.MetricKey, m.MetricNameZH, m.Description))
	}
	return strings.Join(lines, "\n")
}

func datasetNames() []string {
	names := make([]string, 0, len(catalog.Datasets))
	for k := range catalog.Datasets {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
