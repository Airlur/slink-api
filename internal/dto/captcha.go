package dto

// SendCaptchaRequest 发送验证码请求
type SendCaptchaRequest struct {
	Scene   string `json:"scene" binding:"required"`                 // 场景 (register/login/reset_pwd等)
	Account string `json:"account" binding:"required"`               // 手机号或邮箱
	Type    string `json:"type,omitempty" binding:"omitempty,oneof=sms email"` // 类型，默认为email
}

// SendCaptchaResponse 发送验证码响应
type SendCaptchaResponse struct {
	ExpireSecond   int `json:"expire_second"`   // 验证码有效期（秒）
	NextSendSecond int `json:"next_send_second"` // 下次可发送的冷却时间（秒）
}

// VerifyCaptchaRequest 验证验证码请求
type VerifyCaptchaRequest struct {
	Scene   string `json:"scene" binding:"required"`
	Account string `json:"account" binding:"required"`
	Captcha string `json:"captcha" binding:"required,len=6"` // 验证码通常为6位
}