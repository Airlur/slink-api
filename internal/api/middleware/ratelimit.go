package middleware

import (
	"context"
	"fmt"
	"net/http"
	"math/rand"

	"slink-api/internal/model"
	"slink-api/internal/pkg/config"
	"slink-api/internal/pkg/jwt"
	"slink-api/internal/pkg/logger"
	"slink-api/internal/pkg/redis"
	"slink-api/internal/pkg/response"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10" // 令牌桶库
	goRedis "github.com/redis/go-redis/v9"
)

// RateLimitLinkCreate 创建一个用于限制短链接创建频率的中间件（固定窗口算法）
func RateLimitLinkCreate() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var key string
		var limit int
		var isGuest bool // 新增一个标志位，用于判断是否为游客

		userInfo := jwt.GetUserInfo(c)

		if userInfo != nil {
			// 对登录用户进行限流
			if userInfo.Role == model.UserRoleAdmin {
				c.Next() // 管理员不受限制
				return
			}
			key = fmt.Sprintf("ratelimit:create:user:%d", userInfo.ID)
			limit = config.GlobalConfig.RateLimit.CreatePerDayUser
			isGuest = false
		} else {
			// 对游客（按IP）进行限流
			key = fmt.Sprintf("ratelimit:create:ip:%s", c.ClientIP())
			limit = config.GlobalConfig.RateLimit.CreatePerDayGuest
			isGuest = true
		}

		// 获取INCR的结果
		count, err := redis.IncrWithExpiration(ctx, key, 24*time.Hour)
		if err != nil {
			logger.Error("短链接创建限流失败", "error", err, "key", key)
			response.Fail(c, response.InternalError, "服务暂时繁忙，请稍后访问")
			c.Abort()
			return
		}

		if int(count) > limit {
			var msg string
			if isGuest {
				// 对游客的提示
				msg = fmt.Sprintf("今日创建次数已达上限 (%d) 次，请登录后获取更多创建次数。", limit)
			} else {
				// 对普通登录用户的提示
				msg = fmt.Sprintf("今日创建次数已达上限 (%d) 次，升级会员可享受更高额度。", limit)
			}

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    response.TooManyRequests,
				"message": msg,
			})
			return
		}

		c.Next()
	}
}

// RateLimitLinkAccess 创建一个用于限制短链接访问频率的中间件（令牌桶算法）
func RateLimitLinkAccess() gin.HandlerFunc {
	limiter := redis_rate.NewLimiter(redis.Client)
	limit := redis_rate.Limit{
		Rate:   config.GlobalConfig.RateLimit.AccessPerSecondIP, // 每秒允许的速率
		Period: time.Second,                                     // 时间周期为1秒
		Burst:  config.GlobalConfig.RateLimit.AccessBurstIP,     // 峰值（桶的容量）
	}

	return func(c *gin.Context) {
		ctx := c.Request.Context()
		key := c.ClientIP()

		res, err := limiter.Allow(ctx, key, limit)
		if err != nil {
			logger.Error("访问限流令牌桶检查失败", 
				"error", err, 
				"ip", c.ClientIP(),
				"path", c.FullPath(),
			)

			response.Fail(c, response.InternalError, "服务暂时繁忙，请稍后访问")
			c.Abort()
			return
		}

		if res.Allowed == 0 {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    response.TooManyRequests,
				"message": "访问过于频繁，请稍后再试",
			})
			return
		}

		c.Next()
	}
}

