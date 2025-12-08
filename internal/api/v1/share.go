package v1

import (
	"errors"
	"strconv"

	"short-link/internal/dto"
	bizErrors "short-link/internal/pkg/errors"
	"short-link/internal/pkg/jwt"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/response"
	"short-link/internal/pkg/validator"
	"short-link/internal/service"

	"github.com/gin-gonic/gin"
)

type ShareHandler struct {
	svc service.ShareService
}

func NewShareHandler(svc service.ShareService) *ShareHandler {
	return &ShareHandler{svc: svc}
}

// 统一错误处理
func handleShareServiceError(c *gin.Context, err error) {
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

// Get 获取分享信息
func (h *ShareHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	shortCode := c.Param("short_code")

	result, err := h.svc.Get(ctx, userInfo, shortCode)
	if err != nil {
		handleShortlinkServiceError(c, err) // 复用已有的错误处理器
		return
	}
	response.Ok(c, result, "获取成功")
}

// Upsert 创建或更新分享信息
func (h *ShareHandler) Upsert(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	shortCode := c.Param("short_code")

	var req dto.UpdateShareInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	if err := h.svc.Upsert(ctx, userInfo, shortCode, &req); err != nil {
		handleShortlinkServiceError(c, err)
		return
	}
	response.Ok(c, nil, "更新成功")
}

func (h *ShareHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.CreateShareRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	if err := h.svc.Create(ctx, &req); err != nil {
		handleShareServiceError(c, err)
		return
	}
	response.Ok(c, nil, "创建成功")
}

func (h *ShareHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, response.InvalidParam, "无效的ID")
		return
	}

	var req dto.UpdateShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	if err := h.svc.Update(ctx, uint(id), &req); err != nil {
		handleShareServiceError(c, err)
		return
	}

	response.Ok(c, nil, "更新成功")
}

func (h *ShareHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, response.InvalidParam, "无效的ID")
		return
	}

	if err := h.svc.Delete(ctx, uint(id)); err != nil {
		handleShareServiceError(c, err)
		return
	}
	response.Ok(c, nil, "删除成功")
}

func (h *ShareHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, response.InvalidParam, "无效的ID")
		return
	}

	result, err := h.svc.GetByID(ctx, uint(id))
	if err != nil {
		handleShareServiceError(c, err)
		return
	}

	response.Ok(c, result, "获取成功")
}

func (h *ShareHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	// TODO: 添加更健壮的分页参数校验

	results, err := h.svc.List(ctx, offset, limit)
	if err != nil {
		handleShareServiceError(c, err)
		return
	}

	response.Ok(c, results, "获取成功")
}

// ========== 为每个索引生成对应的 Handler 方法 ==========

// GetUniqueShortCode handles the request to get a Share by its index.
// @Router /api/v1/share/uniqueshortcode/:shortCode [get]
func (h *ShareHandler) GetUniqueShortCode(c *gin.Context) {
	ctx := c.Request.Context()
	// 1. 从 URL 路径中解析参数

	paramShortCode := c.Param("shortCode")

	var shortCode string = paramShortCode

	// 2. 调用 Service
	// 注意：这里的 GetBy... 方法名和参数列表必须与 service 层完全对应
	result, err := h.svc.GetUniqueShortCode(ctx, shortCode)
	if err != nil {
		handleShareServiceError(c, err)
		return
	}
	// 3. 成功响应
	response.Ok(c, result, "success")
}
