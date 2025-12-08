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
	// "short-link/internal/repository"
	// "golang.org/x/sync/errgroup"
)

type LogService interface {
	// ProcessLog 是后台Worker的核心处理函数
	ProcessLog(ctx context.Context, event eventbus.AccessLogEvent)
}

type logService struct {
	// logRepo repository.LogRepository
	// statsRepo repository.StatsRepository
}

func NewLogService() LogService {
	return &logService{}
}

// func NewLogService(logRepo repository.LogRepository, statsRepo repository.StatsRepository) LogService {
// 	return &logService{
// 		logRepo: logRepo,
// 		statsRepo: statsRepo,
// 	}
// }

// ProcessLog 负责处理单条访问日志事件，进行数据丰富并存入数据库
// func (s *logService) ProcessLogOld(ctx context.Context, event eventbus.AccessLogEvent) error {
// 	// 1. 数据丰富
// 	deviceType, osVersion, browser := uaparser.Parse(event.UserAgent)
// 	province, city := geoip.Parse(event.IP)

// 	// 2. 构造日志模型
// 	log := &model.AccessLog{
// 		ShortCode:  event.ShortCode,
// 		UserID:     event.UserID,
// 		IP:         event.IP,
// 		UserAgent:  event.UserAgent,
// 		DeviceType: deviceType,
// 		OsVersion:  osVersion,
// 		Browser:    browser,
// 		Province:   province,
// 		City:       city,
// 		Channel:    "direct", // 来源渠道分析，暂未实现
// 		AccessedAt: event.Timestamp,
// 	}

// 	// 3. 【核心优化】使用 errgroup 并行执行两个独立的数据库写操作
// 	var eg errgroup.Group

// 	// 任务一：存储原始日志
// 	eg.Go(func() error {
// 		err := s.logRepo.Create(ctx, log)
// 		if err != nil {
// 			// 在并发任务中，我们只记录日志，并将错误返回给 errgroup
// 			logger.Error("存储原始访问日志失败", "error", err, "log", log)
// 			return err
// 		}
// 		return nil
// 	})

// 	// 任务二：更新预聚合统计数据
// 	eg.Go(func() error {
// 		err := s.statsRepo.IncrementClicks(ctx, log)
// 		if err != nil {
// 			logger.Error("更新预聚合统计失败", "error", err, "log", log)
// 			return err
// 		}
// 		return nil
// 	})

// 	// 4. 等待两个任务完成 + 补偿机制
// 	if err := eg.Wait(); err != nil {
// 		// 如果任何一个任务失败，eg.Wait()会返回错误，我们在这里记录一个总的失败日志
// 		logger.Error("处理日志事件时发生并发错误", "error", err, "event", event)
// 		// TODO：将失败任务写入重试队列（如 Redis 或 MQ ），由定时任务补偿
//         // 此处简化：记录详细日志，人工介入或后续脚本修复
// 	}

// 	return nil
// }

// ProcessLog 实现了将日志事件写入Redis的完整逻辑
func (s *logService) ProcessLog(ctx context.Context, event eventbus.AccessLogEvent){
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

	// 3. 【核心改造点】将数据写入Redis，而不是MySQL

	// 3a. 将原始日志(JSON)推入Redis List作为缓冲区
	logJSON, err := json.Marshal(log)
	if err != nil {
		logger.Error("序列化访问日志失败", "error", err, "log", log)
		return // 序列化失败，无法继续
	}
	// 我们为每个短链接创建一个独立的日志缓冲区，方便后续处理
	if err := redis.LPush(ctx, constant.RawLogBufferKey, logJSON); err != nil {
		logger.Error("访问日志推入Redis缓冲区失败", "error", err)
	}

	// 3b. 使用 HINCRBY 实时更新统计计数器
	// 这样可以利用 Pipeline 批量发送命令，减少网络开销
	pipe := redis.Client.Pipeline()
	dateStr := log.AccessedAt.Format("2006-01-02")

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
		logger.Error("更新Redis统计计数器失败", "error", err)
	}
}