// RateLimitIPBlock 创建一个全局的、用于防护恶意IP攻击的中间件（滑动窗口算法）
func RateLimitIPBlock() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ip := c.ClientIP()
		blockKey := "ratelimit:blocked:ip:" + ip
		counterKey := "ratelimit:burst:ip:" + ip

		// 1. 检查IP是否已被拉黑
		exists, err := redis.Client.Exists(ctx, blockKey).Result()
		if err != nil {
			logger.Error("检查IP黑名单失败", "error", err, "ip", ip)
			c.Next() // Redis出错，暂时放行
			return
		}
		if exists > 0 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    response.Forbidden,
				"message": "检测到异常访问行为，已被临时限制",
			})
			return
		}

		// 2. 使用Redis有序集合(ZSET)实现滑动窗口计数
		now := time.Now().UnixNano()
		window := time.Duration(config.GlobalConfig.RateLimit.BlockIPWindowSeconds) * time.Second

		pipe := redis.Client.Pipeline()
		// 使用时间戳作为score, 时间戳+随机数作为member，记录当前请求
		member := fmt.Sprintf("%d:%d", now, rand.Int63n(1000)) // 加随机数防重，或者用uuid
		pipe.ZAdd(ctx, counterKey, goRedis.Z{Score: float64(now), Member: member})
		// 移除窗口之前的所有记录
		pipe.ZRemRangeByScore(ctx, counterKey, "0", fmt.Sprintf("%d", now-window.Nanoseconds()))
		// 获取窗口内剩余的记录数
		countCmd := pipe.ZCard(ctx, counterKey)
		// 设置一个比窗口稍长的过期时间，用于自动清理冷数据
		pipe.Expire(ctx, counterKey, window+time.Minute)
		
		_, err = pipe.Exec(ctx)
		if err != nil {
			logger.Error("IP滑动窗口计数失败", "error", err, "ip", ip)
			c.Next() // Redis出错，暂时放行
			return
		}

		count := countCmd.Val()

		// 3. 如果超过阈值，则拉黑IP
		if count > int64(config.GlobalConfig.RateLimit.BlockIPMaxRequests) {
			blockDuration := time.Duration(config.GlobalConfig.RateLimit.BlockIPDurationMinutes) * time.Minute
			if err := redis.Client.Set(ctx, blockKey, "1", blockDuration).Err(); err != nil {
				logger.Error("拉黑IP失败", "error", err, "ip", ip)
			}
		}

		c.Next()
	}
}


// checkFixedWindowLimit 是固定窗口计数算法的核心实现
// 它只负责与Redis交互并返回是否超限，不关心任何业务逻辑
// 返回值: isExceeded (bool), error
func checkFixedWindowLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	count, err := redis.IncrWithExpiration(ctx, key, window)
	if err != nil {
		return false, err
	}
	return int(count) > limit, nil
}

// RateLimitAccount 返回一个按用户ID限流的中间件
func RateLimitAccount() gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := jwt.GetUserInfo(c)
		if userInfo == nil {
			c.Next() // 未登录用户，不应用此限流
			return
		}

		key := fmt.Sprintf("ratelimit:account:%d", userInfo.ID)
		limit := config.GlobalConfig.RateLimit.AccountPerMinute
		window := time.Minute
		
		exceeded, err := checkFixedWindowLimit(c.Request.Context(), key, limit, window)
		if err != nil {
			logger.Error("账户操作限流（RateLimitAccount）失败", "error", err, "key", key)
			response.Fail(c, response.InternalError, "服务繁忙，请稍后")
			c.Abort()
			return
		}

		if exceeded {
			response.Fail(c, response.TooManyRequests, "您的操作过于频繁，请稍后再试")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitDevice 返回一个按设备ID（或IP）限流的中间件
func RateLimitDevice(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		var key string
		deviceID := c.GetHeader("X-Device-ID")
		if deviceID != "" {
			key = fmt.Sprintf("ratelimit:device:%s", deviceID)
		} else {
			key = fmt.Sprintf("ratelimit:device:%s", c.ClientIP()) // 降级为IP
		}
		
		// limit := config.GlobalConfig.RateLimit.DevicePerHour
		// window := time.Hour

		exceeded, err := checkFixedWindowLimit(c.Request.Context(), key, limit, window)
		if err != nil {
			logger.Error("设备操作限流（RateLimitDevice）失败", "error", err, "key", key)
			response.Fail(c, response.InternalError, "服务繁忙，请稍后")
			c.Abort()
			return
		}

		if exceeded {
			response.Fail(c, response.TooManyRequests, "当前设备操作过于频繁,请稍后再尝试")
			c.Abort()
			return
		}

		c.Next()
	}
}


// --- 全局QPS限流器 (令牌桶) ---
// RateLimitGlobal 创建一个用于限制整个服务QPS的中间件
func RateLimitGlobal() gin.HandlerFunc {
	limiter := redis_rate.NewLimiter(redis.Client)
	limit := redis_rate.Limit{
		Rate:   config.GlobalConfig.RateLimit.GlobalQPS,
		Period: time.Second,
		Burst:  config.GlobalConfig.RateLimit.GlobalBurst,
	}
	
	return func(c *gin.Context) {
		// 全局限流使用一个固定的key
		const key = "ratelimit:global"
		
		res, err := limiter.Allow(c.Request.Context(), key, limit)
		if err != nil {
			// 全局限流失败是严重问题，可以直接响应服务不可用
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}

		if res.Allowed == 0 {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}

		c.Next()
	}
}