package service

import (
	"context"
	"encoding/json"
	"fmt"

	"short-link/internal/model"
	"short-link/internal/pkg/constant"
	"short-link/internal/pkg/eventbus"
	"short-link/internal/pkg/geoip"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/redis"
	"short-link/internal/pkg/uaparser"
)

type LogService interface {
	// ProcessLog 是后台Worker的核心处理函数
	ProcessLog(ctx context.Context, event eventbus.AccessLogEvent)
}

type logService struct {
}

func NewLogService() LogService {
	return &logService{}
}

// ProcessLog 实现了将日志事件写入Redis的完整逻辑
func (s *logService) ProcessLog(ctx context.Context, event eventbus.AccessLogEvent) {
	// 1. 数据丰富
	deviceType, osVersion, browser := uaparser.Parse(event.UserAgent)
	province, city := geoip.Parse(event.IP)

	// 2. 构造完整的日志模型，用于序列化
	log := &model.AccessLog{
		ShortCode:  event.ShortCode,
		UserID:     event.UserID,
		IP:         event.IP,
		UserAgent:  event.UserAgent,
		DeviceType: deviceType,
		OsVersion:  osVersion,
		Browser:    browser,
		Province:   province,
		City:       city,
		Channel:    "direct", // 来源渠道分析，暂未实现
		AccessedAt: event.Timestamp,
	}

	// 3. 【核心改造点】将数据写入Redis，使用TxPipeline确保原子性
	// 使用Pipeline批量发送命令，减少网络开销，并尝试保证 LPUSH 和 HINCRBY 的原子性

	logJSON, err := json.Marshal(log)
	if err != nil {
		logger.Error("序列化访问日志失败", "error", err, "log", log)
		return // 序列化失败，无法继续
	}

	dateStr := log.AccessedAt.Format("2006-01-02")
	pipe := redis.Client.TxPipeline() // 使用事务管道

	// 3a. 将原始日志(JSON)推入Redis List作为缓冲区
	// 我们为每个短链接创建一个独立的日志缓冲区，方便后续处理 (这里代码注释有点问题，实际是全局buffer)
	pipe.LPush(ctx, constant.LogBufferKey, logJSON)

	// 3b. 实时更新统计计数器
	// 累加总点击数 (存储在 shortlinks 主表，由定时任务更新)
	pipe.HIncrBy(ctx, fmt.Sprintf("stats:total:%s", log.ShortCode), "clicks", 1)

	// 累加按日统计
	pipe.HIncrBy(ctx, fmt.Sprintf("stats:daily:%s", log.ShortCode), dateStr, 1)

	// 累加按地域统计
	if log.Province != "Unknown" && log.City != "Unknown" {
		regionKey := fmt.Sprintf("%s:%s", log.Province, log.City)
		pipe.HIncrBy(ctx, fmt.Sprintf("stats:region:%s:%s", log.ShortCode, dateStr), regionKey, 1)
	}

	// 累加按设备统计
	if log.DeviceType != "" && log.OsVersion != "" && log.Browser != "" {
		deviceKey := fmt.Sprintf("%s:%s:%s", log.DeviceType, log.OsVersion, log.Browser)
		pipe.HIncrBy(ctx, fmt.Sprintf("stats:device:%s:%s", log.ShortCode, dateStr), deviceKey, 1)
	}

	// 执行 Pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		logger.Error("处理访问日志并更新Redis失败", "error", err)
		// 如果Redis挂了，这里会报错。由于是异步处理，用户侧无感知，但数据会丢失。
		// 在高可用方案中，这里可以降级写入本地文件或Kafka。
	}
}
