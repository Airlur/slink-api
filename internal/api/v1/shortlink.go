package v1

import (
	"errors"
	"net/http"

	"short-link/internal/dto"
	bizErrors "short-link/internal/pkg/errors"
	"short-link/internal/pkg/jwt"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/response"
	"short-link/internal/pkg/validator"
	"short-link/internal/service"

	"github.com/gin-gonic/gin"
)

type ShortlinkHandler struct {
	svc service.ShortlinkService
}

func NewShortlinkHandler(svc service.ShortlinkService) *ShortlinkHandler {
	return &ShortlinkHandler{svc: svc}
}

// handleShortlinkServiceError 统一错误处理，通过 Map 驱动
func handleShortlinkServiceError(c *gin.Context, err error) {
	// 尝试从错误链中解析出我们自定义的业务错误
	var bizErr *bizErrors.Error
	if errors.As(err, &bizErr) {
		// 如果Service层返回的错误中包含了具体的消息，我们优先使用它
		// 否则，Fail函数会从errorMap中查找默认消息
		response.Fail(c, bizErr.Code, bizErr.Message)
		return
	}

	// 对于所有其他未知错误，记录日志并返回通用的内部错误
	logger.Error("未处理的服务层错误", "error", err)
	response.Fail(c, response.InternalError, "") // message留空，Fail函数会自动填充
}

// Create 创建短链接
// @Summary      创建短链接
// @Description  创建新的短链接。游客只需提供原始URL；登录用户可自定义短码和过期时间。
// @Tags         短链接
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string                          false  "Bearer Token（可选）"
// @Param        request        body      dto.UserCreateShortlinkRequest  true   "创建请求体"
// @Success      200            {object}  response.Response{data=dto.ShortlinkResponse}
// @Failure      400            {object}  response.Response
// @Failure      409            {object}  response.Response
// @Failure      500            {object}  response.Response
// @Router       /shortlinks [post]
func (h *ShortlinkHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)

	if userInfo != nil {
		// ========== 登录用户逻辑 ==========
		var req dto.UserCreateShortlinkRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			// 统一调用新的绑定错误处理器
			validator.HandleBindingError(c, err)
			return
		}

		// 调用 service 层，并传入用户信息
		result, err := h.svc.CreateForUser(ctx, userInfo, &req)
		if err != nil {
			handleShortlinkServiceError(c, err)
			return
		}
		response.Ok(c, result, "创建成功")

	} else {
		// ========== 游客逻辑 ==========
		var req dto.GuestCreateShortlinkRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			// 统一调用新的绑定错误处理器
			validator.HandleBindingError(c, err)
			return
		}

		// 调用 service 层，传入 nil 或特定的游客标识
		result, err := h.svc.CreateForGuest(ctx, &req)
		if err != nil {
			handleShortlinkServiceError(c, err)
			return
		}
		response.Ok(c, result, "创建成功")
	}
}

// Redirect 短链接重定向
// @Summary      短链接重定向
// @Description  根据短码重定向到原始URL，同时记录访问统计
// @Tags         短链接
// @Produce      html
// @Param        shortCode  path      string  true  "短码"
// @Success      302        {string}  string  "重定向到原始URL"
// @Failure      404        {string}  string  "短链接不存在"
// @Failure      500        {object}  response.Response
// @Router       /{shortCode} [get]
func (h *ShortlinkHandler) Redirect(c *gin.Context) {
	ctx := c.Request.Context()
	shortCode := c.Param("shortCode")
	// 获取IP, UserAgent, Referer 和可能存在的 UserInfo
	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")
	referer := c.Request.Referer()
	userInfo := jwt.GetUserInfo(c)

	if shortCode == "" {
		response.Fail(c, response.InvalidParam, "短码不能为空")
		return
	}

	originalUrl, cacheStatus, err := h.svc.Redirect(ctx, shortCode, ip, ua, referer, userInfo)
	if err != nil {
		// 对于跳转链接，如果找不到，我们通常不返回JSON，而是显示一个404页面。
		// 这里为了简化，我们先返回一个清晰的错误响应。
		// 在实际项目中，你可能会渲染一个HTML模板： c.HTML(http.StatusNotFound, "404.html", nil)
		if e, ok := err.(*bizErrors.Error); ok {
			if e.Code == response.ShortlinkNotFound {
				// 短链接不存在，返回自带的 404 页面
				c.Data(http.StatusNotFound, "text/html; charset=utf-8", []byte(notFoundPageHTML))
				return
			}
			// 其他业务错误，返回500
			response.Fail(c, e.Code, e.Message)
			return
		}

		// 处理其他内部未知错误
		handleShortlinkServiceError(c, err)
		return
	}

	// 注入 X-Cache 响应头
	c.Header("X-Cache", cacheStatus)
	// 执行重定向
	// 使用 302 Found (临时重定向)，这是短链接服务的常见做法，
	// 因为它告诉客户端/浏览器每次都应该检查这个链接，而不是永久缓存结果。
	c.Redirect(http.StatusFound, originalUrl)
}

// Update 更新短链接
// @Summary      更新短链接
// @Description  更新指定短链接的信息
// @Tags         短链接
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string                      true  "短码"
// @Param        request     body      dto.UpdateShortlinkRequest  true  "更新请求体"
// @Success      200         {object}  response.Response
// @Failure      400         {object}  response.Response
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shortlinks/{short_code} [put]
func (h *ShortlinkHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	shortCode := c.Param("short_code")
	userInfo := jwt.GetUserInfo(c)

	var req dto.UpdateShortlinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	if err := h.svc.Update(ctx, userInfo, shortCode, &req); err != nil {
		handleShortlinkServiceError(c, err)
		return
	}

	response.Ok(c, nil, "更新成功")
}

