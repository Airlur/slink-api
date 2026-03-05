package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"slink-api/internal/dto"
	"slink-api/internal/dto/common"
	"slink-api/internal/model"
	"slink-api/internal/pkg/config"
	bizErrors "slink-api/internal/pkg/errors"
	"slink-api/internal/pkg/jwt"
	"slink-api/internal/pkg/logger"
	"slink-api/internal/pkg/redis"
	"slink-api/internal/pkg/response"
	"slink-api/internal/repository"

	"github.com/google/uuid"
	goRedis "github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService interface {
	CheckExistence(ctx context.Context, req *dto.CheckExistenceRequest) (*dto.CheckExistenceResponse, error)
	Register(ctx context.Context, req *dto.RegisterRequest) error
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error)	
	RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest) (*dto.LoginResponse, error)
	Logout(ctx context.Context, userID uint, token string) error
	ForceLogout(ctx context.Context, targetUserID uint) error
	Update(ctx context.Context, userInfo *jwt.UserInfo, targetUserID uint, req *dto.UpdateUserRequest) error
	UpdatePassword(ctx context.Context, userInfo *jwt.UserInfo, targetUserID uint, req *dto.UpdatePasswordRequest) error
	Delete(ctx context.Context, userInfo *jwt.UserInfo, targetUserID uint) error
	Get(ctx context.Context, userInfo *jwt.UserInfo, targetUserID uint) (*dto.UserResponse, error)
	List(ctx context.Context, userInfo *jwt.UserInfo, req *dto.ListUsersRequest) (*common.PaginatedData[*dto.UserResponse], error)
	
	ForgotPassword(ctx context.Context, req *dto.ForgotPasswordRequest) error
	VerifyPasswordResetCaptcha(ctx context.Context, req *dto.VerifyOnceCaptchaRequest) (*dto.VerifyCaptchaResponse, error)
	ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) error
	UpdateUserStatus(ctx context.Context, targetUserID uint, req *dto.UpdateUserStatusRequest) error
	
	RequestRecovery(ctx context.Context, req *dto.RequestRecoveryRequest) error
	VerifyRecoveryCaptcha(ctx context.Context, req *dto.VerifyRecoveryCaptchaRequest) (*dto.VerifyRecoveryCaptchaResponse, error)
	ExecuteRecovery(ctx context.Context, req *dto.ExecuteRecoveryRequest) error
	// TODO: 批量禁用接口，后面有需求有时间再加吧
}

type userService struct {
	db       	 *gorm.DB // 注入 gorm.DB 以便开启事务
	userRepo 	 repository.UserRepository
	captchaSvc   CaptchaService
}

func NewUserService(db *gorm.DB, userRepo repository.UserRepository, captchaSvc CaptchaService) UserService {
	return &userService{
		db:       	  db,
		userRepo: 	  userRepo,
		captchaSvc:   captchaSvc,
	}
}

// getAndCheckOwnership 是一个内部辅助函数，用于获取用户并校验操作权限
func (s *userService) getAndCheckOwnership(ctx context.Context, operator *jwt.UserInfo, targetUserID uint) (*model.User, error) {
	// 管理员或用户本人可以操作
	if operator.Role != model.UserRoleAdmin && operator.ID != targetUserID {
		return nil, bizErrors.New(response.Forbidden, "无权操作")
	}

	// 常规操作只应针对未被软删除的活跃用户
	user, err := s.userRepo.FindOne(ctx, &model.User{Model: gorm.Model{ID: targetUserID}}, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, bizErrors.New(response.UserNotFound, "用户不存在")
		}
		logger.Error("查询用户失败", "error", err, "targetUserID", targetUserID)
		return nil, bizErrors.New(response.InternalError, "获取用户信息失败")
	}
	return user, nil
}

