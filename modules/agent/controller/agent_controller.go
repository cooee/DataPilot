package controller

import (
	"net/http"

	agentdto "github.com/Caknoooo/go-gin-clean-starter/modules/agent/dto"
	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/service"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/utils"
	"github.com/gin-gonic/gin"
)

type AgentController interface {
	AskMetrics(ctx *gin.Context)
	ListAnomalies(ctx *gin.Context)
}

type agentController struct {
	svc service.AgentService
}

func NewAgentController(svc service.AgentService) AgentController {
	return &agentController{svc: svc}
}

func (c *agentController) AskMetrics(ctx *gin.Context) {
	var req agentdto.AskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.BuildResponseFailed(agentdto.MESSAGE_FAILED_BIND, err.Error(), nil))
		return
	}
	if req.Question == "" && req.Preset == "" {
		ctx.JSON(http.StatusBadRequest, utils.BuildResponseFailed(agentdto.MESSAGE_FAILED_BIND, "question 或 preset 至少填一项", nil))
		return
	}

	resp, err := c.svc.AskMetrics(ctx.Request.Context(), &req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.BuildResponseFailed(agentdto.MESSAGE_FAILED_ASK, err.Error(), nil))
		return
	}
	ctx.JSON(http.StatusOK, utils.BuildResponseSuccess(agentdto.MESSAGE_SUCCESS_ASK, resp))
}

func (c *agentController) ListAnomalies(ctx *gin.Context) {
	var req agentdto.AnomalyListRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.BuildResponseFailed(agentdto.MESSAGE_FAILED_BIND, err.Error(), nil))
		return
	}

	resp, err := c.svc.ListAnomalies(ctx.Request.Context(), &req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.BuildResponseFailed(agentdto.MESSAGE_FAILED_ANOMALY, err.Error(), nil))
		return
	}
	ctx.JSON(http.StatusOK, utils.BuildResponseSuccess(agentdto.MESSAGE_SUCCESS_ANOMALY, resp))
}
