package v1

import (
	"errors"
	"short-link/internal/dto"
	"short-link/internal/dto/common"
	bizErrors "short-link/internal/pkg/errors"
	"short-link/internal/pkg/jwt"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/response"
	"short-link/internal/service"

	"github.com/gin-gonic/gin"
)

type StatsHandler struct {
	svc service.StatsService
}

func NewStatsHandler(svc service.StatsService) *StatsHandler {
	return &StatsHandler{svc: svc}
}

// handleStatsServiceError 是 Stats 模块专属的错误处理器
func handleStatsServiceError(c *gin.Context, err error) {
	var bizErr *bizErrors.Error
	if errors.As(err, &bizErr) {
		response.Fail(c, bizErr.Code, bizErr.Message)
		return
	}
	logger.Error("未处理的统计服务错误", "error", err)
	response.Fail(c, response.InternalError, "")
}


// GetOverview 获取概览统计
func (h *StatsHandler) GetOverview(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	shortCode := c.Param("short_code")

	result, err := h.svc.GetOverview(ctx, userInfo, shortCode)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}

	response.Ok(c, result, "获取成功")
}

// GetTrend 获取点击趋势
func (h *StatsHandler) GetTrend(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	shortCode := c.Param("short_code")

	var req dto.GetTrendRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetTrend(ctx, userInfo, shortCode, &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "获取成功")
}

// GetProvinces 获取省级统计
func (h *StatsHandler) GetProvinces(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	shortCode := c.Param("short_code")

	result, err := h.svc.GetProvinces(ctx, userInfo, shortCode)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "获取成功")
}

// GetCities 获取市级统计
func (h *StatsHandler) GetCities(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	shortCode := c.Param("short_code")

	var req dto.GetCitiesStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetCities(ctx, userInfo, shortCode, &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "获取成功")
}

// GetDevices 获取设备统计
func (h *StatsHandler) GetDevices(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	shortCode := c.Param("short_code")
	// 从查询参数获取维度
	dimension := c.DefaultQuery("dimension", "device_type") // 默认为device_type

	result, err := h.svc.GetDevices(ctx, userInfo, shortCode, dimension)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "获取成功")
}

// GetSources 获取来源统计
func (h *StatsHandler) GetSources(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	shortCode := c.Param("short_code")

	result, err := h.svc.GetSources(ctx, userInfo, shortCode)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "获取成功")
}

// GetLogs 获取原始访问日志
func (h *StatsHandler) GetLogs(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	shortCode := c.Param("short_code")

	var req common.PaginationRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}
	
	result, err := h.svc.GetLogs(ctx, userInfo, shortCode, &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "获取成功")
}

// GetGlobalStats 获取平台全局统计
func (h *StatsHandler) GetGlobalStats(c *gin.Context) {
	ctx := c.Request.Context()
	// 注意：此接口已由管理员中间件保护，无需再校验权限
	result, err := h.svc.GetGlobalStats(ctx)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "获取成功")
}