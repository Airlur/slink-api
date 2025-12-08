package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"short-link/internal/dto"
	"short-link/internal/pkg/config"
	bizErrors "short-link/internal/pkg/errors"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/redis"
	"short-link/internal/pkg/response"
	"short-link/internal/model"
	"short-link/internal/repository"

	goRedis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// --- 接口与模拟客户端定义 ---

// SmsClient 定义了短信客户端的接口
type SmsClient interface {
	Send(ctx context.Context, phone string, content string) error
}

// EmailClient 定义了邮件客户端的接口
type EmailClient interface {
	Send(ctx context.Context, email, subject, body string) error
	SendVerificationCode(ctx context.Context, to, code string, expirationMinutes int) error
}

// MockSmsClient 是一个用于开发的模拟短信客户端
type MockSmsClient struct{}
func (m *MockSmsClient) Send(ctx context.Context, phone string, content string) error {
	logger.Info("模拟发送短信", "phone", phone, "content", content)
	return nil
}

// MockEmailClient 是一个用于开发的模拟邮件客户端
type MockEmailClient struct{}
func (m *MockEmailClient) Send(ctx context.Context, email, subject, body string) error {
	logger.Info("模拟发送邮件", "email", email, "subject", subject, "body", body)
	return nil
}


// --- Service 定义与实现 ---

type CaptchaService interface {
	SendCaptcha(ctx context.Context, req *dto.SendCaptchaRequest) (*dto.SendCaptchaResponse, error)
	VerifyCaptcha(ctx context.Context, scene, account, captcha string) error
}

type captchaService struct {
	userRepo    repository.UserRepository
	smsClient   SmsClient
	emailClient EmailClient
}

func NewCaptchaService(userRepo repository.UserRepository, smsCli SmsClient, emailCli EmailClient) CaptchaService {
	return &captchaService{
		userRepo:    userRepo,
		smsClient:   smsCli,
		emailClient: emailCli,
	}
}

// validScenes 定义了支持的业务场景白名单
var validScenes = map[string]struct{}{
	"register":    		{}, // 注册
	"login":       		{}, // 登录
	"reset_password":   {}, // 忘记密码，重置密码
	"recover_account":  {}, // 账号注销，自助恢复
}

// SendCaptcha 实现了发送验证码的完整业务逻辑
func (s *captchaService) SendCaptcha(ctx context.Context, req *dto.SendCaptchaRequest) (*dto.SendCaptchaResponse, error) {
	// 1. 场景合法性校验
	if _, ok := validScenes[req.Scene]; !ok {
		return nil, bizErrors.New(response.InvalidParam, "不支持的业务场景")
	}
	if req.Type == "" { req.Type = "email" }
	
	// 2. 账号格式校验
	if err := s.validateAccount(req.Account, req.Type); err != nil { return nil, err }

	// 3. 多级频率限制校验
	if err := s.checkFrequency(ctx, req.Account, req.Scene); err != nil { return nil, err }
	
	// 4. 【核心】账号存在性校验 (根据场景)
	if err := s.checkAccountExistence(ctx, req.Account, req.Type, req.Scene); err != nil { return nil, err }

	// 5. 生成验证码
	captcha := generateNumericCode(6)

	// 6. 发送验证码
	captchaKey := fmt.Sprintf("captcha:%s:%s", req.Scene, req.Account)
	captchaTTL := time.Duration(config.GlobalConfig.Captcha.ExpirationMinutes) * time.Minute
	if err := redis.Set(ctx, captchaKey, captcha, captchaTTL); err != nil {
		logger.Error("验证码缓存失败", "error", err, "key", captchaKey)
		return nil, bizErrors.New(response.InternalError, "系统繁忙，请稍后再试")
	}

	if err := s.send(ctx, req.Type, req.Account, captcha); err != nil {
		return nil, bizErrors.New(response.InternalError, "验证码发送失败，请稍后重试")
	}

	return &dto.SendCaptchaResponse{
		ExpireSecond:   int(captchaTTL.Seconds()),
		NextSendSecond: config.GlobalConfig.Captcha.CooldownSeconds,
	}, nil
}

