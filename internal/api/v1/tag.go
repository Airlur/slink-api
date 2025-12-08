package v1

import (
	"errors"

	"short-link/internal/dto"
	bizErrors "short-link/internal/pkg/errors"
	"short-link/internal/pkg/jwt"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/response"
	"short-link/internal/pkg/validator"
	"short-link/internal/service"

	"github.com/gin-gonic/gin"
)

type TagHandler struct {
	svc service.TagService
}

func NewTagHandler(svc service.TagService) *TagHandler {
	return &TagHandler{svc: svc}
}

// 统一错误处理
func handleTagServiceError(c *gin.Context, err error) {
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

// Create 为短链接添加标签
func (h *TagHandler) Add(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	shortCode := c.Param("short_code")

	var req dto.AddTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	if err := h.svc.Add(ctx, userInfo, shortCode, &req); err != nil {
		handleTagServiceError(c, err)
		return
	}
	response.Ok(c, nil, "添加标签成功")
}

// Delete 从短链接移除标签
func (h *TagHandler) Remove(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	shortCode := c.Param("short_code")

	// 移除标签时，tagName通常放在请求体中，以支持未来可能更复杂的删除逻辑
	var req dto.RemoveTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	if err := h.svc.Remove(ctx, userInfo, shortCode, &req); err != nil {
		handleTagServiceError(c, err)
		return
	}
	response.Ok(c, nil, "移除标签成功")
}

// List 获取用户的所有标签
func (h *TagHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)

	result, err := h.svc.List(ctx, userInfo)
	if err != nil {
		handleShortlinkServiceError(c, err)
		return
	}
	response.Ok(c, result, "获取成功")
}
