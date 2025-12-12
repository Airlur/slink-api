package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"short-link/internal/dto"
	"short-link/internal/dto/common"
	"short-link/internal/model"
	"short-link/internal/pkg/config"
	"short-link/internal/pkg/constant"
	bizErrors "short-link/internal/pkg/errors"
	"short-link/internal/pkg/eventbus"
	"short-link/internal/pkg/generator"
	"short-link/internal/pkg/jwt"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/redis"
	"short-link/internal/pkg/response"
	"short-link/internal/repository"

	goRedis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// init 在包初始化时执行，用于准备保留字map
func init() {
	reservedWordsMap = make(map[string]struct{})
	for _, word := range config.GlobalConfig.Shortlink.ReservedWords {
		reservedWordsMap[strings.ToLower(word)] = struct{}{}
	}
}

type ShortlinkService interface {
	// ===== 基础 CRUD 接口方法 =====
	// Create(ctx context.Context, req *dto.CreateShortlinkRequest) error
	CreateForGuest(ctx context.Context, req *dto.GuestCreateShortlinkRequest) (*dto.ShortlinkResponse, error)
	CreateForUser(ctx context.Context, user *jwt.UserInfo, req *dto.UserCreateShortlinkRequest) (*dto.ShortlinkResponse, error)
	// Redirect 重定向查找给定短代码的原始URL并处理分析
	// Redirect(ctx context.Context, shortCode string) (originalUrl string, err error)
	Redirect(ctx context.Context, shortCode, ip, ua, referer string, user *jwt.UserInfo) (originalUrl, cacheStatus string, err error)
	Update(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.UpdateShortlinkRequest) error
	Delete(ctx context.Context, user *jwt.UserInfo, shortCode string) error
	ListMyShortlinks(ctx context.Context, user *jwt.UserInfo, req *dto.ListMyShortlinksRequest) (*common.PaginatedData[*dto.ShortlinkDetailResponse], error)
	GetDetailByShortCode(ctx context.Context, user *jwt.UserInfo, shortCode string) (*dto.ShortlinkDetailResponse, error)
	UpdateStatus(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.UpdateShortlinkStatusRequest) error
	ExtendExpiration(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.ExtendShortlinkExpirationRequest) error

	// ===== 根据索引查询接口方法 =====
	GetUniqueShortCode(ctx context.Context, shortCode string) (*dto.ShortlinkResponse, error)
	GetUniqueUserIdAndOriginalUrlMd5(ctx context.Context, userId int64, originalUrlMd5 string) (*dto.ShortlinkResponse, error)
}

type shortlinkService struct {
	db            *gorm.DB // 注入 gorm.DB 以便开启事务
	shortlinkRepo repository.ShortlinkRepository
}

func NewShortlinkService(db *gorm.DB, shortlinkRepo repository.ShortlinkRepository) ShortlinkService {
	return &shortlinkService{
		db:            db,
		shortlinkRepo: shortlinkRepo,
	}
}

var (
	// 用于检查短码是否只包含 Base62 字符 (0-9, a-z, A-Z)
	base62Regex = regexp.MustCompile("^[0-9a-zA-Z]+$")
	// 预先将保留字列表转为map，以提高查询效率
	reservedWordsMap map[string]struct{}
)

// === 用于获取并校验所有权 ===
func (s *shortlinkService) getAndCheckOwnership(ctx context.Context, user *jwt.UserInfo, shortCode string, preloadAssociations bool) (*model.Shortlink, error) {
	// 1. 查找短链接
	m, err := s.shortlinkRepo.GetUniqueShortCode(ctx, shortCode, preloadAssociations)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, bizErrors.New(response.ShortlinkNotFound, "短链接不存在")
		}
		// 这是未知的数据库错误，需要记录日志
		logger.Error("数据库查询失败", "error", err, "shortCode", shortCode)
		return nil, bizErrors.New(response.InternalError, "数据库查询失败")
	}

	// 2. 权限校验：确保操作者是该短链接的所有者
	if m.UserId != int64(user.ID) {
		logger.Info("无权操作短链接", "createUser", m.UserId, "currentUserID", user.ID, "currentUserName", user.Username)
		return nil, bizErrors.New(response.Forbidden, "无权操作此链接")
	}

	return m, nil
}