// VerifyCaptcha 实现了验证验证码的逻辑
func (s *captchaService) VerifyCaptcha(ctx context.Context, scene, account, captcha string) error {
	captchaKey := fmt.Sprintf("captcha:%s:%s", scene, account)
	
	// 从Redis获取缓存的验证码
	storedCaptcha, err := redis.Get(ctx, captchaKey)
	if err != nil {
		if errors.Is(err, goRedis.Nil) {
			return bizErrors.New(response.InvalidParam, "验证码已过期或不存在")
		}
		logger.Error("从Redis获取验证码失败", "error", err, "key", captchaKey)
		return bizErrors.New(response.InternalError, "验证失败，请稍后再试")
	}

	// 对比验证码
	if storedCaptcha != captcha {
		return bizErrors.New(response.InvalidParam, "验证码错误")
	}

	// 【关键】验证成功后，立即删除验证码，防止重复使用
	if err := redis.Del(ctx, captchaKey); err != nil {
		// 即使删除失败，也应认为验证成功，但必须记录严重日志
		logger.Error("删除已使用的验证码失败", "error", err, "key", captchaKey)
	}

	return nil
}

// ================ 辅助函数 ================
// validateAccount 校验账号格式
func (s *captchaService) validateAccount(account, typ string) error {
	if typ == "sms" {
		if !regexp.MustCompile(`^1[3-9]\d{9}$`).MatchString(account) {
			return bizErrors.New(response.InvalidParam, "手机号格式错误")
		}
	} else { // email
		if !regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).MatchString(account) {
			return bizErrors.New(response.InvalidParam, "邮箱格式错误")
		}
	}
	return nil
}

// checkFrequency 实现了分钟/小时/天三级频率限制
// TODO: 是不是还要有多个维度的限流，账号，设备，IP，系统全局？可以后面考虑完善
func (s *captchaService) checkFrequency(ctx context.Context, account, scene string) error {
	cfg := config.GlobalConfig.Captcha
	
	// 1分钟内只能发送1次的冷却限制
	cooldownKey := fmt.Sprintf("ratelimit:captcha:cooldown:%s:%s", scene, account)
	ttl, err := redis.Client.TTL(ctx, cooldownKey).Result()
	if err != nil && !errors.Is(err, goRedis.Nil) {
		logger.Error("检查验证码冷却时间失败", "error", err, "key", cooldownKey)
		return bizErrors.New(response.InternalError, "系统繁忙")
	}
	if ttl > -1 {
		return bizErrors.New(response.TooManyRequests, fmt.Sprintf("操作过于频繁，请在 %d 秒后重试", int(ttl.Seconds())))
	}

	// 分钟/小时/天 总次数限制
	minKey := fmt.Sprintf("ratelimit:captcha:min:%s:%s", scene, account)
	hourKey := fmt.Sprintf("ratelimit:captcha:hour:%s:%s", scene, account)
	dayKey := fmt.Sprintf("ratelimit:captcha:day:%s:%s", scene, account)

	minCount, err := redis.IncrWithExpiration(ctx, minKey, 1*time.Minute)
	if err != nil { return bizErrors.New(response.InternalError, "系统繁忙") }
	if minCount > int64(cfg.LimitPerMinute) {
		return bizErrors.New(response.TooManyRequests, "操作过于频繁，请稍后再试")
	}

	hourCount, err := redis.IncrWithExpiration(ctx, hourKey, 1*time.Hour)
	if err != nil { return bizErrors.New(response.InternalError, "系统繁忙") }
	if hourCount > int64(cfg.LimitPerHour) {
		return bizErrors.New(response.TooManyRequests, "本小时发送次数已达上限")
	}

	dayCount, err := redis.IncrWithExpiration(ctx, dayKey, 24*time.Hour)
	if err != nil { return bizErrors.New(response.InternalError, "系统繁忙") }
	if dayCount > int64(cfg.LimitPerDay) {
		return bizErrors.New(response.TooManyRequests, "今日发送次数已达上限")
	}
	
	// 通过所有频率限制后，设置冷却锁
	cooldownTTL := time.Duration(cfg.CooldownSeconds) * time.Second
	redis.Set(ctx, cooldownKey, "1", cooldownTTL)

	return nil
}

