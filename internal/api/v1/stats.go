package v1

import (
	"errors"

	"slink-api/internal/dto"
	bizErrors "slink-api/internal/pkg/errors"
	"slink-api/internal/pkg/jwt"
	"slink-api/internal/pkg/logger"
	"slink-api/internal/pkg/response"
	"slink-api/internal/service"

	"github.com/gin-gonic/gin"
)

type StatsHandler struct {
	svc service.StatsService
}

func NewStatsHandler(svc service.StatsService) *StatsHandler {
	return &StatsHandler{svc: svc}
}

func handleStatsServiceError(c *gin.Context, err error) {
	var bizErr *bizErrors.Error
	if errors.As(err, &bizErr) {
		response.Fail(c, bizErr.Code, bizErr.Message)
		return
	}
	logger.Error("unhandled stats service error", "error", err)
	response.Fail(c, response.InternalError, "")
}

func (h *StatsHandler) GetOverview(c *gin.Context) {
	result, err := h.svc.GetOverview(c.Request.Context(), jwt.GetUserInfo(c), c.Param("short_code"))
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetTrend(c *gin.Context) {
	var req dto.GetTrendRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetTrend(c.Request.Context(), jwt.GetUserInfo(c), c.Param("short_code"), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetProvinces(c *gin.Context) {
	var req dto.GetProvincesStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetProvinces(c.Request.Context(), jwt.GetUserInfo(c), c.Param("short_code"), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetCities(c *gin.Context) {
	var req dto.GetCitiesStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetCities(c.Request.Context(), jwt.GetUserInfo(c), c.Param("short_code"), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetDevices(c *gin.Context) {
	var req dto.GetDevicesStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetDevices(c.Request.Context(), jwt.GetUserInfo(c), c.Param("short_code"), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetSources(c *gin.Context) {
	var req dto.GetSourcesStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetSources(c.Request.Context(), jwt.GetUserInfo(c), c.Param("short_code"), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetLogs(c *gin.Context) {
	var req dto.GetLogsStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetLogs(c.Request.Context(), jwt.GetUserInfo(c), c.Param("short_code"), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetUserOverview(c *gin.Context) {
	result, err := h.svc.GetUserOverview(c.Request.Context(), jwt.GetUserInfo(c))
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetUserTrend(c *gin.Context) {
	var req dto.UserTrendRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetUserTrend(c.Request.Context(), jwt.GetUserInfo(c), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetUserRegions(c *gin.Context) {
	var req dto.GetProvincesStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetUserRegions(c.Request.Context(), jwt.GetUserInfo(c), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetUserCities(c *gin.Context) {
	var req dto.GetCitiesStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetUserCities(c.Request.Context(), jwt.GetUserInfo(c), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetUserDevices(c *gin.Context) {
	var req dto.GetDevicesStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetUserDevices(c.Request.Context(), jwt.GetUserInfo(c), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetUserSources(c *gin.Context) {
	var req dto.GetSourcesStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetUserSources(c.Request.Context(), jwt.GetUserInfo(c), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetUserTopLinks(c *gin.Context) {
	var req dto.UserTopLinksRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetUserTopLinks(c.Request.Context(), jwt.GetUserInfo(c), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetUserDashboardActions(c *gin.Context) {
	var req dto.DashboardActionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetUserDashboardActions(c.Request.Context(), jwt.GetUserInfo(c), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetUserMap(c *gin.Context) {
	var req dto.MapStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetUserMap(c.Request.Context(), jwt.GetUserInfo(c), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetUserSourceTrend(c *gin.Context) {
	var req dto.SourceTrendRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetUserSourceTrend(c.Request.Context(), jwt.GetUserInfo(c), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetUserTagPerformance(c *gin.Context) {
	var req dto.TagPerformanceRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetUserTagPerformance(c.Request.Context(), jwt.GetUserInfo(c), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetMap(c *gin.Context) {
	var req dto.MapStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetMap(c.Request.Context(), jwt.GetUserInfo(c), c.Param("short_code"), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetCompare(c *gin.Context) {
	var req dto.GetTrendRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetCompare(c.Request.Context(), jwt.GetUserInfo(c), c.Param("short_code"), &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

func (h *StatsHandler) GetGlobalStats(c *gin.Context) {
	result, err := h.svc.GetGlobalStats(c.Request.Context())
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}