// ===== 基础 CRUD 接口方法实现 =====
// CreateForGuest 为游客创建短链接
// 不同游客对同一个URL创建短链接是不同的
func (s *shortlinkService) CreateForGuest(ctx context.Context, req *dto.GuestCreateShortlinkRequest) (*dto.ShortlinkResponse, error) {
	// URL 安全与格式化校验
	formattedURL, err := validateAndFormatURL(req.OriginalUrl)
	if err != nil {
		return nil, err
	}
	urlMd5 := MD5(formattedURL)

	// 2. 先查询该长链接是否已存在
	// 通过 original_url_md5 唯一索引查询
	existing, err := s.shortlinkRepo.GetByOriginalURLMd5(ctx, urlMd5)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, bizErrors.New(response.InternalError, "系统繁忙，请稍后重试")
	}
	// 如果已存在，直接返回已有的短链接信息。
	// TODO: 但是创建有问题，相同的链接还是没隔离，这个时间有效性过期了，后面同样的链接再去创建还是这个已有的信息，并且状态没有改变，没办法等过期了去实时更新状态，这是个问题，但是它访问的话会是过期的，这个没问题
	if existing != nil {
		return convertShortlinkToGuestDTO(existing), nil
	}

	// 游客短链接默认有效期为7天
	guestExpiresIn := "7d"
	expireAt, _ := parseExpiresIn(&guestExpiresIn)

	shortlink := &model.Shortlink{
		OriginalUrl:    formattedURL,
		OriginalUrlMd5: urlMd5,
		UserId:         0, // 0 代表游客
		ExpireAt:       expireAt,
		Status:         1, // 1 代表有效，0 代表失效
		IsCustom:       0, // 0 代表非自定义，1 代表自定义
	}

	// 使用事务保证数据一致性
	err = s.db.Transaction(func(tx *gorm.DB) error {
		txRepo := repository.NewShortlinkRepository(tx)

		// 1. 插入记录以获取ID
		if err := txRepo.Create(ctx, shortlink); err != nil {
			return bizErrors.New(response.InternalError, "数据库错误，创建失败")
		}

		// 2. 使用ID生成短码
		shortCode := generator.Generate(uint64(shortlink.ID))

		// 3. 更新记录，回填short_code
		updates := map[string]interface{}{"short_code": shortCode}
		if err := txRepo.Update(ctx, shortlink.ID, updates); err != nil {
			return bizErrors.New(response.InternalError, "数据库错误，更新短码失败")
		}

		// 将生成的shortCode填充回模型，以便返回
		shortlink.ShortCode = shortCode
		return nil
	})

	if err != nil {
		return nil, err
	}

	return convertShortlinkToGuestDTO(shortlink), nil
}

