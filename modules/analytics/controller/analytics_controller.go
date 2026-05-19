package controller

import (
	"net/http"

	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/service"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/utils"
	"github.com/gin-gonic/gin"
)

type AnalyticsController interface {
	Query(ctx *gin.Context)
	Schema(ctx *gin.Context)
}

type analyticsController struct {
	svc service.AnalyticsService
}

func NewAnalyticsController(svc service.AnalyticsService) AnalyticsController {
	return &analyticsController{svc: svc}
}

func (c *analyticsController) Query(ctx *gin.Context) {
	var req dto.QueryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.BuildResponseFailed(dto.MESSAGE_FAILED_BIND, err.Error(), nil))
		return
	}

	result, meta, err := c.svc.Query(ctx.Request.Context(), &req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.BuildResponseFailed(dto.MESSAGE_FAILED_QUERY, err.Error(), nil))
		return
	}

	res := utils.BuildResponseSuccess(dto.MESSAGE_SUCCESS_QUERY, result)
	res.Meta = meta
	ctx.JSON(http.StatusOK, res)
}

func (c *analyticsController) Schema(ctx *gin.Context) {
	schema, err := c.svc.GetSchema(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.BuildResponseFailed(dto.MESSAGE_FAILED_SCHEMA, err.Error(), nil))
		return
	}
	ctx.JSON(http.StatusOK, utils.BuildResponseSuccess(dto.MESSAGE_SUCCESS_SCHEMA, schema))
}
