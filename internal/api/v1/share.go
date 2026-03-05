package v1

import (
	"errors"
	"strconv"

	"slink-api/internal/dto"
	bizErrors "slink-api/internal/pkg/errors"
	"slink-api/internal/pkg/jwt"
	"slink-api/internal/pkg/logger"
	"slink-api/internal/pkg/response"
	"slink-api/internal/pkg/validator"
	"slink-api/internal/service"

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
// @Summary      获取短链接分享信息
// @Description  获取指定短链接的社交分享元信息（标题、描述、图片）
// @Tags         分享
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string  true  "短码"
// @Success      200         {object}  response.Response{data=dto.GetShareInfoResponse}
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shares/{short_code} [get]
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
// @Summary      设置短链接分享信息
// @Description  创建或更新指定短链接的社交分享元信息
// @Tags         分享
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        short_code  path      string                      true  "短码"
// @Param        request     body      dto.UpdateShareInfoRequest  true  "分享信息请求体"
// @Success      200         {object}  response.Response
// @Failure      400         {object}  response.Response
// @Failure      401         {object}  response.Response
// @Failure      403         {object}  response.Response
// @Failure      404         {object}  response.Response
// @Failure      500         {object}  response.Response
// @Router       /shares/{short_code} [put]
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

// Create 创建分享信息
// @Summary      创建分享信息
// @Description  创建新的分享信息记录
// @Tags         分享
// @Accept       json
// @Produce      json
// @Param        request  body      dto.CreateShareRequest  true  "创建分享请求体"
// @Success      200      {object}  response.Response
// @Failure      400      {object}  response.Response
// @Failure      500      {object}  response.Response
// @Router       /shares [post]
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

// Update 更新分享信息
// @Summary      更新分享信息
// @Description  根据ID更新分享信息
// @Tags         分享
// @Accept       json
// @Produce      json
// @Param        id       path      int                     true  "分享信息ID"
// @Param        request  body      dto.UpdateShareRequest  true  "更新分享请求体"
// @Success      200      {object}  response.Response
// @Failure      400      {object}  response.Response
// @Failure      404      {object}  response.Response
// @Failure      500      {object}  response.Response
// @Router       /shares/{id} [put]
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

// Delete 删除分享信息
// @Summary      删除分享信息
// @Description  根据ID删除分享信息
// @Tags         分享
// @Produce      json
// @Param        id  path      int  true  "分享信息ID"
// @Success      200 {object}  response.Response
// @Failure      400 {object}  response.Response
// @Failure      404 {object}  response.Response
// @Failure      500 {object}  response.Response
// @Router       /shares/{id} [delete]
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

// GetByID 根据ID获取分享信息
// @Summary      获取分享信息详情
// @Description  根据ID获取分享信息详情
// @Tags         分享
// @Produce      json
// @Param        id  path      int  true  "分享信息ID"
// @Success      200 {object}  response.Response{data=dto.ShareResponse}
// @Failure      400 {object}  response.Response
// @Failure      404 {object}  response.Response
// @Failure      500 {object}  response.Response
// @Router       /shares/{id} [get]
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

// List 获取分享信息列表
// @Summary      获取分享信息列表
// @Description  分页获取分享信息列表
// @Tags         分享
// @Produce      json
// @Param        offset  query     int  false  "偏移量"  default(0)
// @Param        limit   query     int  false  "数量"    default(10)
// @Success      200     {object}  response.Response{data=[]dto.ShareResponse}
// @Failure      500     {object}  response.Response
// @Router       /shares [get]
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

// GetUniqueShortCode 根据短码获取分享信息
// @Summary      根据短码获取分享信息
// @Description  通过唯一短码获取对应的分享信息
// @Tags         分享
// @Produce      json
// @Param        shortCode  path      string  true  "短码"
// @Success      200        {object}  response.Response{data=dto.ShareResponse}
// @Failure      404        {object}  response.Response
// @Failure      500        {object}  response.Response
// @Router       /shares/code/{shortCode} [get]
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