// CreateForUser 为已登录用户创建短链接
// 不同用户对同一个URL创建短链接是不同的
func (s *shortlinkService) CreateForUser(ctx context.Context, user *jwt.UserInfo, req *dto.UserCreateShortlinkRequest) (*dto.ShortlinkResponse, error) {
	// 1. URL 安全与格式化校验
	formattedURL, err := validateAndFormatURL(req.OriginalUrl)
	if err != nil {
		return nil, err
	}

	urlMd5 := MD5(formattedURL)

	// 2. 检查该用户是否已为该URL创建过短链接
	existing, err := s.shortlinkRepo.GetUniqueUserIdAndOriginalUrlMd5(ctx, int64(user.ID), urlMd5)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, bizErrors.New(response.InternalError, "系统繁忙，请稍后重试")
	}
	if existing != nil {
		// 如果已存在，直接返回已有的短链接信息
		return convertShortlinkToUserDTO(existing), nil
	}

	// 3. 处理有效期
	expireAt, err := parseExpiresIn(req.ExpiresIn)
	if err != nil {
		return nil, err
	}

	shortlink := &model.Shortlink{
		OriginalUrl:    formattedURL,
		OriginalUrlMd5: urlMd5,
		UserId:         int64(user.ID),
		Status:         1,
		ExpireAt:       expireAt,
	}

	// 4. 处理用户自定义短码
	isCustom := 0
	if req.ShortCode != nil && *req.ShortCode != "" {
		limitKey := fmt.Sprintf("ratelimit:custom:user:%d", user.ID)
		limit := config.GlobalConfig.RateLimit.CustomPerDayUser
		// 自定义短码限流
		count, err := redis.IncrWithExpiration(ctx, limitKey, 24*time.Hour)
		if err != nil {
			return nil, bizErrors.New(response.InternalError, "服务器内部错误")
		}
		if int(count) > limit {
			return nil, bizErrors.New(response.TooManyCustomAttempts, fmt.Sprintf("今日自定义尝试已达上限 (%d) 次，升级会员可享受更高额度。", limit))
		}

		customCode := *req.ShortCode

		// 4.1 校验字符集
		if !base62Regex.MatchString(customCode) {
			return nil, bizErrors.New(response.InvalidShortcodeChars, "自定义短码只能包含字母和数字")
		}

		// 4.2 校验保留字
		if _, ok := reservedWordsMap[strings.ToLower(customCode)]; ok {
			return nil, bizErrors.New(response.ShortcodeIsReserved, "自定义短码是系统保留词")
		}
		// 4.3 校验连续重复字符
		if hasRepetitiveChars(customCode, 6) {
			return nil, bizErrors.New(response.ShortcodeHasRepetitiveChars, "自定义短码包含过多连续重复字符")
		}
		isCustom = 1
	}
	shortlink.IsCustom = isCustom

	// 使用事务保证数据一致性
	err = s.db.Transaction(func(tx *gorm.DB) error {
		txRepo := repository.NewShortlinkRepository(tx)

		// 如果是自定义短码，需要先检查是否已被占用
		if isCustom == 1 {
			_, err := txRepo.GetUniqueShortCode(ctx, *req.ShortCode, false)
			if err == nil { // 如果 err 为 nil，说明找到了记录，短码已被占用
				return bizErrors.New(response.InvalidParam, "自定义短码已被占用")
			}
			if err != gorm.ErrRecordNotFound { // 其他数据库错误
				logger.Error("CreateForUser Fail: 自定义短码查询失败",
					"err", err,
					"shortCode", *req.ShortCode,
				)
				return bizErrors.New(response.InternalError, "系统繁忙，请稍后重试")
			}
		}

		// 1. 插入记录
		if err := txRepo.Create(ctx, shortlink); err != nil {
			// 高并发下两个线程都能通过「先查再插」校验，最终第二个 Insert 会因为 UNIQUE 索引冲突报 1062。
			if strings.Contains(err.Error(), "Duplicate entry") && isCustom == 1 {
				return bizErrors.New(response.InvalidParam, "自定义短码已被占用")
			}
			logger.Error("CreateForUser Fail: 创建短链接失败",
				"err", err,
				"shortlink", shortlink,
			)
			return bizErrors.New(response.InternalError, "数据库错误，创建失败")
		}

		// 2. 确定最终的 short_code
		var finalShortCode string
		if isCustom == 1 {
			finalShortCode = *req.ShortCode
		} else {
			finalShortCode = generator.Generate(uint64(shortlink.ID))
		}

		// 3. 更新记录，回填 short_code
		updates := map[string]interface{}{"short_code": finalShortCode}
		if err := txRepo.Update(ctx, shortlink.ID, updates); err != nil {
			return bizErrors.New(response.InternalError, "数据库错误，更新短码失败")
		}

		shortlink.ShortCode = finalShortCode
		return nil
	})

	if err != nil {
		return nil, err
	}

	return convertShortlinkToUserDTO(shortlink), nil
}

