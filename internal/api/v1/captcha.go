package v1

import (
	"errors"

	"short-link/internal/dto"
	bizErrors "short-link/internal/pkg/errors"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/response"
	"short-link/internal/pkg/validator"
	"short-link/internal/service"

	"github.com/gin-gonic/gin"
)

type CaptchaHandler struct {
	svc service.CaptchaService
}

func NewCaptchaHandler(svc service.CaptchaService) *CaptchaHandler {
	return &CaptchaHandler{svc: svc}
}

// handleCaptchaServiceError 是 Captcha 模块专属的错误处理器
func handleCaptchaServiceError(c *gin.Context, err error) {
	var bizErr *bizErrors.Error
	if errors.As(err, &bizErr) {
		response.Fail(c, bizErr.Code, bizErr.Message)
		return
	}

	logger.Error("未处理的验证码服务错误", "error", err)
	response.Fail(c, response.InternalError, "")
}

// SendCaptcha 处理发送验证码请求
func (h *CaptchaHandler) SendCaptcha(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.SendCaptchaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	resp, err := h.svc.SendCaptcha(ctx, &req)
	if err != nil {
		handleCaptchaServiceError(c, err)
		return
	}

	response.Ok(c, resp, "验证码已发送，请注意查收")
}

// VerifyCaptcha 处理验证验证码的请求
func (h *CaptchaHandler) VerifyCaptcha(c *gin.Context) {
	ctx := c.Request.Context()
	var req dto.VerifyCaptchaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 统一调用新的绑定错误处理器
		validator.HandleBindingError(c, err)
		return
	}

	err := h.svc.VerifyCaptcha(ctx, req.Scene, req.Account, req.Captcha)
	if err != nil {
		handleCaptchaServiceError(c, err)
		return
	}

	response.Ok(c, nil, "验证成功")
}