// CheckExistence 检查用户名或邮箱是否存在
// TODO: 需要限流,不能无限制的查询
func (s *userService) CheckExistence(ctx context.Context, req *dto.CheckExistenceRequest) (*dto.CheckExistenceResponse, error) {
	// 必须至少提供一个查询条件
	if req.Username == nil && req.Email == nil {
		return nil, bizErrors.New(response.InvalidParam, "请提供用户名或邮箱")
	}

	// 优先检查用户名
	if req.Username != nil {
		// 检查用户名是否可用，需要扫描所有记录（包括已注销的）
		_, err := s.userRepo.FindOne(ctx, &model.User{Username: *req.Username}, true)
		if err == nil {
			return &dto.CheckExistenceResponse{Exists: true}, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("通过用户名检查存在性失败", "error", err, "username", *req.Username)
			return nil, bizErrors.New(response.InternalError, "查询失败")
		}
	}

	// 检查邮箱
	if req.Email != nil {
		_, err := s.userRepo.FindOne(ctx, &model.User{Email: *req.Email}, true)
		if err == nil { // 找到了用户，说明已存在
			return &dto.CheckExistenceResponse{Exists: true}, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) { // 如果是其他数据库错误
			logger.Error("通过邮箱检查存在性失败", "error", err, "email", *req.Email)
			return nil, bizErrors.New(response.InternalError, "查询失败")
		}
	}

	// 如果所有检查都未找到，则说明不存在
	return &dto.CheckExistenceResponse{Exists: false}, nil
}

// Register 用户注册
func (s *userService) Register(ctx context.Context, req *dto.RegisterRequest) error {
	// 默认注册类型为 email
	if req.Type == "" {
		req.Type = "email"
	}

	// 1. 【核心】验证验证码，使用通用的 Account
	err := s.captchaSvc.VerifyCaptcha(ctx, "register", req.Account, req.Captcha)
	if err != nil {
		return err
	}

	// 2. 检查用户名是否已存在
	_, err = s.userRepo.FindOne(ctx, &model.User{Username: req.Username}, true)
	if err == nil { // 查到了，说明用户名已存在
		return bizErrors.New(response.UserAlreadyExist, "用户名已存在")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) { // 如果是其他数据库错误
		logger.Error("注册时检查用户名失败", "error", err, "username", req.Username)
		return bizErrors.New(response.InternalError, "注册失败，请稍后再试")
	}
	// 3. 根据类型，检查【账号(Account)】是否已存在
	var checkErr error
	if req.Type == "email" {
		_, checkErr = s.userRepo.FindOne(ctx, &model.User{Email: req.Account}, true)
		if checkErr == nil {
			return bizErrors.New(response.UserAlreadyExist, "该邮箱已被注册")
		}
	} else {
		_, checkErr = s.userRepo.FindOne(ctx, &model.User{Phone: req.Account}, true)
		if checkErr == nil {
			return bizErrors.New(response.UserAlreadyExist, "该手机号已被注册")
		}
	}

	if !errors.Is(checkErr, gorm.ErrRecordNotFound) {
		logger.Error("注册时检查账号失败", "error", checkErr, "account", req.Account)
		return bizErrors.New(response.InternalError, "注册失败，请稍后再试")
	}

	// 4. 密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("密码加密失败", "error", err)
		return bizErrors.New(response.InternalError, "注册失败，请稍后再试")
	}

	// 5. 创建用户，根据类型填充不同字段
	user := &model.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Status:   model.UserStatusNormal, // 默认状态为正常
		Role:     model.UserRoleNormal,   // 默认角色为普通用户
	}
	if req.Type == "email" {
		user.Email = req.Account
	} else {
		user.Phone = req.Account
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		// 此处不再需要检查唯一键冲突，因为前面已经检查过了
		logger.Error("创建用户失败", "error", err)
		return bizErrors.New(response.InternalError, "注册失败，请稍后再试")
	}
	
	return nil
}

