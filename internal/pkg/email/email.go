package email

import (
	"bytes"
	"context"
	"html/template"

	"slink-api/internal/pkg/config"
	"slink-api/internal/service" // 导入service包以获取EmailClient接口定义

	"gopkg.in/gomail.v2"
)

// verificationCodeTemplate 是一个专业的HTML邮件模板
const verificationCodeTemplate = `
<div style="font-family: Arial, 'Helvetica Neue', Helvetica, sans-serif; color: #333; line-height: 1.6;">
    <div style="max-width: 600px; margin: 20px auto; padding: 20px; border: 1px solid #ddd; border-radius: 5px;">
        <h2 style="text-align: center; color: #007BFF;">您的验证码</h2>
        <p>您好,</p>
        <p>您正在进行一项需要验证身份的操作，您的验证码是：</p>
        <p style="font-size: 24px; font-weight: bold; text-align: center; color: #FF5722; letter-spacing: 4px; margin: 20px 0; padding: 10px; background-color: #f9f9f9; border-radius: 3px;">
            {{.Code}}
        </p>
        <p>此验证码将在 <strong>{{.ExpirationMinutes}}分钟</strong> 内失效。为保障您的账户安全，请勿将此验证码泄露给他人。</p>
        <p>如果您没有请求此验证码，请忽略此邮件。</p>
        <hr style="border: none; border-top: 1px solid #ddd; margin: 20px 0;" />
        <p style="font-size: 12px; color: #999; text-align: center;">此邮件由 {{.SenderName}} 自动发送，请勿直接回复。</p>
    </div>
</div>
`

// SmtpClient 实现了 service.EmailClient 接口
type SmtpClient struct {
	dialer *gomail.Dialer
	cfg    *config.EmailConfig
}

// NewSmtpClient 创建一个真实的邮件客户端实例
func NewSmtpClient(cfg *config.EmailConfig) service.EmailClient {
	// 创建一个 Dialer 对象，用于连接SMTP服务器
	// 它会根据端口号自动处理SSL/TLS
	dialer := gomail.NewDialer(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	return &SmtpClient{
		dialer: dialer,
		cfg:    cfg,
	}
}

// Send 实现了发送邮件的方法
func (s *SmtpClient) Send(ctx context.Context, to, subject, body string) error {
	// 创建一封新邮件
	m := gomail.NewMessage()

	// 设置发件人，可以包含一个友好的名称
	m.SetAddressHeader("From", s.cfg.Username, s.cfg.SenderName)

	// 设置收件人
	m.SetHeader("To", to)

	// 设置邮件主题
	m.SetHeader("Subject", subject)

	// 设置邮件正文，类型为 text/html，可以支持HTML格式
	m.SetBody("text/html", body)

	// 使用 Dialer 连接服务器并发送邮件
	// DialAndSend 会自动处理连接、认证和发送的全过程
	return s.dialer.DialAndSend(m)
}

// SendVerificationCode 专门用于发送验证码邮件
func (s *SmtpClient) SendVerificationCode(ctx context.Context, to, code string, expirationMinutes int) error {
	// 1. 解析HTML模板
	tmpl, err := template.New("verificationEmail").Parse(verificationCodeTemplate)
	if err != nil {
		return err
	}

	// 2. 准备模板所需的数据
	data := struct {
		Code              string
		ExpirationMinutes int
		SenderName        string
	}{
		Code:              code,
		ExpirationMinutes: expirationMinutes,
		SenderName:        s.cfg.SenderName,
	}

	// 3. 渲染模板
	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return err
	}

	// 4. 调用通用的 Send 方法发送
	return s.Send(ctx, to, "您的验证码", body.String())
}