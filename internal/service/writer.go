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
func (s *batchWriterService) syncRawLogs(ctx context.Context) error {
	// 定义一个临时的、唯一的处理中key
	processingKey := fmt.Sprintf("logs:buffer:processing:%d", time.Now().UnixNano())

	// 原子性重命名日志缓冲区，实现安全的“日志轮转”
	err := redis.Rename(ctx, constant.LogBufferKey, processingKey)
	if err != nil {
		if errors.Is(err, goRedis.Nil) || strings.Contains(err.Error(), "no such key") {
			// logger.Info("没有待处理的原始日志，跳过同步") // 降低日志噪音
			return nil
		}
		return err
	}

	// 分批次从Redis读取日志（每批500条）
	batchSize := 500
	var allAccessLogs []*model.AccessLog
	offset := 0 // Redis列表的起始偏移量

	for {
		// 每次读取batchSize条日志（LRANGE key start stop，stop=start+batchSize-1）
		logsJSON, err := redis.LRange(ctx, processingKey, int64(offset), int64(offset+batchSize-1))
		if err != nil {
			logger.Error("从Redis分批获取日志失败", "error", err, "offset", offset)
			s.restoreRawLogs(ctx, processingKey) // 尝试恢复
			return err
		}

		// 若读取到空切片，说明已读完所有日志
		if len(logsJSON) == 0 {
			break
		}

		// 反序列化当前批次日志（Redis List是先进先出还是后进先出取决于Push方式，
		// 这里假设我们关心的是将数据全部入库，顺序只要相对一致即可。
		// 如果是 LPush + LRange，则 logsJSON[0] 是最新的。
		// 入库顺序通常不强制要求严格时间序，只要都进去就行。）
		for i := len(logsJSON) - 1; i >= 0; i-- {
			var log model.AccessLog
			if err := json.Unmarshal([]byte(logsJSON[i]), &log); err == nil {
				allAccessLogs = append(allAccessLogs, &log)
			} else {
				logger.Error("日志反序列化失败", "data", logsJSON[i])
			}
		}

		// 更新偏移量，准备读取下一批
		offset += batchSize
	}

	// 若没有日志（可能全是坏数据），直接删除processingKey
	if len(allAccessLogs) == 0 {
		redis.Del(ctx, processingKey)
		return nil
	}

	// 调用Repository分批写入
	if err := s.logRepo.CreateInBatches(ctx, allAccessLogs); err != nil {
		logger.Error("批量写入原始日志到数据库失败", "error", err)
		// 写入失败，必须将日志恢复回 Redis，避免数据丢失
		s.restoreRawLogs(ctx, processingKey)
		return err
	}

	// 成功后删除临时key
	redis.Del(ctx, processingKey)
	logger.Info("批量同步原始日志成功", "总日志条数", len(allAccessLogs))
	return nil
}

// restoreRawLogs 将处理中的日志恢复回主缓冲区
func (s *batchWriterService) restoreRawLogs(ctx context.Context, processingKey string) {
	// 获取所有待恢复的日志
	logs, err := redis.LRange(ctx, processingKey, 0, -1)
	if err != nil || len(logs) == 0 {
		return
	}

	// 将日志追加回主缓冲区的末尾（RPush），保持大致的时间顺序
	// 注意：go-redis 的 RPush 支持 []interface{}，我们需要转换一下
	args := make([]interface{}, len(logs))
	for i, v := range logs {
		args[i] = v
	}
	
	if err := redis.Client.RPush(ctx, constant.LogBufferKey, args...).Err(); err != nil {
		logger.Error("严重错误：日志恢复失败，数据可能丢失！", "key", processingKey, "error", err)
	} else {
		logger.Warn("数据库写入失败，已将日志恢复回Redis缓冲区", "count", len(logs))
		// 恢复成功后，可以删除 processingKey，或者留着让下次重试（Rename会覆盖）。
		// 既然已经 RPush 回去了，就应该删除 processingKey，否则下次 Rename 会把 processingKey 里的旧数据丢弃（如果它是目标），
		// 或者如果下次 Rename 成功了，我们这里残留的 processingKey 就变成了垃圾数据。
		// 这里选择删除 processingKey。
		redis.Del(ctx, processingKey)
	}
}