// Redirect 重定向短链接访问
func (s *shortlinkService) Redirect(ctx context.Context, shortCode, ip, ua, referer string, user *jwt.UserInfo) (string, string, error) {
	cacheKey := "cache:short_code:" + shortCode
	var cacheStatus string // 新增：记录缓存状态（HIT/MISS/NULL）

	// 1. 查缓存
	originalUrl, err := redis.Get(ctx, cacheKey)
	if err != nil && err != goRedis.Nil {
		// 如果是Redis本身出错，记录日志并继续走数据库（服务降级）
		logger.Error("查询Redis缓存失败", "error", err, "key", cacheKey)
	}

	// 提取UserID，无论是否登录，都确保有一个值
	var userID uint = 0
	if user != nil {
		userID = user.ID
	}

	// 2. 缓存命中 (Cache Hit)
	if err == nil {
		// 命中"空值缓存"，说明这是一个已知的不存在的短码，直接返回错误
		cacheStatus = "HIT"
		if originalUrl == constant.NullCacheValue {
			cacheStatus = "NULL" // 空值缓存
			return "", cacheStatus, bizErrors.New(response.ShortlinkNotFound, "短链接不存在")
		}
		// // 正常命中，异步更新统计信息并返回
		// go s.recordAnalyticsByShortCode(shortCode) // 使用 shortCode 异步更新
		// 发布事件，异步更新统计信息
		eventbus.PublishAccessLog(eventbus.AccessLogEvent{
			ShortCode: shortCode,
			IP:        ip,
			UserAgent: ua,
			Referer:   referer,
			UserID:    userID,
			Timestamp: time.Now(),
		})

		return originalUrl, cacheStatus, nil
	}

	// 3. 缓存未命中 (err == goRedis.Nil)，进入防击穿逻辑
	cacheStatus = "MISS" // 标记为缓存未命中
	lockKey := "lock:short_code:" + shortCode
	lockTTL := 10 * time.Second // 锁的过期时间，防止死锁

	// 3.1 尝试获取分布式锁
	locked, err := redis.SetNX(ctx, lockKey, "1", lockTTL)
	if err != nil {
		logger.Error("尝试获取分布式锁失败", "error", err, "key", lockKey)
	}

	if locked {
		// 3.2 获取锁成功：由我来查询数据库并回写缓存
		defer func() {
			// Lua脚本：安全释放锁（仅当锁的值匹配时才删除）
			// 防止因业务逻辑执行时间过长导致锁过期后，误删了其他线程持有的新锁
			const releaseLockScript = `
				if redis.call("get", KEYS[1]) == ARGV[1] then
					return redis.call("del", KEYS[1])
				else
					return 0
				end
			`
			redis.Client.Eval(ctx, releaseLockScript, []string{lockKey}, "1")
		}()

		// 3.2.1 缓存未命中 (Cache Miss)，查询数据库
		shortlink, dbErr := s.shortlinkRepo.GetUniqueShortCode(ctx, shortCode, false)
		if dbErr != nil {
			// 数据库也查不到，是缓存穿透的典型场景
			if dbErr == gorm.ErrRecordNotFound {
				// 写入"空值缓存"，并设置较短的过期时间
				nullCacheTTL := time.Duration(config.GlobalConfig.Cache.NullCacheExpirationSeconds) * time.Second
				redis.Set(ctx, cacheKey, constant.NullCacheValue, nullCacheTTL)
				cacheStatus = "NULL" // 空值缓存
				return "", cacheStatus, bizErrors.New(response.ShortlinkNotFound, "短链接不存在")
			}
			// 其他数据库错误
			return "", cacheStatus, bizErrors.New(response.InternalError, "系统繁忙，请稍后重试")
		}

		// 3.2.2 数据库查到了，校验链接有效性（检查状态和过期时间）
		if shortlink.Status != 1 || (shortlink.ExpireAt != nil && !shortlink.ExpireAt.IsZero() && time.Now().After(*shortlink.ExpireAt)) {
			// 对于已失效或过期的链接，同样写入空值缓存，避免反复查询
			nullCacheTTL := time.Duration(config.GlobalConfig.Cache.NullCacheExpirationSeconds) * time.Second
			redis.Set(ctx, cacheKey, constant.NullCacheValue, nullCacheTTL)
			cacheStatus = "NULL" // 空值缓存
			return "", cacheStatus, bizErrors.New(response.ShortlinkNotFound, "该链接已失效或过期")
		}

		// // 回写缓存时，先查现有缓存的剩余时间
		// existingTTL, err := redis.TTL(ctx, cacheKey)
		// if err == nil && existingTTL > 0 {
		// 	// 若缓存已存在且剩余时间<30分钟（即将过期），延长TTL（热点短码会频繁触发此逻辑）
		// 	if existingTTL < 30*time.Minute {
		// 		if existingTTL > coldTTL {
		// 			redis.Expire(ctx, cacheKey, hotTTL) // 已接近热点TTL，直接设为热点TTL
		// 		} else {
		// 			redis.Expire(ctx, cacheKey, existingTTL+30*time.Minute) // 冷短码延长30分钟
		// 		}
		// 	}
		// 	return shortlink.OriginalUrl, nil // 无需重新设置缓存，直接返回
		// }
		// // 缓存不存在，按原逻辑设置TTL

		// 3.2.3 回写缓存
		// 增加随机抖动，防止缓存雪崩
		baseTTL := time.Duration(config.GlobalConfig.Cache.DefaultExpirationMinutes) * time.Minute
		jitter := time.Duration(rand.Intn(config.GlobalConfig.Cache.RandomJitterSeconds)) * time.Second
		redis.Set(ctx, cacheKey, shortlink.OriginalUrl, baseTTL+jitter)

		// // 3.2.4 异步更新统计信息并返回原始URL
		// go s.recordAnalytics(shortlink.ID)

		// 数据库查询成功后，发布事件，异步更新统计信息
		eventbus.PublishAccessLog(eventbus.AccessLogEvent{
			ShortCode: shortCode,
			IP:        ip,
			UserAgent: ua,
			Referer:   referer,
			UserID:    userID,
			Timestamp: time.Now(),
		})
		return shortlink.OriginalUrl, cacheStatus, nil
	} else {
		// 3.3 获取锁失败：说明有其他线程正在查询数据库，我稍等片刻后重试缓存
		maxRetries := 5
		retryCount := 0
		for retryCount < maxRetries {
			time.Sleep(50 * time.Millisecond) // 等待50毫秒
			originalUrl, err = redis.Get(ctx, cacheKey)
			if err == nil {
				// 命中缓存，处理后返回
				if originalUrl == constant.NullCacheValue {
					cacheStatus = "NULL" // 空值缓存
					return "", cacheStatus, bizErrors.New(response.ShortlinkNotFound, "短链接不存在")
				}
				// go s.recordAnalyticsByShortCode(shortCode) // 使用 shortCode 异步更新

				// 重试成功命中缓存后，发布事件，异步更新统计信息
				eventbus.PublishAccessLog(eventbus.AccessLogEvent{
					ShortCode: shortCode,
					IP:        ip,
					UserAgent: ua,
					Referer:   referer,
					UserID:    userID,
					Timestamp: time.Now(),
				})
				return originalUrl, cacheStatus, nil
			}
			retryCount++
		}
		// 超过重试次数，返回友好提示
		return "", cacheStatus, bizErrors.New(response.InternalError, "服务繁忙，请稍后再试")
	}
}

