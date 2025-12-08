package dto

import (
	"time"

	"short-link/internal/dto/common"
)

// RegisterRequest 注册请求参数
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=32,password"`
	Account string `json:"account" binding:"required"`	// Account 是用于接收验证码和绑定的账号（邮箱或手机号）
	Type string `json:"type,omitempty" binding:"omitempty,oneof=email phone"`	// Type 指明了Account的类型
	Captcha  string `json:"captcha" binding:"required,len=6"`
}

// LoginRequest 登录请求参数
type LoginRequest struct {
	Username string `json:"username" binding:"required" `
	Password string `json:"password" binding:"required"`
}

// LoginResponse 是登录成功后返回的数据
type LoginResponse struct {
	AccessToken  string       `json:"accessToken"`  // 短期访问令牌
	RefreshToken string       `json:"refreshToken"` // 长期刷新令牌
	User         UserResponse `json:"user"`
}

// RefreshTokenRequest 是刷新Token接口的请求体
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// CheckExistenceRequest 定义了检查用户名/邮箱是否存在的查询参数
type CheckExistenceRequest struct {
	// 用指针：区分“参数未传（nil）”和“参数传了空值（*val == ""）”，避免误判
	// 关键：用 omitempty + required_without 实现“二选一非空”
	// omitempty：参数不存在时忽略（允许只传一个）
	// min=1：禁止空字符串（如 "username": ""）
	// required_without_all=Email 保证了当Email字段不存在时，Username字段必须存在
	Username *string `form:"username" binding:"omitempty,required_without_all=Email,min=1"` 
	// required_without_all=Username 保证了当Username字段不存在时，Email字段必须存在
	Email    *string `form:"email" binding:"omitempty,required_without_all=Username,min=1,email"`
}

// CheckExistenceResponse 定义了检查结果的响应体
type CheckExistenceResponse struct {
	Exists bool `json:"exists"`
}

// UpdateUserRequest 更新用户信息请求参数 (部分更新)
type UpdateUserRequest struct {
	Username *string `json:"username,omitempty" binding:"omitempty,min=3,max=32"`
	Nickname *string `json:"nickname" binding:"omitempty,max=32"`
	Avatar   *string `json:"avatar" binding:"omitempty,url"`
	Email    *string `json:"email" binding:"omitempty,email"`
	Phone    *string `json:"phone" binding:"omitempty,len=11"`
}

// UpdatePasswordRequest 修改密码请求参数
type UpdatePasswordRequest struct {
	OldPassword     string `json:"oldPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=6,max=32,password"`
}

// ForgotPasswordRequest 定义了忘记密码的请求体
type ForgotPasswordRequest struct {
	Account string `json:"account" binding:"required"` // 手机号或邮箱
	Type    string `json:"type,omitempty" binding:"omitempty,oneof=email phone"`
}

// VerifyOnceCaptchaRequest 验证码校验请求体 (可用于多种场景)
type VerifyOnceCaptchaRequest struct {
	Account string `json:"account" binding:"required"`
	Type    string `json:"type,omitempty" binding:"omitempty,oneof=email phone"`
	Captcha string `json:"captcha" binding:"required,len=6"`
}

// VerifyCaptchaResponse 验证码校验成功响应体
type VerifyCaptchaResponse struct {
	ResetToken string `json:"resetToken"` // 返回一次性的密码重置令牌
}


// ResetPasswordRequest 定义了重置密码的请求体
// 它现在使用 ResetToken 而不是验证码
type ResetPasswordRequest struct {
	ResetToken      string `json:"resetToken" binding:"required"`
	Password        string `json:"password" binding:"required,min=6,max=32,password"`
}

// UpdateUserStatusRequest 定义了管理员更新用户状态的请求体
type UpdateUserStatusRequest struct {
	// 状态值，必填，且必须是预定义的状态之一
	// oneof 的值应与 model/user.go 中的常量保持一致
	Status *int `json:"status" binding:"required,oneof=1 2 3"`
}

// RequestRecoveryRequest 定义了请求恢复账号验证码的请求体
type RequestRecoveryRequest struct {
	Account string `json:"account" binding:"required"`
	Type    string `json:"type,omitempty" binding:"omitempty,oneof=email phone"`
}

// VerifyRecoveryCaptchaRequest 验证恢复验证码的请求体
type VerifyRecoveryCaptchaRequest struct {
	Account string `json:"account" binding:"required"`
	Type    string `json:"type,omitempty" binding:"omitempty,oneof=email phone"`
	Captcha string `json:"captcha" binding:"required,len=6"`
}

// VerifyRecoveryCaptchaResponse 验证恢复验证码的响应体
type VerifyRecoveryCaptchaResponse struct {
	RecoveryToken string `json:"recoveryToken"` // 返回一次性的账号恢复令牌
}

// ExecuteRecoveryRequest 定义了执行账号恢复的请求体
type ExecuteRecoveryRequest struct {
	RecoveryToken string `json:"recoveryToken" binding:"required"`
}

// =================== 【管理员接口】===================
//  【管理员】ListUsersRequest 用户列表查询参数
type ListUsersRequest struct {
	common.PaginationRequest // 嵌入通用分页请求
	// --- 筛选字段 ---
	// 使用指针，以便区分“不筛选”和“筛选零值”
	Username *string `form:"username"` // 按用户名模糊查询
	Status   *int    `form:"status"`   // 按状态精确查询
}

// --- 响应 DTO ---

// UserResponse 是返回给客户端的用户信息，屏蔽了敏感字段
type UserResponse struct {
	ID          uint       `json:"id"`
	Username    string     `json:"username"`
	Nickname    string     `json:"nickname"`
	Avatar      string     `json:"avatar"`
	Email       string     `json:"email"`
	Phone       string     `json:"phone"`
	Status      int        `json:"status"`
	Role        int        `json:"role"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}