// checkAccountExistence 根据场景校验账号是否存在
func (s *captchaService) checkAccountExistence(ctx context.Context, account, typ, scene string) error {
	// 准备查询条件
	conditions := &model.User{}
	if typ == "email" {
		conditions.Email = account
	} else {
		conditions.Phone = account
	}

	/* 核心：根据场景决定是否查询已注销用户。注册、恢复场景需要检查所有记录，登录、重置密码等场景只检查有效记录
	* 注册时 (register)： 只要数据库里物理存在这个账号（无论是否已注销），就意味着“已存在”，不允许注册。
	* 登录时 (login)： 用户必须存在，且未被注销，且状态正常。
	* 忘记密码时 (reset_password)： 用户必须存在，且未被注销，但状态可能是正常的或被锁定的。
	* 自助恢复时 (recover_account)： 用户必须存在，但状态必须是“已注销”。 
	*/

	shouldUnscoped := (scene == "register" || scene == "recover_account")
	
	// 这里我们需要一个新的、更灵活的Repo方法
	user, err := s.userRepo.FindOne(ctx, conditions, shouldUnscoped)

	// --- 精确的业务逻辑判断 ---

	// 处理查询时可能发生的其他数据库错误
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("校验账号存在性时数据库查询失败", "error", err, "account", account)
		return bizErrors.New(response.InternalError, "系统繁忙，请稍后再试")
	}

	userExists := (err == nil)

	// 场景：注册
	if scene == "register" {
		if userExists {
			return bizErrors.New(response.UserAlreadyExist, "该账号已被注册")
		}
	} 

	// 场景：登录 或 重置密码
	if scene == "login" || scene == "reset_password" {
		if !userExists {
			return bizErrors.New(response.UserNotFound, "该账号未注册")
		}
		if user.Status != model.UserStatusNormal {
			return bizErrors.New(response.UserStatusError, "账号状态异常，无法执行此操作")
		}
	}

	// 场景：恢复账号
	if scene == "recover_account" {
		if !userExists {
			return bizErrors.New(response.UserNotFound, "该账号未注册")
		}
		if user.Status != model.UserStatusCancellation {
			return bizErrors.New(response.UserStatusError, "账号非注销状态，无法恢复")
		}
		// 【核心】检查是否在恢复宽限期内
		gracePeriod := time.Duration(config.GlobalConfig.User.RecoveryGracePeriodDays) * 24 * time.Hour
		if time.Since(user.DeletedAt.Time) > gracePeriod {
			return bizErrors.New(response.Forbidden, "账号已超出可恢复期限，已永久注销")
		}
	}

	return nil
}

// send 根据类型调用不同的客户端发送验证码
func (s *captchaService) send(ctx context.Context, typ, account, captcha string) error {
	if typ == "email" {
		// 使用 goroutine 异步发送，API 无需等待邮件发送完成
		go func() {
			expirationMinutes := config.GlobalConfig.Captcha.ExpirationMinutes
			err := s.emailClient.SendVerificationCode(context.Background(), account, captcha, expirationMinutes)
			if err != nil {
				logger.Error("异步发送验证码邮件失败", "error", err, "email", account)
			}
		}()
		return nil
	}
	return s.smsClient.Send(ctx, account, fmt.Sprintf("您的验证码是%s", captcha))
}

// generateNumericCode 是一个内部辅助函数，生成指定长度的数字验证码
func generateNumericCode(length int) string {
	const table = "1234567890"
	b := make([]byte, length)
	n, err := io.ReadAtLeast(rand.Reader, b, length)
	if n != length || err != nil {
		// Fallback for crypto/rand failure
		for i := range b {
			b[i] = table[time.Now().UnixNano()%int64(len(table))]
		}
	} else {
		for i := 0; i < len(b); i++ {
			b[i] = table[int(b[i])%len(table)]
		}
	}
	return string(b)
}