// Update 更新短链接信息
func (s *shortlinkService) Update(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.UpdateShortlinkRequest) error {
	// 1. 获取并校验所有权
	m, err := s.getAndCheckOwnership(ctx, user, shortCode, false)
	if err != nil {
		return err
	}

	// 2. 动态构建更新数据
	updates := map[string]interface{}{}

	if req.OriginalUrl != nil {
		updates["original_url"] = *req.OriginalUrl
		updates["original_url_md5"] = MD5(*req.OriginalUrl)
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.ExpiresIn != nil {
		newExpireAt, err := parseDurationString(*req.ExpiresIn)
		if err != nil {
			return err
		}
		updates["expire_at"] = newExpireAt
	}
	// 如果未来DTO增加了其他字段（如分享标题），在这里继续添加判断
	// if req.ShareTitle != nil { ... }

	// 3. 如果没有任何字段需要更新，则直接返回成功
	if len(updates) == 0 {
		return nil
	}

	// 4. 执行更新操作
	if err := s.shortlinkRepo.Update(ctx, m.ID, updates); err != nil {
		logger.Error("db update shortlink fail",
			"err", err,
			"shortCode", shortCode,
			"updates", updates,
		)
		return bizErrors.New(response.InternalError, "更新失败，请稍后重试")
	}

	// 5. 删除缓存
	cacheKey := "cache:short_code:" + m.ShortCode
	if err := redis.Del(ctx, cacheKey); err != nil {
		// 缓存删除失败通常不应该阻塞主流程，但是要记录严重的错误日志
		logger.Error("删除Redis缓存失败", "error", err, "Key", cacheKey)
	}

	return nil
}

// Delete 软删除短链接
func (s *shortlinkService) Delete(ctx context.Context, user *jwt.UserInfo, shortCode string) error {
	// 1. 先获取记录，以便拿到 shortCode 用于失效缓存
	m, err := s.getAndCheckOwnership(ctx, user, shortCode, false)
	if err != nil {
		return err
	}

	// 2. 删除数据库记录
	if err := s.shortlinkRepo.Delete(ctx, m.ID); err != nil {
		// 这是未知的数据库错误，需要记录日志
		logger.Error("数据库删除短链接失败", "error", err, "id", m.ID)
		return bizErrors.New(response.InternalError, "删除失败，请稍后重试")
	}

	// 3. 删除缓存
	cacheKey := "cache:short_code:" + m.ShortCode
	if err := redis.Del(ctx, cacheKey); err != nil {
		logger.Error("删除Redis缓存失败", "error", err, "key", cacheKey)
	}

	return nil
}

// ListMyShortlinks 获取当前用户的短链接列表
func (s *shortlinkService) ListMyShortlinks(ctx context.Context, user *jwt.UserInfo, req *dto.ListMyShortlinksRequest) (*common.PaginatedData[*dto.ShortlinkDetailResponse], error) {
	// 1. 设置分页默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 { // 设置一个最大值，防止恶意请求
		req.Limit = 100
	}

	// 2. 调用Repository层获取数据
	links, total, err := s.shortlinkRepo.ListByUserID(ctx, user.ID, req)
	if err != nil {
		return nil, bizErrors.New(response.InternalError, "获取短链接列表失败，请稍后重试")
	}

	// 3. 将 model 列表转换为 DTO 列表
	var dtoList []*dto.ShortlinkDetailResponse
	for _, link := range links {
		dtoList = append(dtoList, convertShortlinkToDetailDTO(link))
	}

	// 4. 【核心修正点】使用通用的、泛型的 PaginatedData 结构来构建最终响应
	return &common.PaginatedData[*dto.ShortlinkDetailResponse]{
		Data: dtoList,
		Pagination: common.PaginationResponse{ // 使用 common 包中的分页响应
			Total: total,
			Page:  req.Page,
			Limit: req.Limit,
		},
	}, nil
}