// syncStatsCounters 负责同步所有统计计数器
func (s *batchWriterService) syncStatsCounters(ctx context.Context) error {
	// 定义各维度的批处理缓冲区
	const batchSize = 500
	var (
		// Total Clicks: map[shortCode]clicks
		totalClickUpdates = make(map[string]int64)
		
		// Daily Stats: slice of models
		dailyStatsBuffer []model.StatsDaily
		
		// Region Stats: slice of models
		regionStatsBuffer []model.StatsRegionDaily
		
		// Device Stats: slice of models
		deviceStatsBuffer []model.StatsDeviceDaily
		
		// 记录当前批次处理过的临时Key，用于写入成功后删除或失败后恢复
		// map[tempKey]originalKey
		processedKeys = make(map[string]string)
	)

	// 定义数据恢复函数（在数据库写入失败时调用）
	restoreBatch := func(keys map[string]string) {
		for tempKey, originalKey := range keys {
			s.restoreStats(ctx, tempKey, originalKey)
		}
	}

	// 定义Flush函数：将缓冲区数据写入数据库
	flush := func() error {
		var errOccurred error

		// 1. 批量更新总点击数 (Shortlink表)
		// 由于GORM不支持批量更新不同行的不同值，这里我们对总点击数只能逐个更新，或者按点击数分组更新。
		// 为了简单和安全，我们这里先逐个更新，但因为是在 Flush 里，至少减少了外层循环的逻辑复杂度。
		// *优化方向*：可以使用 CASE WHEN 语句拼接原生 SQL 实现一次 DB 调用更新多行。
		// 但考虑到 TotalClicks 更新频率不如日志高（因为只有增量），且 GORM 拼接大 SQL 有风险，
		// 这里我们采用 "分组并发" 或 "简单遍历"。
		// 鉴于 syncStatsCounters 是串行的，我们简单遍历。
		for code, clicks := range totalClickUpdates {
			if err := s.db.WithContext(ctx).Model(&model.Shortlink{}).
				Where("short_code = ?", code).
				UpdateColumn("click_count", gorm.Expr("click_count + ?", clicks)).Error; err != nil {
				logger.Error("更新总点击数失败", "short_code", code, "error", err)
				errOccurred = err
			}
		}

		// 2. 批量写入每日统计
		if len(dailyStatsBuffer) > 0 {
			if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "short_code"}, {Name: "date"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + VALUES(clicks)")}),
			}).Create(&dailyStatsBuffer).Error; err != nil {
				logger.Error("批量写入每日统计失败", "error", err)
				errOccurred = err
			}
		}

		// 3. 批量写入地域统计
		if len(regionStatsBuffer) > 0 {
			if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "short_code"}, {Name: "date"}, {Name: "province"}, {Name: "city"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + VALUES(clicks)")}),
			}).Create(&regionStatsBuffer).Error; err != nil {
				logger.Error("批量写入地域统计失败", "error", err)
				errOccurred = err
			}
		}

		// 4. 批量写入设备统计
		if len(deviceStatsBuffer) > 0 {
			if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "short_code"}, {Name: "date"}, {Name: "device_type"}, {Name: "os_version"}, {Name: "browser"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + VALUES(clicks)")}),
			}).Create(&deviceStatsBuffer).Error; err != nil {
				logger.Error("批量写入设备统计失败", "error", err)
				errOccurred = err
			}
		}

		// 5. 善后处理
		if errOccurred != nil {
			// 如果有错误，尝试恢复这一批次的所有Key
			logger.Warn("批量写入部分失败，尝试恢复数据", "keys_count", len(processedKeys))
			restoreBatch(processedKeys)
		} else {
			// 如果成功，删除所有临时Key
			pipe := redis.Client.Pipeline()
			for tempKey := range processedKeys {
				pipe.Del(ctx, tempKey)
			}
			pipe.Exec(ctx)
		}

		// 6. 清空缓冲区
		totalClickUpdates = make(map[string]int64)
		dailyStatsBuffer = dailyStatsBuffer[:0]
		regionStatsBuffer = regionStatsBuffer[:0]
		deviceStatsBuffer = deviceStatsBuffer[:0]
		processedKeys = make(map[string]string)

		return errOccurred
	}

	// -----------------------------------------------------------
	// 核心扫描循环
	// -----------------------------------------------------------
	patterns := []string{"stats:total:*", "stats:daily:*", "stats:region:*", "stats:device:*"}

	for _, pattern := range patterns {
		var cursor uint64
		var keys []string
		var err error

		for {
			// SCAN 1000 keys at a time
			keys, cursor, err = redis.Client.Scan(ctx, cursor, pattern, 1000).Result()
			if err != nil {
				logger.Error("Redis Scan 失败", "pattern", pattern, "error", err)
				break 
			}

			for _, key := range keys {
				// 1. 生成临时Key并重命名 (Atomically Move)
				processingKey := fmt.Sprintf("%s:processing:%d", key, time.Now().UnixNano())
				if err := redis.Rename(ctx, key, processingKey); err != nil {
					// 只有在 Key 不存在时（被删或过期）才忽略错误，其他错误需记录
					if !errors.Is(err, goRedis.Nil) && !strings.Contains(err.Error(), "no such key") {
						logger.Warn("Rename 统计Key失败", "key", key, "error", err)
					}
					continue
				}

				// 2. 获取数据
				dataMap, err := redis.HGetAll(ctx, processingKey)
				if err != nil {
					logger.Error("HGetAll 统计数据失败", "key", processingKey, "error", err)
					s.restoreStats(ctx, processingKey, key) // 立即尝试单条恢复
					continue
				}
				if len(dataMap) == 0 {
					redis.Del(ctx, processingKey)
					continue
				}

				// 3. 将处理中的 Key 记录下来，以便 Flush 时处理
				processedKeys[processingKey] = key

				// 4. 解析数据并放入缓冲区
				//    注意：这里我们不需要立即 Handle DB Error，因为我们只是 append 到 slice
				switch {
				case strings.HasPrefix(key, "stats:total:"):
					s.appendToTotalClicks(key, dataMap, totalClickUpdates)
				case strings.HasPrefix(key, "stats:daily:"):
					s.appendToDailyStats(key, dataMap, &dailyStatsBuffer)
				case strings.HasPrefix(key, "stats:region:"):
					s.appendToRegionStats(key, dataMap, &regionStatsBuffer)
				case strings.HasPrefix(key, "stats:device:"):
					s.appendToDeviceStats(key, dataMap, &deviceStatsBuffer)
				}

				// 5. 检查是否需要 Flush (任意缓冲区满)
				//    由于 patterns 是顺序处理的，buffer 增长通常集中在某一种类型上
				if len(totalClickUpdates) >= batchSize ||
					len(dailyStatsBuffer) >= batchSize ||
					len(regionStatsBuffer) >= batchSize ||
					len(deviceStatsBuffer) >= batchSize {
					
					flush() // 执行写入并清空
				}
			}

			if cursor == 0 {
				break
			}
		}
	}

	// 循环结束后，处理剩余的数据
	if len(processedKeys) > 0 {
		flush()
	}

	return nil
}

