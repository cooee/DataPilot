package service

import (
	"context"
	"fmt"

	analyticsCLI "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/cli"
	analyticsSvc "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/service"
	agentdto "github.com/Caknoooo/go-gin-clean-starter/modules/agent/dto"
	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/orchestrator"
	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/provider"
	analyticsRepo "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/repository"
)

type AgentService interface {
	AskMetrics(ctx context.Context, req *agentdto.AskRequest) (*agentdto.AgentResponse, error)
	ListAnomalies(ctx context.Context, req *agentdto.AnomalyListRequest) (*agentdto.AgentResponse, error)
}

type agentService struct {
	analytics analyticsSvc.AnalyticsService
	compiler  *orchestrator.Compiler
}

func NewAgentService(
	analytics analyticsSvc.AnalyticsService,
	metricRepo *analyticsRepo.MetricDictRepo,
	llm provider.LLMProvider,
) AgentService {
	metrics, _ := metricRepo.FindAll()
	return &agentService{
		analytics: analytics,
		compiler:  orchestrator.NewCompiler(metrics, llm),
	}
}

func (s *agentService) AskMetrics(ctx context.Context, req *agentdto.AskRequest) (*agentdto.AgentResponse, error) {
	plan, err := s.resolvePlan(ctx, req.Question, orchestrator.IntentMetricQA, req.Date, req.Preset)
	if err != nil {
		return nil, err
	}
	return s.executePlan(ctx, plan)
}

func (s *agentService) ListAnomalies(ctx context.Context, req *agentdto.AnomalyListRequest) (*agentdto.AgentResponse, error) {
	plan, err := s.compiler.Compile(ctx, req.Question, orchestrator.IntentAnomalyList, req.Date)
	if err != nil {
		return nil, err
	}
	return s.executePlan(ctx, plan)
}

func (s *agentService) resolvePlan(ctx context.Context, question, intent, date, preset string) (*orchestrator.QueryPlan, error) {
	if preset != "" {
		fn, ok := analyticsCLI.Presets[preset]
		if !ok {
			return nil, fmt.Errorf("未知 preset: %s", preset)
		}
		req, err := fn(date, date)
		if err != nil {
			return nil, err
		}
		return &orchestrator.QueryPlan{
			Intent:         intent,
			Preset:         preset,
			Query:          req,
			Interpretation: "直接指定 preset=" + preset,
			Confidence:     1,
			Source:         "rule",
		}, nil
	}
	return s.compiler.Compile(ctx, question, intent, date)
}

func (s *agentService) executePlan(ctx context.Context, plan *orchestrator.QueryPlan) (*agentdto.AgentResponse, error) {
	result, meta, err := s.analytics.Query(ctx, plan.Query)
	if err != nil {
		return nil, err
	}

	reportDate := ""
	if plan.Query.DateRange != nil {
		reportDate = plan.Query.DateRange.To
	}

	return &agentdto.AgentResponse{
		Intent:         plan.Intent,
		Preset:         plan.Preset,
		Source:         plan.Source,
		SkillLoaded:    plan.Skill.Loaded,
		SkillID:        plan.Skill.ID,
		SkillPath:      plan.Skill.Path,
		SkillSHA256:    plan.Skill.SHA256,
		Interpretation: plan.Interpretation,
		Answer:         buildAnswer(plan.Preset, result, reportDate),
		Query:          plan.Query,
		Data:           result,
		Meta:           meta,
	}, nil
}