// Delete 删除短链接
// @Summary      删除短链接
// @Description  删除指定的短链接
// @Tags         短链接
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string  true  "短码"
// @Success      200         {object}  response.Response
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shortlinks/{short_code} [delete]
func (h *ShortlinkHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	shortCode := c.Param("short_code")
	userInfo := jwt.GetUserInfo(c)

	if err := h.svc.Delete(ctx, userInfo, shortCode); err != nil {
		handleShortlinkServiceError(c, err)
		return
	}
	response.Ok(c, nil, "删除成功")
}

// ListMy 获取我的短链接列表
// @Summary      获取我的短链接列表
// @Description  分页获取当前用户创建的短链接
// @Tags         短链接
// @Produce      json
// @Security     BearerAuth
// @Param        page     query     int     false  "页码"      default(1)
// @Param        limit    query     int     false  "每页数量"  default(20)
// @Param        tag      query     string  false  "标签筛选"
// @Param        sort_by  query     string  false  "排序字段"
// @Success      200      {object}  response.Response{data=dto.ShortlinkListResponse}
// @Failure      401      {object}  response.Response
// @Failure      500      {object}  response.Response
// @Router       /shortlinks/my [get]
func (h *ShortlinkHandler) ListMy(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)

	var req dto.ListMyShortlinksRequest
	// 使用 ShouldBindQuery 来绑定 URL 查询参数
	if err := c.ShouldBindQuery(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	result, err := h.svc.ListMyShortlinks(ctx, userInfo, &req)
	if err != nil {
		handleShortlinkServiceError(c, err)
		return
	}

	response.Ok(c, result, "获取成功")
}

// GetDetail 获取短链接详情
// @Summary      获取短链接详情
// @Description  获取指定短链接的详细信息
// @Tags         短链接
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string  true  "短码"
// @Success      200         {object}  response.Response{data=dto.ShortlinkDetailResponse}
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shortlinks/{short_code} [get]
func (h *ShortlinkHandler) GetDetail(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	shortCode := c.Param("short_code")

	result, err := h.svc.GetDetailByShortCode(ctx, userInfo, shortCode)
	if err != nil {
		handleShortlinkServiceError(c, err)
		return
	}

	response.Ok(c, result, "获取成功")
}

// UpdateStatus 更新短链接状态
// @Summary      更新短链接状态
// @Description  更新短链接的启用/禁用状态
// @Tags         短链接
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string                            true  "短码"
// @Param        request     body      dto.UpdateShortlinkStatusRequest  true  "状态请求体"
// @Success      200         {object}  response.Response
// @Failure      400         {object}  response.Response
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shortlinks/{short_code}/status [patch]
func (h *ShortlinkHandler) UpdateStatus(c *gin.Context) {
	ctx := c.Request.Context()
	shortCode := c.Param("short_code")
	userInfo := jwt.GetUserInfo(c) // 必须是登录用户

	var req dto.UpdateShortlinkStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	if err := h.svc.UpdateStatus(ctx, userInfo, shortCode, &req); err != nil {
		handleShortlinkServiceError(c, err)
		return
	}

	response.Ok(c, nil, "更新成功")
}

// ExtendExpiration 延长短链接有效期
// @Summary      延长短链接有效期
// @Description  延长指定短链接的过期时间
// @Tags         短链接
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string                                true  "短码"
// @Param        request     body      dto.ExtendShortlinkExpirationRequest  true  "延期请求体"
// @Success      200         {object}  response.Response
// @Failure      400         {object}  response.Response
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shortlinks/{short_code}/expiration [patch]
func (h *ShortlinkHandler) ExtendExpiration(c *gin.Context) {
	ctx := c.Request.Context()
	shortCode := c.Param("short_code")
	userInfo := jwt.GetUserInfo(c) // 必须是登录用户

	var req dto.ExtendShortlinkExpirationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	if err := h.svc.ExtendExpiration(ctx, userInfo, shortCode, &req); err != nil {
		handleShortlinkServiceError(c, err)
		return
	}

	response.Ok(c, nil, "有效期延长成功")
}

const notFoundPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="UTF-8">
	<title>404 - 页面未找到</title>
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		:root {
			font-family: "Inter", "PingFang SC", "Microsoft YaHei", sans-serif;
			color: #0f172a;
			background-color: #fff;
		}
		body {
			margin: 0;
			padding: 0;
		}
		.wrapper {
			min-height: 100vh;
			display: flex;
			align-items: center;
			justify-content: center;
			padding: 2rem;
		}
		.card {
			text-align: center;
			max-width: 480px;
		}
		.icon-wrapper {
			width: 72px;
			height: 72px;
			margin: 0 auto 24px;
			border-radius: 50%;
			background: #f1f5f9;
			display: flex;
			align-items: center;
			justify-content: center;
		}
		.icon {
			width: 36px;
			height: 36px;
			color: #64748b;
		}
		h1 {
			margin: 0 0 12px;
			font-size: 32px;
		}
		p {
			margin: 0 auto;
			color: #475569;
			line-height: 1.6;
		}
	</style>
</head>
<body>
	<div class="wrapper">
		<div class="card">
			<div class="icon-wrapper">
				<svg xmlns="http://www.w3.org/2000/svg" class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
					<path d="M12 8v4m0 4h.01"></path>
					<circle cx="12" cy="12" r="9" />
				</svg>
			</div>
			<h1>404 - 页面未找到</h1>
			<p>抱歉，您访问的页面不存在。它可能已被移动、删除，或者您输入的网址有误。</p>
		</div>
	</div>
</body>
</html>`