// --- Append Helpers (将原有 Update 逻辑改为 Append) ---

func (s *batchWriterService) appendToTotalClicks(redisKey string, dataMap map[string]string, buffer map[string]int64) {
	// key: stats:total:{short_code}
	parts := strings.Split(redisKey, ":")
	if len(parts) < 3 {
		return
	}
	shortCode := parts[2]
	clicks, _ := strconv.ParseInt(dataMap["clicks"], 10, 64)
	if clicks > 0 {
		buffer[shortCode] += clicks // 聚合内存中的点击数（虽然一般只有一个，但防万一）
	}
}

func (s *batchWriterService) appendToDailyStats(redisKey string, dataMap map[string]string, buffer *[]model.StatsDaily) {
	// key: stats:daily:{short_code}
	parts := strings.Split(redisKey, ":")
	if len(parts) < 3 {
		return
	}
	shortCode := parts[2]

	for dateStr, clicksStr := range dataMap {
		clicks, _ := strconv.ParseInt(clicksStr, 10, 64)
		date, _ := time.Parse("2006-01-02", dateStr)
		if clicks > 0 {
			*buffer = append(*buffer, model.StatsDaily{
				ShortCode: shortCode,
				Date:      date,
				Clicks:    uint(clicks),
			})
		}
	}
}