// Login 用户登录
func (s *userService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	lockoutKey := fmt.Sprintf("lockout:user:%s", req.Username)
	loginAttemptsKey := fmt.Sprintf("ratelimit:login_attempts:%s", req.Username)

	// 1. 首先检查账户是否已被Redis锁定
	if redis.Exists(ctx, lockoutKey) {
		ttl, _ := redis.Client.TTL(ctx, lockoutKey).Result()
		// 如果剩余时间大于1分钟，则以分钟显示
		var message string
		if ttl.Minutes() >= 1 {
			message = fmt.Sprintf("账号已被锁定，请在 %.0f 分钟后重试", math.Ceil(ttl.Minutes()))
		} else {
			// 否则，以秒显示
			message = fmt.Sprintf("账号已被锁定，请在 %.0f 秒后重试", math.Ceil(ttl.Seconds()))
		}
		return nil, bizErrors.New(response.UserStatusError, message)
	}
	
	// 2. 查询用户（只查找未被软删除的）
	user, err := s.userRepo.FindOne(ctx, &model.User{Username: req.Username}, true)
	// 2. 对查询结果进行精细化错误处理
	if err != nil {
		// a. 如果错误是“未找到”，为了安全，依然返回模糊提示
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, bizErrors.New(response.PasswordError, "用户名或密码错误")
		}	// b. 如果是其他未知数据库错误，记录日志并返回内部错误
		logger.Error("登录时查询用户失败", "error", err, "username", req.Username)
		return nil, bizErrors.New(response.InternalError, "登录失败，请稍后再试")
	}

	// 3. 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		// 密码错误，增加失败计数
		lockoutMinutes := config.GlobalConfig.Security.AccountLock.LockoutMinutes
		lockoutDuration := time.Duration(lockoutMinutes) * time.Minute
		count, _ := redis.IncrWithExpiration(ctx, loginAttemptsKey, lockoutDuration) // TODO：使用lua脚本改造，确保原子性
		maxFailures := int64(config.GlobalConfig.Security.AccountLock.MaxLoginFailures)
		
		remaining := maxFailures - count // 计算剩余尝试次数
		// 根据失败次数生成提示信息
		var errMsg string
		if remaining > 0 {
			errMsg = fmt.Sprintf("用户名或密码错误，还可尝试%d次，失败后将锁定%d分钟", remaining, lockoutMinutes)
		} else {
			errMsg = fmt.Sprintf("失败次数过多，账号已锁定，%d分钟后自动解锁", lockoutMinutes)
		}
		// 如果达到失败次数阈值，则正式锁定
		if count >= maxFailures {
			if err := redis.Set(ctx, lockoutKey, "1", lockoutDuration); err != nil {
				logger.Error("设置用户锁定状态到Redis失败", "error", err, "username", req.Username)
			}
			redis.Del(ctx, loginAttemptsKey) // 锁定后删除计数器
			return nil, bizErrors.New(response.UserStatusError, errMsg)
		}

		return nil, bizErrors.New(response.PasswordError, "用户名或密码错误")
	}

	// 4. 在比对密码前，先检查用户状态
	if user.Status != model.UserStatusNormal {
		// 根据不同状态给出明确提示
		switch user.Status {
		case model.UserStatusBanned:
			return nil, bizErrors.New(response.UserStatusError, "您的账号已被禁用，请联系管理员")
		case model.UserStatusLocked:
			return nil, bizErrors.New(response.UserStatusError, "您的账号已被锁定，请稍后再试")
		case model.UserStatusPending:
			return nil, bizErrors.New(response.UserStatusError, "您的账号尚未激活，请检查邮箱")
		case model.UserStatusCancellation:
			// 此时用户账号存在，但账号已注销，给出明确提示
			return nil, bizErrors.New(response.UserStatusError, "您的账号已注销，无法登录")
		default:
			return nil, bizErrors.New(response.UserStatusError, "账号状态异常，无法登录")
		}
	}

	// 5. 登录成功，清除失败计数器
	redis.Del(ctx, loginAttemptsKey)

	// 6. 生成Token并更新最后登录时间
	accessToken, refreshToken, err := jwt.GenerateTokens(ctx, user)
	if err != nil {
		logger.Error("生成token失败", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.GenerateTokenFailed, "登录失败")
	}
	logger.Info("登录成功server:", "user_id", user.ID)

	// 7. 异步更新用户最后登录时间
	go s.updateLastLoginAt(user.ID)

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:  *convertUserToDTO(user),
	}, nil
}

