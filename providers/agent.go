package providers

import (
	"context"

	agentCtrl "github.com/Caknoooo/go-gin-clean-starter/modules/agent/controller"
	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/provider"
	agentSvc "github.com/Caknoooo/go-gin-clean-starter/modules/agent/service"
	analyticsRepo "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/repository"
	analyticsSvc "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/service"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/constants"
	"github.com/samber/do"
	"gorm.io/gorm"
)

func InitAgentServices(injector *do.Injector) {
	do.Provide(injector, func(i *do.Injector) (provider.LLMProvider, error) {
		return provider.NewLLMProviderFromEnv(context.Background())
	})

	do.Provide(injector, func(i *do.Injector) (agentSvc.AgentService, error) {
		analytics := do.MustInvoke[analyticsSvc.AnalyticsService](i)
		db := do.MustInvokeNamed[*gorm.DB](i, constants.DB)
		metricRepo := analyticsRepo.NewMetricDictRepo(db)
		llm := do.MustInvoke[provider.LLMProvider](i)
		return agentSvc.NewAgentService(analytics, metricRepo, llm), nil
	})

	do.Provide(injector, func(i *do.Injector) (agentCtrl.AgentController, error) {
		svc := do.MustInvoke[agentSvc.AgentService](i)
		return agentCtrl.NewAgentController(svc), nil
	})
}
