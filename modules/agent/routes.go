package agent

import (
	"github.com/Caknoooo/go-gin-clean-starter/middlewares"
	authService "github.com/Caknoooo/go-gin-clean-starter/modules/auth/service"
	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/controller"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/constants"
	"github.com/gin-gonic/gin"
	"github.com/samber/do"
)

func RegisterRoutes(server *gin.Engine, injector *do.Injector) {
	ctrl := do.MustInvoke[controller.AgentController](injector)
	jwtService := do.MustInvokeNamed[authService.JWTService](injector, constants.JWTService)

	routes := server.Group("/api/agent")
	routes.Use(middlewares.Authenticate(jwtService))
	{
		routes.POST("/metrics/ask", ctrl.AskMetrics)
		routes.POST("/anomalies", ctrl.ListAnomalies)
	}
}