// updateLastLoginAt 是一个内部辅助函数，用于异步更新登录时间
func (s *userService) updateLastLoginAt(userID uint) {
	// 使用一个新的background context，因为它在独立的goroutine中运行
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates := map[string]interface{}{"last_login_at": time.Now()}
	if err := s.userRepo.Update(ctx, userID, updates); err != nil {
		// 在后台任务中，我们只记录日志，不影响主流程
		logger.Error("异步更新用户最后登录时间失败", "error", err, "userID", userID)
	}
}

// RefreshToken 刷新Token
func (s *userService) RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest) (*dto.LoginResponse, error) {
	// 1. 调用 jwt 包解析 Refresh Token
	claims, err := jwt.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		// 无论何种解析错误，都返回同样的提示，防止信息泄露
		return nil, bizErrors.New(response.InvalidToken, "无效的Refresh Token")
	}

	// 2. 调用 jwt 包检查 Refresh Token 是否在Redis中仍然活跃
	if !jwt.IsRefreshTokenActive(ctx, req.RefreshToken, claims.UserID) {
		return nil, bizErrors.New(response.InvalidToken, "Refresh Token已失效或被取代")
	}

	// 3. 【安全】从数据库获取最新的用户信息，以确保角色等信息是最新的, 刷新Token的用户必须是活跃用户
	user, err := s.userRepo.FindOne(ctx, &model.User{Model: gorm.Model{ID: claims.UserID}}, false)
	if err != nil {
		// 用户
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("刷新Token时用户记录不存在", 
				"userID", claims.UserID, 
				"token", req.RefreshToken[:10]+"***",  // 脱敏
				"clientIP", ctx.Value("client_ip"),    // 记录请求IP
				"userAgent", ctx.Value("user_agent"),  // 记录设备信息
			)  
			return nil, bizErrors.New(response.UserNotFound, "登录状态已失效，请重新登录")
		}
		// 数据库查询异常（如连接超时、权限问题）
		logger.Error("刷新Token时查询用户信息失败", "error", err, "userID", claims.UserID)
		// 用户提示：系统问题统一提示，避免技术术语
		return nil, bizErrors.New(response.InternalError, "服务暂时不不可用，请稍后重试")
	}

	// 在刷新Token时，也必须检查用户状态
	if user.Status != model.UserStatusNormal {
		// 如果用户在登录期间被管理员禁用，刷新Token时应立即失败
		return nil, bizErrors.New(response.UserStatusError, "登录状态已失效，请重新登录")
	}

	// 4. 调用 jwt 包生成新的一对Token（这会自动覆盖Redis中旧的RefreshToken，实现轮换）
	accessToken, refreshToken, err := jwt.GenerateTokens(ctx, user)
	if err != nil {
		logger.Error("刷新时生成新Tokens失败", "error", err, "userID", user.ID)
		return nil, bizErrors.New(response.GenerateTokenFailed, "刷新令牌失败，请重新登录")
	}
	
	// 5. 封装并返回新的Token和用户信息
	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         *convertUserToDTO(user),
	}, nil
}

// Logout 用户登出
func (s *userService) Logout(ctx context.Context, userID uint, token string) error {
	// 记录用户登出日志
	logger.Info("用户登出", "userID:", userID, ",token:", token)

	// 使 token 失效（这会删除活跃token并加入黑名单）
	if err := jwt.InvalidateToken(ctx, token); err != nil {
		// InvalidateToken 内部已经记录了详细日志
		return bizErrors.New(response.InternalError, "退出登录失败")
	}

	return nil
}

// ForceLogout 【管理员】强制用户下线
func (s *userService) ForceLogout(ctx context.Context, targetUserID uint) error {
	// 1. 检查目标用户是否存在
	// 强制下线的用户应该是当前活跃的用户
	targetUser, err := s.userRepo.FindOne(ctx, &model.User{Model: gorm.Model{ID: targetUserID}}, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 即使目标用户不存在，从API层面看，操作也可以视为“成功”（因为最终状态是该用户无会话）
			// 但为了接口的严谨性，我们明确返回错误
			return bizErrors.New(response.UserNotFound, "目标用户不存在")
		}
		// 记录其他未知的数据库错误
		logger.Error("强制下线时查询目标用户失败", "error", err, "targetUserID", targetUserID)
		return bizErrors.New(response.InternalError, "操作失败")
	}

	// 2. 【业务规则】禁止管理员强制登出其他管理员
	if targetUser.Role == model.UserRoleAdmin {
		return bizErrors.New(response.Forbidden, "不允许强制登出其他管理员")
	}

	// 3. 【核心逻辑】删除存储在Redis中的RefreshToken，使其所有会话失效
	refreshTokenKey := fmt.Sprintf("%s%d", jwt.TokenActivePrefix, targetUserID)
	if err := redis.Del(ctx, refreshTokenKey); err != nil {
		// Redis操作失败是严重的内部错误，需要记录日志
		logger.Error("强制下线用户（删除RefreshToken）失败", "error", err, "userID", targetUserID)
		return bizErrors.New(response.InternalError, "操作失败")
	}

	return nil
}

