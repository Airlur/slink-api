package v1

import (
	"errors"

	"slink-api/internal/dto"
	bizErrors "slink-api/internal/pkg/errors"
	"slink-api/internal/pkg/logger"
	"slink-api/internal/pkg/response"
	"slink-api/internal/pkg/validator"
	"slink-api/internal/service"

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

// SendCaptcha 发送验证码
// @Summary      发送验证码
// @Description  向指定邮箱或手机发送验证码，用于注册、登录、重置密码等场景
// @Tags         验证码
// @Accept       json
// @Produce      json
// @Param        request  body      dto.SendCaptchaRequest  true  "发送验证码请求体"
// @Success      200      {object}  response.Response{data=dto.SendCaptchaResponse}
// @Failure      400      {object}  response.Response
// @Failure      429      {object}  response.Response  "发送过于频繁"
// @Failure      500      {object}  response.Response
// @Router       /captcha/send [post]
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

// VerifyCaptcha 验证验证码
// @Summary      验证验证码
// @Description  验证用户输入的验证码是否正确
// @Tags         验证码
// @Accept       json
// @Produce      json
// @Param        request  body      dto.VerifyCaptchaRequest  true  "验证请求体"
// @Success      200      {object}  response.Response
// @Failure      400      {object}  response.Response
// @Failure      401      {object}  response.Response  "验证码错误或已过期"
// @Failure      500      {object}  response.Response
// @Router       /captcha/verify [post]
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