// GetDetailByShortCode 获取单个短链接的详情
func (s *shortlinkService) GetDetailByShortCode(ctx context.Context, user *jwt.UserInfo, shortCode string) (*dto.ShortlinkDetailResponse, error) {
	// 复用鉴权辅助函数
	link, err := s.getAndCheckOwnership(ctx, user, shortCode, true)
	if err != nil {
		return nil, err
	}
	return convertShortlinkToDetailDTO(link), nil
}

// UpdateStatus 更新短链接状态的业务逻辑
func (s *shortlinkService) UpdateStatus(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.UpdateShortlinkStatusRequest) error {
	m, err := s.getAndCheckOwnership(ctx, user, shortCode, false)
	if err != nil {
		return err
	}
	// 更新数据库
	updates := map[string]interface{}{"status": *req.Status}
	if err := s.shortlinkRepo.Update(ctx, m.ID, updates); err != nil {
		logger.Error("UpdateStatus Fail: 短链接更新状态失败",
			"err", err,
			"shortCode", shortCode,
			"updates", updates,
		)
		return bizErrors.New(response.InternalError, "更新失败，请稍后重试")
	}

	//【核心】缓存失效
	cacheKey := "cache:short_code:" + shortCode
	if err := redis.Del(ctx, cacheKey); err != nil {
		logger.Error("删除Redis缓存失败", "error", err, "key", cacheKey)
	}

	return nil
}

// ExtendExpiration 延长有效期的业务逻辑
func (s *shortlinkService) ExtendExpiration(ctx context.Context, user *jwt.UserInfo, shortCode string, req *dto.ExtendShortlinkExpirationRequest) error {
	m, err := s.getAndCheckOwnership(ctx, user, shortCode, false)
	if err != nil {
		return err
	}

	// 计算新的过期时间
	newExpireAt, err := parseDurationString(req.ExpiresIn)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{"expire_at": newExpireAt}

	// if req.ExpiresIn == "never" {
	// 	// 当值为 nil 时，GORM会将其更新为数据库的 NULL
	// 	updates["expire_at"] = nil
	// } else {
	// 	// 对于其他情况，才调用解析函数
	// 	newExpireAt, err := parseDurationString(req.ExpiresIn)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	updates["expire_at"] = newExpireAt
	// }

	if err := s.shortlinkRepo.Update(ctx, m.ID, updates); err != nil {
		logger.Error("ExtendExpiration Fail: 短链接延长有效期失败",
			"err", err,
			"shortCode", shortCode,
			"updates", updates,
		)
		return bizErrors.New(response.InternalError, "更新失败，请稍后重试")
	}

	cacheKey := "cache:short_code:" + shortCode
	if err := redis.Del(ctx, cacheKey); err != nil {
		logger.Error("删除Redis缓存失败", "error", err, "key", cacheKey)
	}

	return nil
}