func (s *batchWriterService) appendToRegionStats(redisKey string, dataMap map[string]string, buffer *[]model.StatsRegionDaily) {
	// key: stats:region:{short_code}:{date}
	parts := strings.Split(redisKey, ":")
	if len(parts) < 4 {
		return
	}
	shortCode, dateStr := parts[2], parts[3]
	date, _ := time.Parse("2006-01-02", dateStr)

	for regionKey, clicksStr := range dataMap {
		clicks, _ := strconv.ParseInt(clicksStr, 10, 64)
		if clicks == 0 {
			continue
		}
		regionParts := strings.Split(regionKey, ":") // Province:City
		province := regionParts[0]
		city := "Unknown"
		if len(regionParts) > 1 {
			city = regionParts[1]
		}
		*buffer = append(*buffer, model.StatsRegionDaily{
			ShortCode: shortCode,
			Date:      date,
			Province:  province,
			City:      city,
			Clicks:    uint(clicks),
		})
	}
}

func (s *batchWriterService) appendToDeviceStats(redisKey string, dataMap map[string]string, buffer *[]model.StatsDeviceDaily) {
	// key: stats:device:{short_code}:{date}
	parts := strings.Split(redisKey, ":")
	if len(parts) < 4 {
		return
	}
	shortCode, dateStr := parts[2], parts[3]
	date, _ := time.Parse("2006-01-02", dateStr)

	for deviceKey, clicksStr := range dataMap {
		clicks, _ := strconv.ParseInt(clicksStr, 10, 64)
		if clicks == 0 {
			continue
		}
		// deviceKey: Device:OS:Browser
		deviceParts := strings.Split(deviceKey, ":")
		if len(deviceParts) < 3 {
			continue
		}
		*buffer = append(*buffer, model.StatsDeviceDaily{
			ShortCode:  shortCode,
			Date:       date,
			DeviceType: deviceParts[0],
			OsVersion:  deviceParts[1],
			Browser:    deviceParts[2],
			Clicks:     uint(clicks),
		})
	}
}

// restoreStats 将统计数据恢复回原Key
func (s *batchWriterService) restoreStats(ctx context.Context, processingKey, originalKey string) {
	dataMap, err := redis.HGetAll(ctx, processingKey)
	if err != nil || len(dataMap) == 0 {
		return
	}

	pipe := redis.Client.Pipeline()
	for field, valStr := range dataMap {
		val, _ := strconv.ParseInt(valStr, 10, 64)
		if val > 0 {
			// 使用 HIncrBy 将数据加回去，这样即使 originalKey 在此期间有了新数据，也是累加而不是覆盖
			pipe.HIncrBy(ctx, originalKey, field, val)
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		logger.Error("严重错误：统计数据恢复失败！", "key", originalKey, "error", err)
	} else {
		logger.Warn("数据库写入失败，已将统计数据恢复回Redis", "key", originalKey)
		redis.Del(ctx, processingKey)
	}
}