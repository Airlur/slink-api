package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"short-link/internal/model"
	"short-link/internal/pkg/constant"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/redis"
	"short-link/internal/repository"

	goRedis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BatchWriterService 定义了批量数据写入服务的接口
type BatchWriterService interface {
	SyncRedisToDB(ctx context.Context) error
}

type batchWriterService struct {
	db        *gorm.DB
	logRepo   repository.LogRepository
	statsRepo repository.StatsRepository
}

// NewBatchWriterService 创建一个新的批量写入服务实例
func NewBatchWriterService(db *gorm.DB, logRepo repository.LogRepository, statsRepo repository.StatsRepository) BatchWriterService {
	return &batchWriterService{
		db:        db,
		logRepo:   logRepo,
		statsRepo: statsRepo,
	}
}

// SyncRedisToDB 是定时任务的核心，负责将Redis数据同步到MySQL
func (s *batchWriterService) SyncRedisToDB(ctx context.Context) error {
	logger.Info("开始执行 [Redis -> DB] 数据同步任务")

	// 串行执行，确保在处理统计数据前，原始日志已尽可能入库
	if err := s.syncRawLogs(ctx); err != nil {
		logger.Error("同步原始访问日志时发生错误", "error", err)
		// 即使失败，也继续同步统计，避免相互影响
	}
	if err := s.syncStatsCounters(ctx); err != nil {
		logger.Error("同步统计计数器时发生错误", "error", err)
	}

	logger.Info("[Redis -> DB] 数据同步任务执行完毕")
	return nil
}

// syncRawLogs 负责同步原始日志
func (s *batchWriterService) syncRawLogsOld(ctx context.Context) error {
	// 1. 定义一个临时的、唯一的处理中key
	processingKey := fmt.Sprintf("logs:buffer:processing:%d", time.Now().UnixNano())

	// 2. 原子性地重命名日志缓冲区，实现安全的“日志轮转”
	err := redis.Rename(ctx, constant.RawLogBufferKey, processingKey)
	if err != nil {
		// 这是一种正常情况，意味着上一分钟内没有任何访问日志
		if errors.Is(err, goRedis.Nil) || (strings.Contains(err.Error(), "no such key")) {
			logger.Info("没有待处理的原始日志，跳过同步")
			return nil // 没有待处理的日志，是正常情况
		}
		return err
	}

	// 3. 从已隔离的processingKey中安全地取出所有日志
	logsJSON, err := redis.LRange(ctx, processingKey, 0, -1)
	if err != nil {
		logger.Error("从Redis获取待处理日志失败", "error", err)
		return err
	}
	if len(logsJSON) == 0 {
		redis.Del(ctx, processingKey)
		return nil
	}

	// 4. 反序列化JSON字符串为Go结构体
	var accessLogs []*model.AccessLog
	for i := len(logsJSON) - 1; i >= 0; i-- { // LRange + LPush 导致顺序是反的，需要逆序遍历回来
		var log model.AccessLog
		if err := json.Unmarshal([]byte(logsJSON[i]), &log); err == nil {
			accessLogs = append(accessLogs, &log)
		}
	}

	// 5. 调用Repository层进行批量写入
	if err := s.logRepo.CreateInBatches(ctx, accessLogs); err != nil {
		logger.Error("批量写入原始日志到数据库失败", "error", err)
		// 补偿机制：可以将失败的键重新写回主缓冲区
		// redis.Rename(ctx, processingKey, rawLogBufferKey)
		return err
	}

	// 6. 成功写入数据库后，删除临时的processingKey
	redis.Del(ctx, processingKey)
	logger.Info("批量同步原始日志成功", "日志条数", len(accessLogs))
	return nil
}

