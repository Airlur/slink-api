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
// @Summary      获取统计概览
// @Description  获取指定短链接的统计概览数据（总点击、UV、今日点击等）
// @Tags         统计
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string  true  "短码"
// @Success      200         {object}  response.Response{data=dto.OverviewStatsResponse}
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shortlinks/{short_code}/stats/overview [get]
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
// @Summary      获取点击趋势
// @Description  获取指定短链接的点击趋势数据，支持按天/小时粒度
// @Tags         统计
// @Produce      json
// @Security     BearerAuth
// @Param        short_code   path      string  true   "短码"
// @Param        granularity  query     string  false  "粒度：hour/day"  default(day)
// @Param        start_date   query     string  false  "开始日期"
// @Param        end_date     query     string  false  "结束日期"
// @Success      200          {object}  response.Response{data=[]dto.TrendStatsResponse}
// @Failure      400          {object}  response.Response
// @Failure      401          {object}  response.Response
// @Failure      403          {object}  response.Response
// @Failure      404          {object}  response.Response
// @Failure      500          {object}  response.Response
// @Router       /shortlinks/{short_code}/stats/trend [get]
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
// @Summary      获取省份分布统计
// @Description  获取指定短链接的访问省份分布数据
// @Tags         统计
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string  true  "短码"
// @Success      200         {object}  response.Response{data=[]dto.RegionStatsResponse}
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shortlinks/{short_code}/stats/provinces [get]
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
// @Summary      获取城市分布统计
// @Description  获取指定短链接在某省份下的城市分布数据
// @Tags         统计
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string  true   "短码"
// @Param        province    query     string  false  "省份名称"
// @Success      200         {object}  response.Response{data=[]dto.RegionStatsResponse}
// @Failure      400         {object}  response.Response
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shortlinks/{short_code}/stats/cities [get]
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
// @Summary      获取设备分布统计
// @Description  获取指定短链接的设备分布数据，支持按设备类型/操作系统/浏览器维度
// @Tags         统计
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string  true   "短码"
// @Param        dimension   query     string  false  "维度：device_type/os/browser"  default(device_type)
// @Success      200         {object}  response.Response{data=[]dto.DeviceStatsResponse}
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shortlinks/{short_code}/stats/devices [get]
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
// @Summary      获取来源分布统计
// @Description  获取指定短链接的访问来源（Referer）分布数据
// @Tags         统计
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string  true  "短码"
// @Success      200         {object}  response.Response{data=[]dto.SourceStatsResponse}
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shortlinks/{short_code}/stats/sources [get]
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

// GetLogs 获取访问日志
// @Summary      获取访问日志
// @Description  分页获取指定短链接的原始访问日志记录
// @Tags         统计
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string  true   "短码"
// @Param        page        query     int     false  "页码"      default(1)
// @Param        page_size   query     int     false  "每页数量"  default(20)
// @Success      200         {object}  response.Response{data=dto.AccessLogListResponse}
// @Failure      400         {object}  response.Response
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shortlinks/{short_code}/stats/logs [get]
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

// GetUserOverview ��ȡ�û��ۺ�ͳ��
func (h *StatsHandler) GetUserOverview(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)

	result, err := h.svc.GetUserOverview(ctx, userInfo)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "��ȡ�ɹ�")
}

// GetUserTrend ��ȡ�û��ۺ�����
func (h *StatsHandler) GetUserTrend(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)

	var req dto.UserTrendRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.InvalidParam, err.Error())
		return
	}

	result, err := h.svc.GetUserTrend(ctx, userInfo, &req)
	if err != nil {
		handleStatsServiceError(c, err)
		return
	}
	response.Ok(c, result, "success")
}

// GetGlobalStats 获取全局统计
// @Summary      获取平台全局统计
// @Description  获取平台级别的全局统计数据（仅管理员可访问）
// @Tags         统计
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response{data=dto.GlobalStatsResponse}
// @Failure      401  {object}  response.Response
// @Failure      403  {object}  response.Response  "无管理员权限"
// @Failure      500  {object}  response.Response
// @Router       /admin/stats/global [get]
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
