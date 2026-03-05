package v1

import (
	"errors"

	"slink-api/internal/dto"
	bizErrors "slink-api/internal/pkg/errors"
	"slink-api/internal/pkg/jwt"
	"slink-api/internal/pkg/logger"
	"slink-api/internal/pkg/validator"
	"slink-api/internal/pkg/response"
	"slink-api/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}



// handleUserServiceError 统一错误处理
func handleUserServiceError(c *gin.Context, err error) {
	// 尝试从错误链中解析出我们自定义的业务错误
	var bizErr *bizErrors.Error
	if errors.As(err, &bizErr) {
		// 如果Service层返回的错误中包含了具体的消息，我们优先使用它
		// 否则，Fail函数会从errorMap中查找默认消息
		response.Fail(c, bizErr.Code, bizErr.Message)
		return
	}

	// 对于所有其他未知错误，记录日志并返回通用的内部错误
	logger.Error("未处理的用户服务层错误", "error", err)
	response.Fail(c, response.InternalError, "") // message留空，Fail函数会自动填充
}

// CheckExistence godoc
// @Summary      检查用户名或邮箱是否存在
// @Description  检查用户名或邮箱是否已被注册，至少传入一个参数
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Param        username  query     string  false  "用户名"
// @Param        email     query     string  false  "邮箱"
// @Success      200       {object}  response.Response{data=dto.CheckExistenceResponse}  "查询成功"
// @Failure      400       {object}  response.Response  "请求参数错误"
// @Router       /users/check [get]
func (h *UserHandler) CheckExistence(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.CheckExistenceRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	result, err := h.userService.CheckExistence(ctx, &req)
	if err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, result, "查询成功")
}

// Register godoc
// @Summary      用户注册
// @Description  新用户注册，需要先获取验证码
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Param        request  body      dto.RegisterRequest  true  "注册信息"
// @Success      200      {object}  response.Response    "注册成功"
// @Failure      400      {object}  response.Response    "请求参数错误"
// @Failure      409      {object}  response.Response    "用户已存在"
// @Router       /register [post]
func (h *UserHandler) Register(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}
	if err := h.userService.Register(ctx, &req); err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, nil, "注册成功，请检查邮箱完成验证") // 提示语可以根据未来逻辑调整
}

// Login godoc
// @Summary      用户登录
// @Description  用户登录获取访问令牌
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Param        request  body      dto.LoginRequest                            true  "登录信息"
// @Success      200      {object}  response.Response{data=dto.LoginResponse}   "登录成功"
// @Failure      400      {object}  response.Response                           "用户名或密码错误"
// @Failure      403      {object}  response.Response                           "用户状态异常"
// @Router       /login [post]
func (h *UserHandler) Login(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}
	result, err := h.userService.Login(ctx, &req)
	if err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, result, "登录成功")
}

// RefreshToken godoc
// @Summary      刷新访问令牌
// @Description  使用刷新令牌获取新的访问令牌
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Param        request  body      dto.RefreshTokenRequest                      true  "刷新令牌"
// @Success      200      {object}  response.Response{data=dto.LoginResponse}    "令牌刷新成功"
// @Failure      401      {object}  response.Response                            "令牌无效或已过期"
// @Router       /users/token/refresh [post]
func (h *UserHandler) RefreshToken(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	result, err := h.userService.RefreshToken(ctx, &req)
	if err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, result, "令牌刷新成功")
}

// Logout godoc
// @Summary      退出登录
// @Description  用户退出登录，使当前令牌失效
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response  "退出登录成功"
// @Failure      401  {object}  response.Response  "用户未认证"
// @Router       /users/logout [post]
func (h *UserHandler) Logout(c *gin.Context) {
	ctx := c.Request.Context()
	// 由于加了 auth 中间件，这里一定能获取到用户信息
	userInfo := jwt.GetUserInfo(c)
	token := c.GetHeader("Authorization")

	// 调用service层处理登出逻辑
	if err := h.userService.Logout(ctx, userInfo.ID, token); err != nil {
		handleUserServiceError(c, err)
		return
	}

	logger.Info("用户登出成功", "userInfo", userInfo)
	response.Ok(c, nil, "退出登录成功")
}

// ForceLogout godoc
// @Summary      强制用户下线
// @Description  管理员强制指定用户下线
// @Tags         管理员-用户管理
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int                true  "用户ID"
// @Success      200  {object}  response.Response  "强制下线成功"
// @Failure      400  {object}  response.Response  "无效的用户ID"
// @Failure      401  {object}  response.Response  "用户未认证"
// @Failure      403  {object}  response.Response  "无权访问"
// @Router       /admin/users/{id}/session [delete]
func (h *UserHandler) ForceLogout(c *gin.Context) {
	ctx := c.Request.Context()
	
	// 从URL路径中获取目标用户ID
	targetUserID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, response.InvalidParam, "无效的用户ID")
		return
	}
	
	// 调用Service层执行核心逻辑
	if err := h.userService.ForceLogout(ctx, uint(targetUserID)); err != nil {
		handleUserServiceError(c, err)
		return
	}
	
	response.Ok(c, nil, "强制下线成功")
}