func (s *batchWriterService) syncRawLogs(ctx context.Context) error {
	// 定义一个临时的、唯一的处理中key
	processingKey := fmt.Sprintf("logs:buffer:processing:%d", time.Now().UnixNano())

	// 原子性重命名日志缓冲区，实现安全的“日志轮转”
	err := redis.Rename(ctx, constant.RawLogBufferKey, processingKey)
	if err != nil {
		if errors.Is(err, goRedis.Nil) || strings.Contains(err.Error(), "no such key") {
			logger.Info("没有待处理的原始日志，跳过同步")
			return nil
		}
		return err
	}

	// 【核心修改2】分批次从Redis读取日志（每批500条）
	batchSize := 500
	var allAccessLogs []*model.AccessLog
	offset := 0 // Redis列表的起始偏移量

	for {
		// 每次读取batchSize条日志（LRANGE key start stop，stop=start+batchSize-1）
		logsJSON, err := redis.LRange(ctx, processingKey, int64(offset), int64(offset+batchSize-1))
		if err != nil {
			logger.Error("从Redis分批获取日志失败", "error", err, "offset", offset)
			return err
		}

		// 若读取到空切片，说明已读完所有日志
		if len(logsJSON) == 0 {
			break
		}

		// 反序列化当前批次日志（原有逻辑保留，逆序处理）
		for i := len(logsJSON) - 1; i >= 0; i-- {
			var log model.AccessLog
			if err := json.Unmarshal([]byte(logsJSON[i]), &log); err == nil {
				allAccessLogs = append(allAccessLogs, &log)
			}
		}

		// 更新偏移量，准备读取下一批
		offset += batchSize
	}

	// 若没有日志，直接删除processingKey
	if len(allAccessLogs) == 0 {
		redis.Del(ctx, processingKey)
		return nil
	}

	// 调用Repository分批写入（复用步骤1的修改）
	if err := s.logRepo.CreateInBatches(ctx, allAccessLogs); err != nil {
		logger.Error("批量写入原始日志到数据库失败", "error", err)
		// 【可选补偿】若写入失败，将processingKey重命名回原缓冲区（避免日志丢失）
		// redis.Rename(ctx, processingKey, constant.RawLogBufferKey)
		return err
	}

	// 成功后删除临时key
	redis.Del(ctx, processingKey)
	logger.Info("批量同步原始日志成功", "总日志条数", len(allAccessLogs))
	return nil
}
// syncStatsCounters 负责同步所有统计计数器
func (s *batchWriterService) syncStatsCounters(ctx context.Context) error {
	// 定义需要扫描的key模式
	patterns := []string{"stats:total:*", "stats:daily:*", "stats:region:*", "stats:device:*"}
	var allKeys []string
	for _, pattern := range patterns {
		// 注意：生产环境如果Key数量巨大，应使用SCAN代替KEYS以避免阻塞
		keys, err := redis.Client.Keys(ctx, pattern).Result()
		if err != nil {
			logger.Warn("扫描统计key失败", "error", err, "pattern", pattern)
			continue
		}
		allKeys = append(allKeys, keys...)
	}

	if len(allKeys) == 0 {
		return nil
	}

	// 遍历所有待处理的统计key
	for _, key := range allKeys {
		processingKey := fmt.Sprintf("%s:processing:%d", key, time.Now().UnixNano())

		// 原子性地重命名，将数据“收割”过来
		if err := redis.Rename(ctx, key, processingKey); err != nil {
			if errors.Is(err, goRedis.Nil) {
				continue
			}
			logger.Warn("重命名统计key失败", "error", err, "key", key)
			continue
		}

		// 获取收割过来的哈希表中的所有数据
		dataMap, err := redis.HGetAll(ctx, processingKey)
		if err != nil {
			logger.Error("HGetAll 统计数据失败", "error", err, "key", processingKey)
			continue
		}

		// 根据key的前缀，分发到不同的处理函数进行批量数据库更新
		var dbErr error
		switch {
		case strings.HasPrefix(key, "stats:total:"):
			dbErr = s.batchUpdateTotalClicks(ctx, key, dataMap)
		case strings.HasPrefix(key, "stats:daily:"):
			dbErr = s.batchUpdateDailyStats(ctx, key, dataMap)
		case strings.HasPrefix(key, "stats:region:"):
			dbErr = s.batchUpdateRegionStats(ctx, key, dataMap)
		case strings.HasPrefix(key, "stats:device:"):
			dbErr = s.batchUpdateDeviceStats(ctx, key, dataMap)
		}

		// 根据数据库操作结果决定是否删除processingKey
		if dbErr != nil {
			logger.Error("批量更新数据库统计失败", "error", dbErr, "key", processingKey)
			// TODO: 补偿机制，例如将 processingKey 重命名回原名
		} else {
			redis.Del(ctx, processingKey)
		}
	}
	logger.Info("批量同步统计计数器成功", "处理Key数量", len(allKeys))
	return nil
}

// --- 具体的批量更新辅助函数 ---