// Update 更新用户信息
func (s *userService) Update(ctx context.Context, userInfo *jwt.UserInfo, targetUserID uint, req *dto.UpdateUserRequest) error {
	// 1. 获取并校验所有权
	if _, err := s.getAndCheckOwnership(ctx, userInfo, targetUserID); err != nil {
		return err
	}
	
	// 2. 构建更新map
	updates := make(map[string]interface{})

	// 使用指针的优势在这里体现：只有当 req 中的字段不为 nil 时，才加入更新 map
	// 这可以区分 "用户没传这个字段" 和 "用户想把这个字段设置为空值" 的情况
	if req.Username != nil {
		existingUser, err := s.userRepo.FindOne(ctx, &model.User{Username: *req.Username}, true)
		if err == nil && existingUser.ID != targetUserID {
			return bizErrors.New(response.UserAlreadyExist, "用户名已存在")
		}
		updates["username"] = *req.Username
	}
	if req.Nickname != nil {
		updates["nickname"] = *req.Nickname
	}
	if req.Avatar != nil {
		updates["avatar"] = *req.Avatar
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	
	// 如果没有任何需要更新的字段，直接返回成功
	if len(updates) == 0 {
		return nil
	}
	
	// 3. 执行更新
	if err := s.userRepo.Update(ctx, targetUserID, updates); err != nil {
		logger.Error("更新用户信息失败", "error", err, "userID", targetUserID)
		return bizErrors.New(response.InternalError, "更新用户信息失败")
	}
	return nil
}

// UpdatePassword 修改密码
func (s *userService) UpdatePassword(ctx context.Context, userInfo *jwt.UserInfo, targetUserID uint, req *dto.UpdatePasswordRequest) error {
	user, err := s.getAndCheckOwnership(ctx, userInfo, targetUserID)
	if err != nil {
		return err
	}
	
	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return bizErrors.New(response.PasswordError, "原密码错误")
	}
	
	// 新密码不能与旧密码相同
    if req.NewPassword == req.OldPassword {
        return bizErrors.New(response.PasswordError, "新密码不能与原密码相同")
    }

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("密码加密失败", "error", err, "userID", targetUserID)
		return bizErrors.New(response.InternalError, "密码修改失败")
	}

	updates := map[string]interface{}{"password": string(hashedPassword)}
	if err := s.userRepo.Update(ctx, targetUserID, updates); err != nil {
		logger.Error("更新密码失败", "error", err, "userID", targetUserID)
		return bizErrors.New(response.InternalError, "密码修改失败")
	}
	return nil
}

// Delete 用户自注销
func (s *userService) Delete(ctx context.Context, userInfo *jwt.UserInfo, targetUserID uint) error {
	if _, err := s.getAndCheckOwnership(ctx, userInfo, targetUserID); err != nil {
		return err
	}
	if err := s.userRepo.Delete(ctx, targetUserID); err != nil {
		logger.Error("用户自注销失败", "error", err, "userID", targetUserID)
		return bizErrors.New(response.InternalError, "注销失败,请联系管理员")
	}
	return nil
}

// Get 获取用户详细信息
func (s *userService) Get(ctx context.Context, userInfo *jwt.UserInfo, targetUserID uint) (*dto.UserResponse, error) {
	user, err := s.getAndCheckOwnership(ctx, userInfo, targetUserID)
	if err != nil {
		return nil, err
	}
	return convertUserToDTO(user), nil
}