// ===== 根据索引查询接口方法实现 =====

func (s *shortlinkService) GetUniqueShortCode(ctx context.Context, shortCode string) (*dto.ShortlinkResponse, error) {
	// 1. 调用 Repository 层获取 DO
	m, err := s.shortlinkRepo.GetUniqueShortCode(ctx, shortCode, false)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, bizErrors.New(response.ShortlinkNotFound, "短链接不存在")
		}
		return nil, bizErrors.New(response.InternalError, "系统繁忙，请稍后重试")
	}
	// 2. 将 DO 转换为 DTO 并返回
	return convertShortlinkToUserDTO(m), nil
}

func (s *shortlinkService) GetUniqueUserIdAndOriginalUrlMd5(ctx context.Context, userId int64, originalUrlMd5 string) (*dto.ShortlinkResponse, error) {
	// 1. 调用 Repository 层获取 DO
	m, err := s.shortlinkRepo.GetUniqueUserIdAndOriginalUrlMd5(ctx, userId, originalUrlMd5)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, bizErrors.New(response.ShortlinkNotFound, "短链接不存在")
		}
		return nil, bizErrors.New(response.InternalError, "系统繁忙，请稍后重试")
	}
	// 2. 将 DO 转换为 DTO 并返回
	return convertShortlinkToUserDTO(m), nil
}

// ===== 辅助函数 =====

// MD5 一个简单的 MD5 编码方法
func MD5(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// convertShortlinkToGuestDTO 为游客转换DTO，只包含最基本的信息
func convertShortlinkToGuestDTO(m *model.Shortlink) *dto.ShortlinkResponse {
	// 动态拼接完整的短链接URL
	// 这里的协议 "https://" 也可以根据环境配置化，但通常默认为 https
	shortURL := fmt.Sprintf("%s://%s/%s", config.GlobalConfig.App.Scheme, config.GlobalConfig.App.Domain, m.ShortCode)

	return &dto.ShortlinkResponse{
		ShortUrl:    shortURL,
		ShortCode:   m.ShortCode,
		OriginalUrl: m.OriginalUrl,
		ExpireAt:    m.ExpireAt, // 只返回有效期，其他统计和内部信息全部隐藏
	}
}

// convertShortlinkToUserDTO 为登录用户（所有者）转换DTO，包含详细信息
func convertShortlinkToUserDTO(m *model.Shortlink) *dto.ShortlinkResponse {
	// 动态拼接完整的短链接URL
	// 这里的协议 "https://" 也可以根据环境配置化，但通常默认为https
	shortURL := fmt.Sprintf("%s://%s/%s", config.GlobalConfig.App.Scheme, config.GlobalConfig.App.Domain, m.ShortCode)

	return &dto.ShortlinkResponse{
		ShortUrl:    shortURL,
		ShortCode:   m.ShortCode,
		OriginalUrl: m.OriginalUrl,
		ExpireAt:    m.ExpireAt,
		ClickCount:  m.ClickCount,
		Status:      m.Status,
		IsCustom:    m.IsCustom,
		ID:          m.ID,
		UserId:      m.UserId,
		CreatedAt:   &m.CreatedAt,
		UpdatedAt:   &m.UpdatedAt,
	}
}

// convertShortlinkToDetailDTO 转换DTO，包含详细信息
func convertShortlinkToDetailDTO(m *model.Shortlink) *dto.ShortlinkDetailResponse {
	shortURL := fmt.Sprintf("%s://%s/%s", config.GlobalConfig.App.Scheme, config.GlobalConfig.App.Domain, m.ShortCode)

	resp := &dto.ShortlinkDetailResponse{
		ShortUrl:    shortURL,
		ShortCode:   m.ShortCode,
		OriginalUrl: m.OriginalUrl, // 注意：这里可以根据需求做脱敏处理
		ExpireAt:    m.ExpireAt,
		ClickCount:  m.ClickCount,
		Status:      m.Status,
		CreatedAt:   m.CreatedAt,
	}

	if len(m.Tags) > 0 {
		resp.Tags = make([]string, len(m.Tags))
		for i, tag := range m.Tags {
			resp.Tags[i] = tag.TagName
		}
	}

	if m.Share != nil {
		resp.Share = &dto.ShareInfo{
			Title: m.Share.ShareTitle,
			Desc:  m.Share.ShareDesc,
			Image: m.Share.ShareImage,
		}
	}

	return resp
}

// validateAndFormatURL 对原始URL进行安全校验和格式化
func validateAndFormatURL(originalURL string) (string, error) {
	// 0. 预处理：去除首尾空格
	originalURL = strings.TrimSpace(originalURL)

	// 1. 长度校验
	if len(originalURL) > 2048 {
		return "", bizErrors.New(response.InvalidParam, "原始链接长度不能超过2048个字符")
	}

	// 2. 协议处理：检查是否已有 http/https 前缀（忽略大小写）
	// 必须先补全协议，url.Parse 才能正确解析出 Host
	lowerURL := strings.ToLower(originalURL)
	if !strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://") {
		originalURL = "https://" + originalURL
	}

	// 3. 合法性与防嵌套校验
	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return "", bizErrors.New(response.InvalidParam, "无效的URL格式")
	}

	// 4. Host 校验：必须包含 "." 或者是 "localhost"
	// 这一步必须在 Parse 之后，因为只有解析出 Host 才能准确判断
	if !strings.Contains(parsedURL.Host, ".") && parsedURL.Host != "localhost" {
		return "", bizErrors.New(response.InvalidParam, "无效的URL格式")
	}

	if strings.EqualFold(parsedURL.Host, config.GlobalConfig.App.Domain) {
		return "", bizErrors.New(response.LinkNestingNotAllowed, "当前网址已经是短链接，无需再次缩短")
	}

	// 5. 修复片段编码问题
	// 直接设置 RawFragment 来避免自动编码
	if parsedURL.Fragment != "" {
		parsedURL.RawFragment = parsedURL.Fragment
	}

	return parsedURL.String(), nil
}