// batchUpdateTotalClicks 批量更新主表的总点击数
func (s *batchWriterService) batchUpdateTotalClicks(ctx context.Context, redisKey string, dataMap map[string]string) error {
	shortCode := strings.Split(redisKey, ":")[2]
	clicks, _ := strconv.ParseInt(dataMap["clicks"], 10, 64)
	if clicks == 0 {
		return nil
	}

	// 使用 gorm.Expr 实现原子性的 "click_count = click_count + ?"
	return s.db.WithContext(ctx).Model(&model.Shortlink{}).Where("short_code = ?", shortCode).
		UpdateColumn("click_count", gorm.Expr("click_count + ?", clicks)).Error
}

// batchUpdateDailyStats 批量更新每日统计表
func (s *batchWriterService) batchUpdateDailyStats(ctx context.Context, redisKey string, dataMap map[string]string) error {
	shortCode := strings.Split(redisKey, ":")[2]

	// 准备批量更新的数据切片
	var statsToUpsert []model.StatsDaily
	for dateStr, clicksStr := range dataMap {
		clicks, _ := strconv.ParseInt(clicksStr, 10, 64)
		date, _ := time.Parse("2006-01-02", dateStr)
		if clicks > 0 {
			statsToUpsert = append(statsToUpsert, model.StatsDaily{
				ShortCode: shortCode,
				Date:      date,
				Clicks:    uint(clicks),
			})
		}
	}

	// 如果有数据需要更新，则执行批量 ON DUPLICATE KEY UPDATE
	if len(statsToUpsert) > 0 {
		return s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "short_code"}, {Name: "date"}},
			// 当唯一键冲突时，将现有的 clicks 字段加上新传入的值
			DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + VALUES(clicks)")}),
		}).Create(&statsToUpsert).Error
	}
	return nil
}

// batchUpdateRegionStats 批量更新地域统计表
func (s *batchWriterService) batchUpdateRegionStats(ctx context.Context, redisKey string, dataMap map[string]string) error {
	parts := strings.Split(redisKey, ":")
	if len(parts) < 4 {
		return fmt.Errorf("invalid region stats key: %s", redisKey)
	}
	shortCode, dateStr := parts[2], parts[3]
	date, _ := time.Parse("2006-01-02", dateStr)

	var statsToUpsert []model.StatsRegionDaily
	for regionKey, clicksStr := range dataMap {
		clicks, _ := strconv.ParseInt(clicksStr, 10, 64)
		regionParts := strings.Split(regionKey, ":")
		if len(regionParts) < 1 || clicks == 0 {
			continue
		}

		province := regionParts[0]
		city := "Unknown" // 默认值
		if len(regionParts) > 1 {
			city = regionParts[1]
		}
		statsToUpsert = append(statsToUpsert, model.StatsRegionDaily{
			ShortCode: shortCode,
			Date:      date,
			Province:  province,
			City:      city,
			Clicks:    uint(clicks),
		})
	}

	if len(statsToUpsert) > 0 {
		return s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "short_code"}, {Name: "date"}, {Name: "province"}, {Name: "city"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + VALUES(clicks)")}),
		}).Create(&statsToUpsert).Error
	}
	return nil
}

// batchUpdateDeviceStats 批量更新设备统计表
func (s *batchWriterService) batchUpdateDeviceStats(ctx context.Context, redisKey string, dataMap map[string]string) error {
	parts := strings.Split(redisKey, ":")
	if len(parts) < 4 {
		return fmt.Errorf("invalid device stats key: %s", redisKey)
	}
	shortCode, dateStr := parts[2], parts[3]
	date, _ := time.Parse("2006-01-02", dateStr)

	var statsToUpsert []model.StatsDeviceDaily
	for deviceKey, clicksStr := range dataMap {
		clicks, _ := strconv.ParseInt(clicksStr, 10, 64)
		deviceParts := strings.Split(deviceKey, ":")
		if len(deviceParts) < 3 || clicks == 0 {
			continue
		}

		statsToUpsert = append(statsToUpsert, model.StatsDeviceDaily{
			ShortCode:  shortCode,
			Date:       date,
			DeviceType: deviceParts[0],
			OsVersion:  deviceParts[1],
			Browser:    deviceParts[2],
			Clicks:     uint(clicks),
		})
	}

	if len(statsToUpsert) > 0 {
		return s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "short_code"}, {Name: "date"}, {Name: "device_type"}, {Name: "os_version"}, {Name: "browser"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + VALUES(clicks)")}),
		}).Create(&statsToUpsert).Error
	}
	return nil
}