// List 【管理员】获取用户详情列表
func (s *userService) List(ctx context.Context, userInfo *jwt.UserInfo, req *dto.ListUsersRequest) (*common.PaginatedData[*dto.UserResponse], error) {
	// 1. 分页参数的默认值和最大值保护
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	
	// 2. 调用 Repository 层获取数据
	userDOs, total, err := s.userRepo.List(ctx, req)
	if err != nil {
		logger.Error("获取用户列表失败", "error", err)
		return nil, bizErrors.New(response.InternalError, "获取用户列表失败")
	}
	
	// 3. 将 DO 列表转换为 DTO 列表
	userDTOs := make([]*dto.UserResponse, 0, len(userDOs))
	for _, userDO := range userDOs {
		userDTOs = append(userDTOs, convertUserToDTO(userDO))
	}

	// 4. 使用通用的分页结构返回响应
	return &common.PaginatedData[*dto.UserResponse]{
		Data: userDTOs,
		Pagination: common.PaginationResponse{
			Total: total,
			Page:  req.Page,
			Limit: req.Limit,
		},
	}, nil
}

// ForgotPassword 忘记密码，请求发送验证码
func (s *userService) ForgotPassword(ctx context.Context, req *dto.ForgotPasswordRequest) error {
	// 直接复用 CaptchaService 的能力
	// SendCaptcha 内部已经包含了“账号必须存在”的业务校验
	captchaReq := &dto.SendCaptchaRequest{
		Scene:   "reset_password",
		Account: req.Account,
		Type:    req.Type,
	}
	_, err := s.captchaSvc.SendCaptcha(ctx, captchaReq)
	return err
}

// VerifyPasswordResetCaptcha 验证用于重置密码的验证码，并返回重置令牌
func (s *userService) VerifyPasswordResetCaptcha(ctx context.Context, req *dto.VerifyOnceCaptchaRequest) (*dto.VerifyCaptchaResponse, error) {
	// 1. 验证验证码
	if err := s.captchaSvc.VerifyCaptcha(ctx, "reset_password", req.Account, req.Captcha); err != nil {
		return nil, err
	}

	accountType := req.Type
	if accountType == "" { accountType = "email" } // 默认
	
	// 【核心修正点】使用 FindOne 进行查询，unscoped=false 因为只有活跃用户才能重置密码
	conditions := &model.User{}
	if accountType == "email" {
		conditions.Email = req.Account
	} else {
		conditions.Phone = req.Account
	}
	user, err := s.userRepo.FindOne(ctx, conditions, false)
	if err != nil {
		logger.Error("数据不一致：验证码校验成功，但找不到对应的正常状态用户", "error", err, "account", req.Account)
		return nil, bizErrors.New(response.UserNotFound, "用户不存在或状态异常")
	}

	// 3. 生成并存储一次性的密码重置令牌
	resetToken := uuid.NewString()
	key := fmt.Sprintf("token:password_reset:%s", resetToken)
	// 重置令牌有效期10分钟
	ttl := time.Duration(config.GlobalConfig.Security.OneTimeTokenTTL.PasswordReset) * time.Minute
	if err := redis.Set(ctx, key, user.ID, ttl); err != nil {
		logger.Error("存储密码重置令牌失败", "error", err)
		return nil, bizErrors.New(response.InternalError, "操作失败，请稍后再试")
	}

	// 4. 返回重置令牌给前端
	return &dto.VerifyCaptchaResponse{ResetToken: resetToken}, nil
}