// parseExpiresIn 解析用于【创建】操作的有效期字符串 (接收指针)
func parseExpiresIn(expiresIn *string) (*time.Time, error) {
	if expiresIn == nil || *expiresIn == "never" {
		return nil, nil // 返回零值时间，GORM会自动处理为NULL
	}
	return parseDurationString(*expiresIn)
}

// parseDurationString 是【核心】的解析逻辑 (接收字符串)
func parseDurationString(durationStr string) (*time.Time, error) {
	if durationStr == "never" {
		return nil, nil // 【修正点】直接返回 nil
	}

	now := time.Now()
	var expireAt time.Time // 先计算出具体时间

	switch durationStr {
	case "7d":
		expireAt = now.AddDate(0, 0, 7)
	case "30d":
		expireAt = now.AddDate(0, 1, 0)
	case "90d":
		expireAt = now.AddDate(0, 3, 0)
	case "1y":
		expireAt = now.AddDate(1, 0, 0)
	}

	// 兼容创建时更灵活的格式，如 "1h", "2y" 等
	if len(durationStr) < 2 {
		return nil, bizErrors.New(response.InvalidExpiresInFormat, "无效的有效期格式")
	}
	unit := durationStr[len(durationStr)-1:]
	value, err := strconv.Atoi(durationStr[:len(durationStr)-1])
	if err != nil {
		return nil, bizErrors.New(response.InvalidExpiresInFormat, "无效的有效期格式")
	}

	switch unit {
	case "h":
		expireAt = now.Add(time.Duration(value) * time.Hour)
	case "d":
		expireAt = now.AddDate(0, 0, value)
	case "m":
		expireAt = now.AddDate(0, value, 0)
	case "y":
		expireAt = now.AddDate(value, 0, 0)
	default:
		return nil, bizErrors.New(response.InvalidExpiresInFormat, fmt.Sprintf("不支持的有效期单位: %s", unit))
	}
	// 所有成功计算出时间的路径，最后返回计算结果的指针
	return &expireAt, nil
}

// hasRepetitiveChars 检查字符串是否包含指定长度的连续重复字符
// s: 待检查的字符串
// limit: 连续字符的最小长度阈值
func hasRepetitiveChars(s string, limit int) bool {
	if len(s) < limit {
		return false
	}
	// runeCount 用于处理多字节字符（如中文）
	// 如果你的短码确认只有ASCII，可以直接遍历字节 i < len(s)
	runes := []rune(s)
	if len(runes) < limit {
		return false
	}

	count := 1
	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1] {
			count++
			if count >= limit {
				return true
			}
		} else {
			count = 1 // 重置计数器
		}
	}
	return false
}
