package v1

import (
	"errors"

	"short-link/internal/dto"
	bizErrors "short-link/internal/pkg/errors"
	"short-link/internal/pkg/jwt"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/validator"
	"short-link/internal/pkg/response"
	"short-link/internal/service"
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

// CheckExistence 检查用户名或邮箱是否存在
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

// Register 用户注册
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

// 用户登录 Login
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

// RefreshToken 刷新访问令牌
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

// Logout 退出登录
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

// ForceLogout 管理员强制用户下线
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

// Update 更新用户信息
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

// UpdatePassword 用户修改密码
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

// Delete 删除用户
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

// Get 根据用户ID查询用户
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

// List 查询所有用户
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

// ForgotPassword 忘记密码
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

// VerifyPasswordResetCaptcha 验证用于重置密码的验证码，并返回重置令牌
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

// ResetPassword 重置密码
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

// UpdateUserStatus 管理员更新用户状态
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

// RequestRecovery 请求恢复账号
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

// VerifyRecoveryCaptcha 验证恢复账号的验证码
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

// ExecuteRecovery 执行恢复账号
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