// Update godoc
// @Summary      更新用户信息
// @Description  更新指定用户的基本信息
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int                    true  "用户ID"
// @Param        request  body      dto.UpdateUserRequest  true  "更新信息"
// @Success      200      {object}  response.Response      "更新成功"
// @Failure      400      {object}  response.Response      "请求参数错误"
// @Failure      401      {object}  response.Response      "用户未认证"
// @Failure      403      {object}  response.Response      "无权访问"
// @Failure      404      {object}  response.Response      "用户不存在"
// @Router       /users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	targetUserID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, response.InvalidParam, "无效的用户ID")
		return
	}

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	if err := h.userService.Update(ctx, userInfo, uint(targetUserID), &req); err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, nil, "更新用户信息成功")
}

// UpdatePassword godoc
// @Summary      修改密码
// @Description  用户修改自己的密码
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int                        true  "用户ID"
// @Param        request  body      dto.UpdatePasswordRequest  true  "密码信息"
// @Success      200      {object}  response.Response          "密码修改成功"
// @Failure      400      {object}  response.Response          "请求参数错误"
// @Failure      401      {object}  response.Response          "用户未认证"
// @Failure      403      {object}  response.Response          "无权访问"
// @Router       /users/{id}/reset [put]
func (h *UserHandler) UpdatePassword(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	targetUserID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, response.InvalidParam, "无效的用户ID")
		return
	}
	
	var req dto.UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	if err := h.userService.UpdatePassword(ctx, userInfo, uint(targetUserID), &req); err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, nil, "密码修改成功")
}

// Delete godoc
// @Summary      删除用户
// @Description  删除指定用户（软删除）
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int                true  "用户ID"
// @Success      200  {object}  response.Response  "删除成功"
// @Failure      400  {object}  response.Response  "无效的用户ID"
// @Failure      401  {object}  response.Response  "用户未认证"
// @Failure      403  {object}  response.Response  "无权访问"
// @Failure      404  {object}  response.Response  "用户不存在"
// @Router       /users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	targetUserID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, response.InvalidParam, "无效的用户ID")
		return
	}

	if err := h.userService.Delete(ctx, userInfo, uint(targetUserID)); err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, nil, "删除成功")
}

// Get godoc
// @Summary      获取用户信息
// @Description  获取指定用户的详细信息
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int                                         true  "用户ID"
// @Success      200  {object}  response.Response{data=dto.UserResponse}    "获取成功"
// @Failure      400  {object}  response.Response                           "无效的用户ID"
// @Failure      401  {object}  response.Response                           "用户未认证"
// @Failure      403  {object}  response.Response                           "无权访问"
// @Failure      404  {object}  response.Response                           "用户不存在"
// @Router       /users/{id} [get]
func (h *UserHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c)
	targetUserID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, response.InvalidParam, "无效的用户ID")
		return
	}

	user, err := h.userService.Get(ctx, userInfo, uint(targetUserID))
	if err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, user, "获取用户信息成功")
}

// List godoc
// @Summary      获取用户列表
// @Description  管理员获取用户列表，支持分页和筛选
// @Tags         管理员-用户管理
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page      query     int     false  "页码"      default(1)
// @Param        pageSize  query     int     false  "每页数量"  default(10)
// @Param        username  query     string  false  "用户名（模糊查询）"
// @Param        status    query     int     false  "用户状态"
// @Success      200       {object}  response.Response{data=dto.UserListResponse}  "获取成功"
// @Failure      401       {object}  response.Response  "用户未认证"
// @Failure      403       {object}  response.Response  "无权访问"
// @Router       /admin/users [get]
func (h *UserHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userInfo := jwt.GetUserInfo(c) // 获取用户信息用于鉴权

	var req dto.ListUsersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	result, err := h.userService.List(ctx, userInfo, &req)
	if err != nil {
		handleUserServiceError(c, err)
		return
	}

	response.Ok(c, result, "获取用户列表成功")
}

// ForgotPassword godoc
// @Summary      忘记密码
// @Description  发送密码重置验证码到用户邮箱或手机
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Param        request  body      dto.ForgotPasswordRequest  true  "账号信息"
// @Success      200      {object}  response.Response          "验证码已发送"
// @Failure      400      {object}  response.Response          "请求参数错误"
// @Failure      404      {object}  response.Response          "用户不存在"
// @Router       /users/password/forgot [post]
func (h *UserHandler) ForgotPassword(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}
	if err := h.userService.ForgotPassword(ctx, &req); err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, nil, "重置密码验证码已发送，请注意查收")
}

