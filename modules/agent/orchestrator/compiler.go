package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	analyticsCLI "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/cli"
	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/catalog"
	analyticsdto "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
	"github.com/Caknoooo/go-gin-clean-starter/database/entities"
	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/provider"
	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/skill"
)

// Compiler NL→QueryPlan 编排入口（LLM 优先，失败回退规则）。
type Compiler struct {
	rule    *RulePlanner
	orch    *provider.Orchestrator
	metrics []entities.MetaMetricDict
	skill   *skill.Document
}

func NewCompiler(metrics []entities.MetaMetricDict, llm provider.LLMProvider) *Compiler {
	c := &Compiler{
		rule:    NewRulePlanner(metrics),
		metrics: metrics,
	}
	if doc, err := skill.LoadFromEnv(); err != nil && llmDebug() {
		log.Printf("[agent] skill load warning: %v", err)
	} else {
		c.skill = doc
		if doc != nil && llmDebug() {
			log.Printf("[agent] skill injected id=%s path=%s sha=%s", doc.ID, doc.Path, doc.SHA256)
		}
	}
	if llm != nil {
		if _, ok := llm.(provider.StubLLMProvider); !ok {
			c.orch = provider.NewOrchestrator(llm)
		}
	}
	return c
}

func (c *Compiler) Compile(ctx context.Context, question, intentHint, dateOverride string) (*QueryPlan, error) {
	useLLM := c.orch != nil && isLLMEnabled()
	if useLLM {
		plan, err := c.compileViaLLM(ctx, question, intentHint, dateOverride)
		if err == nil {
			return plan, nil
		}
		if llmDebug() {
			log.Printf("[agent] LLM 编排失败，回退规则引擎: %v", err)
		}
	}
	plan, err := c.rule.Plan(question, intentHint, dateOverride)
	if err != nil {
		return nil, err
	}
	c.attachSkill(plan)
	return plan, nil
}

func (c *Compiler) attachSkill(plan *QueryPlan) {
	if c.skill != nil {
		plan.Skill = c.skill.Meta
	}
}

func llmDebug() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_LLM_DEBUG")))
	return v == "1" || v == "true" || v == "yes"
}

func isLLMEnabled() bool {
	p := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_LLM_PROVIDER")))
	return p != "" && p != "stub" && p != "rule"
}

func (c *Compiler) compileViaLLM(ctx context.Context, question, intentHint, dateOverride string) (*QueryPlan, error) {
	req := provider.PlanRequest{
		Question: question,
		Date:     dateOverride,
		Intent:   intentHint,
		Presets:  presetNames(),
		Glossary: provider.BuildGlossary(c.metrics, 35),
	}
	if c.skill != nil {
		req.SkillID = c.skill.ID
		req.SkillText = c.skill.Body
	}
	result, err := c.orch.Plan(ctx, req)
	if err != nil {
		return nil, err
	}

	query, err := normalizeLLMPlan(result, dateOverride)
	if err != nil {
		return nil, err
	}

	plan := &QueryPlan{
		Intent:         result.Intent,
		Preset:         result.Preset,
		Query:          query,
		Interpretation: result.Interpretation,
		Confidence:     0.9,
		Source:         "llm",
	}
	if c.skill != nil {
		plan.Skill = c.skill.Meta
	}
	return plan, nil
}

// normalizeLLMPlan 校验 LLM 输出：合法 preset 优先用内置构建器，避免中文 dataset 名。
func normalizeLLMPlan(result *provider.PlanResult, dateOverride string) (*analyticsdto.QueryRequest, error) {
	preset := strings.TrimSpace(result.Preset)
	if preset != "" {
		if fn, ok := analyticsCLI.Presets[preset]; ok {
			from, to := resolvePlanDates(result.Query, dateOverride)
			return fn(from, to)
		}
	}

	if result.Query != nil && result.Query.Dataset != "" {
		if _, ok := catalog.GetDataset(result.Query.Dataset); ok {
			return result.Query, nil
		}
	}

	ds := ""
	if result.Query != nil {
		ds = result.Query.Dataset
	}
	return nil, fmt.Errorf("LLM 返回无效计划: preset=%q dataset=%q", preset, ds)
}

func resolvePlanDates(q *analyticsdto.QueryRequest, dateOverride string) (string, string) {
	from, to := dateOverride, dateOverride
	if q != nil && q.DateRange != nil {
		if q.DateRange.From != "" {
			from = q.DateRange.From
		}
		if q.DateRange.To != "" {
			to = q.DateRange.To
		}
	}
	if from == "" {
		from = to
	}
	if to == "" {
		to = from
	}
	return from, to
}

func presetNames() []string {
	names := make([]string, 0, len(analyticsCLI.Presets))
	for k := range analyticsCLI.Presets {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