// ResetPassword 重置密码
func (s *userService) ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) error {
	// 1. 【验证一次性重置令牌】使用原子性的Lua脚本实现的方法 GetAndDel 操作
	resetTokenKey := fmt.Sprintf("token:password_reset:%s", req.ResetToken)
	userIDStr, err := redis.GetAndDel(ctx, resetTokenKey)
	if err != nil {
		if errors.Is(err, goRedis.Nil) {
			return bizErrors.New(response.InvalidToken, "重置令牌无效或已过期")
		}
		// GetAndDel 内部已记录日志
		return bizErrors.New(response.InternalError, "操作失败，请稍后再试")
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		logger.Error("从Redis解析userID失败(reset_pwd)", "error", err, "value", userIDStr)
		return bizErrors.New(response.InternalError, "操作失败，请稍后再试")
	}

	// 2. 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("重置密码时加密失败", "error", err, "userID", userID)
		return bizErrors.New(response.InternalError, "操作失败")
	}

	// 3. 更新数据库中的密码
	updates := map[string]interface{}{"password": string(hashedPassword)}
	if err := s.userRepo.UpdateUnscoped(ctx, uint(userID), updates); err != nil {
		logger.Error("重置密码时更新数据库失败", "error", err, "userID", userID)
		return bizErrors.New(response.InternalError, "密码重置失败")
	}

	// 4. 【安全增强】强制下线该用户的所有会话
	refreshTokenKey := fmt.Sprintf("%s%d", jwt.TokenActivePrefix, userID)
	if err := redis.Del(ctx, refreshTokenKey); err != nil {
		logger.Error("重置密码后强制下线用户失败", "error", err, "userID", userID)
	}

	return nil
}

// UpdateUserStatus 管理员更新用户状态 | 管理员恢复账号
func (s *userService) UpdateUserStatus(ctx context.Context, targetUserID uint, req *dto.UpdateUserStatusRequest) error {
	// 1. 检查目标用户是否存在
	// 需要使用 Unscoped() 来确保能查找到已被软删除（注销）的用户
	targetUser, err := s.userRepo.FindOne(ctx, &model.User{Model: gorm.Model{ID: targetUserID}}, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return bizErrors.New(response.UserNotFound, "目标用户不存在")
		}
		logger.Error("查询目标用户失败", "error", err, "targetUserID", targetUserID)
		return bizErrors.New(response.InternalError, "获取用户信息失败")
	}

	// 2. 业务规则：禁止管理员操作其他管理员账号
	if targetUser.Role == model.UserRoleAdmin {
		return bizErrors.New(response.Forbidden, "不允许修改其他管理员的状态")
	}

	// 3. 动态构建需要更新的字段
	updates := map[string]interface{}{"status": *req.Status}

	// 4. 【核心逻辑】如果新状态是“正常”，则同时执行“恢复”操作
	if *req.Status == model.UserStatusNormal {
		// gorm更新nil会设置为NULL，从而取消软删除
		updates["deleted_at"] = nil

		// 解锁Redis中的登录锁定标记
		lockoutKey := fmt.Sprintf("lockout:user:%s", targetUser.Username)
		if err := redis.Del(ctx, lockoutKey); err != nil {
			logger.Error("管理员解锁用户时删除Redis锁失败", "error", err, "userID", targetUserID)
			// 此为辅助操作，不阻塞主流程
		}
		loginAttemptsKey := fmt.Sprintf("ratelimit:login_attempts:%s", targetUser.Username)
		if err := redis.Del(ctx, loginAttemptsKey); err != nil {
			logger.Error("管理员解锁用户时删除Redis计数器失败", "error", err, "userID", targetUserID)
		}
	}
	
	// 5. 执行更新
	// 更新包括软删除在内的用户
	if err := s.userRepo.UpdateUnscoped(ctx, targetUserID, updates); err != nil {
		logger.Error("管理员更新用户状态失败", "error", err, "targetUserID", targetUserID)
		return bizErrors.New(response.InternalError, "更新用户状态失败")
	}

	// 6. 安全增强：如果用户被禁用或锁定，立即强制其下线
	if *req.Status == model.UserStatusBanned || *req.Status == model.UserStatusLocked {
		refreshTokenKey := fmt.Sprintf("%s%d", jwt.TokenActivePrefix, targetUserID)
		if err := redis.Del(ctx, refreshTokenKey); err != nil {
			logger.Error("强制下线用户（删除RefreshToken）失败", "error", err, "userID", targetUserID)
		}
	}

	return nil
}

// RequestRecovery 请求恢复账号验证码
func (s *userService) RequestRecovery(ctx context.Context, req *dto.RequestRecoveryRequest) error {
	// 复用 CaptchaService，并指定新的场景
	captchaReq := &dto.SendCaptchaRequest{
		Scene:   "recover_account",
		Account: req.Account,
		Type:    req.Type,
	}
	_, err := s.captchaSvc.SendCaptcha(ctx, captchaReq)
	return err
}