// VerifyPasswordResetCaptcha godoc
// @Summary      验证重置密码验证码
// @Description  验证用于重置密码的验证码，返回重置令牌
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Param        request  body      dto.VerifyOnceCaptchaRequest                    true  "验证码信息"
// @Success      200      {object}  response.Response{data=dto.VerifyCaptchaResponse}  "验证成功"
// @Failure      400      {object}  response.Response                               "验证码错误"
// @Router       /users/password/verify [post]
func (h *UserHandler) VerifyPasswordResetCaptcha(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.VerifyOnceCaptchaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}
	result, err := h.userService.VerifyPasswordResetCaptcha(ctx, &req)
	if err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, result, "验证成功")
}

// ResetPassword godoc
// @Summary      重置密码
// @Description  使用重置令牌设置新密码
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Param        request  body      dto.ResetPasswordRequest  true  "重置信息"
// @Success      200      {object}  response.Response         "密码重置成功"
// @Failure      400      {object}  response.Response         "请求参数错误"
// @Failure      401      {object}  response.Response         "令牌无效或已过期"
// @Router       /users/password/reset [post]
func (h *UserHandler) ResetPassword(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}
	if err := h.userService.ResetPassword(ctx, &req); err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, nil, "密码重置成功")
}

// UpdateUserStatus godoc
// @Summary      更新用户状态
// @Description  管理员更新用户状态（正常/禁用/锁定）
// @Tags         管理员-用户管理
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int                          true  "用户ID"
// @Param        request  body      dto.UpdateUserStatusRequest  true  "状态信息"
// @Success      200      {object}  response.Response            "更新成功"
// @Failure      400      {object}  response.Response            "请求参数错误"
// @Failure      401      {object}  response.Response            "用户未认证"
// @Failure      403      {object}  response.Response            "无权访问"
// @Failure      404      {object}  response.Response            "用户不存在"
// @Router       /admin/users/{id}/status [put]
func (h *UserHandler) UpdateUserStatus(c *gin.Context) {
	ctx := c.Request.Context()
	
	// 从URL中获取目标用户ID
	targetUserID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, response.InvalidParam, "无效的用户ID")
		return
	}

	var req dto.UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	// 调用Service（注意：这里不再需要传入操作者userInfo，因为权限已由中间件处理）
	if err := h.userService.UpdateUserStatus(ctx, uint(targetUserID), &req); err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, nil, "用户状态更新成功")
}

// RequestRecovery godoc
// @Summary      请求恢复账号
// @Description  发送账号恢复验证码
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Param        request  body      dto.RequestRecoveryRequest  true  "账号信息"
// @Success      200      {object}  response.Response           "验证码已发送"
// @Failure      400      {object}  response.Response           "请求参数错误"
// @Router       /users/recovery/request [post]
func (h *UserHandler) RequestRecovery(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.RequestRecoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validator.HandleBindingError(c, err)
		return
	}
	if err := h.userService.RequestRecovery(ctx, &req); err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, nil, "恢复账号验证码已发送，请注意查收")
}

// VerifyRecoveryCaptcha godoc
// @Summary      验证恢复验证码
// @Description  验证账号恢复验证码，返回恢复令牌
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Param        request  body      dto.VerifyRecoveryCaptchaRequest                       true  "验证码信息"
// @Success      200      {object}  response.Response{data=dto.VerifyRecoveryCaptchaResponse}  "验证成功"
// @Failure      400      {object}  response.Response                                      "验证码错误"
// @Router       /users/recovery/verify [post]
func (h *UserHandler) VerifyRecoveryCaptcha(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.VerifyRecoveryCaptchaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validator.HandleBindingError(c, err)
		return
	}
	result, err := h.userService.VerifyRecoveryCaptcha(ctx, &req)
	if err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, result, "验证成功")
}

// ExecuteRecovery godoc
// @Summary      执行账号恢复
// @Description  使用恢复令牌执行账号恢复操作
// @Tags         用户模块
// @Accept       json
// @Produce      json
// @Param        request  body      dto.ExecuteRecoveryRequest  true  "恢复令牌"
// @Success      200      {object}  response.Response           "账号恢复成功"
// @Failure      400      {object}  response.Response           "请求参数错误"
// @Failure      401      {object}  response.Response           "令牌无效或已过期"
// @Router       /users/recovery/execute [post]
func (h *UserHandler) ExecuteRecovery(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.ExecuteRecoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validator.HandleBindingError(c, err)
		return
	}
	if err := h.userService.ExecuteRecovery(ctx, &req); err != nil {
		handleUserServiceError(c, err)
		return
	}
	response.Ok(c, nil, "账号恢复成功")
}