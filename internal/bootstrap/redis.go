package bootstrap

import (
	"context"
	"fmt"
	"short-link/internal/pkg/config"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/redis"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// InitRedis 初始化Redis
func InitRedis() error {
	cfg := config.GlobalConfig.Redis

	rdb := goredis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("redis connection failed: %v", err)
	}

	// 初始化Redis包
	redis.Client = rdb
	logger.Info("Redis connection successful")
	return nil
}