// VerifyRecoveryCaptcha 验证恢复验证码，并返回恢复令牌
func (s *userService) VerifyRecoveryCaptcha(ctx context.Context, req *dto.VerifyRecoveryCaptchaRequest) (*dto.VerifyRecoveryCaptchaResponse, error) {
	// 1. 验证验证码
	if err := s.captchaSvc.VerifyCaptcha(ctx, "recover_account", req.Account, req.Captcha); err != nil {
		return nil, err
	}

	accountType := req.Type
	if accountType == "" { accountType = "email" }

	// 【核心修正点】使用 FindOne 进行查询，unscoped=true 因为要查找已注销的用户
	conditions := &model.User{}
	if accountType == "email" {
		conditions.Email = req.Account
	} else {
		conditions.Phone = req.Account
	}
	user, err := s.userRepo.FindOne(ctx, conditions, true)
	if err != nil {
		logger.Error("数据不一致：验证码校验成功，但找不到对应的正常状态用户", "error", err, "account", req.Account)
		return nil, bizErrors.New(response.UserNotFound, "用户不存在")
	}

	// 3. 生成并存储一次性的账号恢复令牌
	recoveryToken := uuid.NewString()
	key := fmt.Sprintf("token:account_recovery:%s", recoveryToken)
	// 恢复令牌有效期10分钟
	ttl := time.Duration(config.GlobalConfig.Security.OneTimeTokenTTL.AccountRecovery) * time.Minute 
	if err := redis.Set(ctx, key, user.ID, ttl); err != nil {
		logger.Error("存储账号恢复令牌失败", "error", err)
		return nil, bizErrors.New(response.InternalError, "操作失败，请稍后再试")
	}

	// 4. 返回重置令牌给前端
	return &dto.VerifyRecoveryCaptchaResponse{RecoveryToken: recoveryToken}, nil
}

// ExecuteRecovery 执行账号恢复
func (s *userService) ExecuteRecovery(ctx context.Context, req *dto.ExecuteRecoveryRequest) error {
	// 1. 验证并销毁一次性恢复令牌
	recoveryTokenKey := fmt.Sprintf("token:account_recovery:%s", req.RecoveryToken)
	userIDStr, err := redis.Get(ctx, recoveryTokenKey)
	if err != nil {
		if errors.Is(err, goRedis.Nil) {
			return bizErrors.New(response.InvalidToken, "恢复令牌无效或已过期")
		}
		return bizErrors.New(response.InternalError, "操作失败，请稍后再试")
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		logger.Error("从Redis解析userID失败(recovery)", "error", err, "value", userIDStr)
		return bizErrors.New(response.InternalError, "操作失败，请稍后再试")
	}

	// 2. 更新用户状态，完成恢复
	updates := map[string]interface{}{
		"status":     model.UserStatusNormal,
		"deleted_at": nil, // gorm更新nil会设置为NULL
	}
	// 需要使用 Unscoped 的方法更新，否则会找不到这行记录
	if err := s.userRepo.UpdateUnscoped(ctx, uint(userID), updates); err != nil {
		logger.Error("执行账号恢复时更新数据库失败", "error", err, "userID", userID)
		return bizErrors.New(response.InternalError, "账号恢复失败")
	}

	// 3. DB操作成功后，再从redis里面删除令牌
	if err = redis.Del(ctx, recoveryTokenKey); err != nil {
		logger.Error("删除已使用的恢复账号令牌失败", "error", err, "key", recoveryTokenKey)
	}

	return nil
}



// ============ 【辅助函数】 ============
// DO to DTO 转换器
func convertUserToDTO(user *model.User) *dto.UserResponse {
	return &dto.UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		Nickname:    user.Nickname,
		Avatar:      user.Avatar,
		Email:       user.Email,
		Phone:       user.Phone,
		Status:      user.Status,
		Role:        user.Role,
		LastLoginAt: user.LastLoginAt,
		CreatedAt:   user.CreatedAt,
	}
}
