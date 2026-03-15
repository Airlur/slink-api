package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"slink-api/internal/model"
	"slink-api/internal/pkg/constant"
	"slink-api/internal/pkg/logger"
	"slink-api/internal/pkg/redis"
	"slink-api/internal/repository"

	goRedis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BatchWriterService interface {
	SyncRedisToDB(ctx context.Context) error
}

type batchWriterService struct {
	db        *gorm.DB
	logRepo   repository.LogRepository
	statsRepo repository.StatsRepository
}

func NewBatchWriterService(db *gorm.DB, logRepo repository.LogRepository, statsRepo repository.StatsRepository) BatchWriterService {
	return &batchWriterService{
		db:        db,
		logRepo:   logRepo,
		statsRepo: statsRepo,
	}
}

func (s *batchWriterService) SyncRedisToDB(ctx context.Context) error {
	logger.Info("starting Redis to DB sync")

	if err := s.syncRawLogs(ctx); err != nil {
		logger.Error("failed to sync raw logs", "error", err)
	}
	if err := s.syncStatsCounters(ctx); err != nil {
		logger.Error("failed to sync stat counters", "error", err)
	}

	logger.Info("finished Redis to DB sync")
	return nil
}

func (s *batchWriterService) syncRawLogs(ctx context.Context) error {
	processingKey := fmt.Sprintf("logs:buffer:processing:%d", time.Now().UnixNano())
	if err := redis.Rename(ctx, constant.LogBufferKey, processingKey); err != nil {
		if errors.Is(err, goRedis.Nil) || strings.Contains(err.Error(), "no such key") {
			return nil
		}
		return err
	}

	const batchSize = 500
	var allAccessLogs []*model.AccessLog
	offset := 0

	for {
		logsJSON, err := redis.LRange(ctx, processingKey, int64(offset), int64(offset+batchSize-1))
		if err != nil {
			s.restoreRawLogs(ctx, processingKey)
			return err
		}
		if len(logsJSON) == 0 {
			break
		}

		for i := len(logsJSON) - 1; i >= 0; i-- {
			var accessLog model.AccessLog
			if err := json.Unmarshal([]byte(logsJSON[i]), &accessLog); err != nil {
				logger.Warn("failed to unmarshal access log payload", "error", err)
				continue
			}
			allAccessLogs = append(allAccessLogs, &accessLog)
		}
		offset += batchSize
	}

	if len(allAccessLogs) == 0 {
		redis.Del(ctx, processingKey)
		return nil
	}

	if err := s.logRepo.CreateInBatches(ctx, allAccessLogs); err != nil {
		s.restoreRawLogs(ctx, processingKey)
		return err
	}

	redis.Del(ctx, processingKey)
	logger.Info("synced raw access logs", "count", len(allAccessLogs))
	return nil
}

func (s *batchWriterService) restoreRawLogs(ctx context.Context, processingKey string) {
	logs, err := redis.LRange(ctx, processingKey, 0, -1)
	if err != nil || len(logs) == 0 {
		return
	}

	args := make([]interface{}, len(logs))
	for i, entry := range logs {
		args[i] = entry
	}

	if err := redis.Client.RPush(ctx, constant.LogBufferKey, args...).Err(); err != nil {
		logger.Error("failed to restore raw logs into Redis buffer", "key", processingKey, "error", err)
		return
	}

	redis.Del(ctx, processingKey)
	logger.Warn("restored raw logs after database failure", "count", len(logs))
}

func (s *batchWriterService) syncStatsCounters(ctx context.Context) error {
	const batchSize = 500

	var (
		totalClickUpdates = make(map[string]int64)
		dailyStatsBuffer  []model.StatsDaily
		regionStatsBuffer []model.StatsRegionDaily
		deviceStatsBuffer []model.StatsDeviceDaily
		processedKeys     = make(map[string]string)
	)

	restoreBatch := func(keys map[string]string) {
		for processingKey, originalKey := range keys {
			s.restoreStats(ctx, processingKey, originalKey)
		}
	}

	flush := func() error {
		var errOccurred error

		for code, clicks := range totalClickUpdates {
			if err := s.db.WithContext(ctx).Model(&model.Shortlink{}).
				Where("short_code = ?", code).
				UpdateColumn("click_count", gorm.Expr("click_count + ?", clicks)).Error; err != nil {
				logger.Error("failed to update total click count", "shortCode", code, "error", err)
				errOccurred = err
			}
		}

		if len(dailyStatsBuffer) > 0 {
			if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "short_code"}, {Name: "date"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + VALUES(clicks)")}),
			}).Create(&dailyStatsBuffer).Error; err != nil {
				logger.Error("failed to write daily stats", "error", err)
				errOccurred = err
			}
		}

		if len(regionStatsBuffer) > 0 {
			if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "short_code"}, {Name: "date"}, {Name: "country"}, {Name: "province"}, {Name: "city"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + VALUES(clicks)")}),
			}).Create(&regionStatsBuffer).Error; err != nil {
				logger.Error("failed to write region stats", "error", err)
				errOccurred = err
			}
		}

		if len(deviceStatsBuffer) > 0 {
			if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "short_code"}, {Name: "date"}, {Name: "device_type"}, {Name: "os_version"}, {Name: "browser"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"clicks": gorm.Expr("clicks + VALUES(clicks)")}),
			}).Create(&deviceStatsBuffer).Error; err != nil {
				logger.Error("failed to write device stats", "error", err)
				errOccurred = err
			}
		}

		if errOccurred != nil {
			restoreBatch(processedKeys)
		} else {
			pipe := redis.Client.Pipeline()
			for processingKey := range processedKeys {
				pipe.Del(ctx, processingKey)
			}
			_, _ = pipe.Exec(ctx)
		}

		totalClickUpdates = make(map[string]int64)
		dailyStatsBuffer = dailyStatsBuffer[:0]
		regionStatsBuffer = regionStatsBuffer[:0]
		deviceStatsBuffer = deviceStatsBuffer[:0]
		processedKeys = make(map[string]string)
		return errOccurred
	}

	patterns := []string{"stats:total:*", "stats:daily:*", "stats:region:*", "stats:device:*"}
	for _, pattern := range patterns {
		var cursor uint64
		for {
			keys, nextCursor, err := redis.Client.Scan(ctx, cursor, pattern, 1000).Result()
			if err != nil {
				logger.Error("failed to scan Redis stat keys", "pattern", pattern, "error", err)
				break
			}
			cursor = nextCursor

			for _, key := range keys {
				processingKey := fmt.Sprintf("%s:processing:%d", key, time.Now().UnixNano())
				if err := redis.Rename(ctx, key, processingKey); err != nil {
					if !errors.Is(err, goRedis.Nil) && !strings.Contains(err.Error(), "no such key") {
						logger.Warn("failed to rename Redis stat key", "key", key, "error", err)
					}
					continue
				}

				dataMap, err := redis.HGetAll(ctx, processingKey)
				if err != nil {
					s.restoreStats(ctx, processingKey, key)
					continue
				}
				if len(dataMap) == 0 {
					redis.Del(ctx, processingKey)
					continue
				}

				processedKeys[processingKey] = key
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

				if len(totalClickUpdates) >= batchSize || len(dailyStatsBuffer) >= batchSize || len(regionStatsBuffer) >= batchSize || len(deviceStatsBuffer) >= batchSize {
					if err := flush(); err != nil {
						logger.Warn("failed to flush stat buffers", "error", err)
					}
				}
			}

			if cursor == 0 {
				break
			}
		}
	}

	if len(processedKeys) > 0 {
		return flush()
	}
	return nil
}

func (s *batchWriterService) appendToTotalClicks(redisKey string, dataMap map[string]string, buffer map[string]int64) {
	parts := strings.Split(redisKey, ":")
	if len(parts) < 3 {
		return
	}
	shortCode := parts[2]
	clicks, _ := strconv.ParseInt(dataMap["clicks"], 10, 64)
	if clicks > 0 {
		buffer[shortCode] += clicks
	}
}

func (s *batchWriterService) appendToDailyStats(redisKey string, dataMap map[string]string, buffer *[]model.StatsDaily) {
	parts := strings.Split(redisKey, ":")
	if len(parts) < 3 {
		return
	}
	shortCode := parts[2]

	for dateStr, clicksStr := range dataMap {
		clicks, _ := strconv.ParseInt(clicksStr, 10, 64)
		date, _ := time.Parse("2006-01-02", dateStr)
		if clicks <= 0 {
			continue
		}
		*buffer = append(*buffer, model.StatsDaily{
			ShortCode: shortCode,
			Date:      date,
			Clicks:    uint(clicks),
		})
	}
}

func (s *batchWriterService) appendToRegionStats(redisKey string, dataMap map[string]string, buffer *[]model.StatsRegionDaily) {
	parts := strings.Split(redisKey, ":")
	if len(parts) < 4 {
		return
	}
	shortCode, dateStr := parts[2], parts[3]
	date, _ := time.Parse("2006-01-02", dateStr)

	for regionKey, clicksStr := range dataMap {
		clicks, _ := strconv.ParseInt(clicksStr, 10, 64)
		if clicks <= 0 {
			continue
		}
		country, province, city := parseRegionCounterKey(regionKey)
		*buffer = append(*buffer, model.StatsRegionDaily{
			ShortCode: shortCode,
			Date:      date,
			Country:   country,
			Province:  province,
			City:      city,
			Clicks:    uint(clicks),
		})
	}
}

func (s *batchWriterService) appendToDeviceStats(redisKey string, dataMap map[string]string, buffer *[]model.StatsDeviceDaily) {
	parts := strings.Split(redisKey, ":")
	if len(parts) < 4 {
		return
	}
	shortCode, dateStr := parts[2], parts[3]
	date, _ := time.Parse("2006-01-02", dateStr)

	for deviceKey, clicksStr := range dataMap {
		clicks, _ := strconv.ParseInt(clicksStr, 10, 64)
		if clicks <= 0 {
			continue
		}
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

func (s *batchWriterService) restoreStats(ctx context.Context, processingKey, originalKey string) {
	dataMap, err := redis.HGetAll(ctx, processingKey)
	if err != nil || len(dataMap) == 0 {
		return
	}

	pipe := redis.Client.Pipeline()
	for field, valueStr := range dataMap {
		value, _ := strconv.ParseInt(valueStr, 10, 64)
		if value > 0 {
			pipe.HIncrBy(ctx, originalKey, field, value)
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		logger.Error("failed to restore stat buffer into Redis", "key", originalKey, "error", err)
		return
	}

	redis.Del(ctx, processingKey)
	logger.Warn("restored stat counter after database failure", "key", originalKey)
}

func parseRegionCounterKey(regionKey string) (country, province, city string) {
	country = "Unknown"
	province = "Unknown"
	city = "Unknown"

	if strings.Contains(regionKey, "|") {
		parts := strings.Split(regionKey, "|")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			country = parts[0]
		}
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			province = parts[1]
		}
		if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
			city = parts[2]
		}
		return
	}

	legacyParts := strings.Split(regionKey, ":")
	if len(legacyParts) > 0 && strings.TrimSpace(legacyParts[0]) != "" {
		province = legacyParts[0]
	}
	if len(legacyParts) > 1 && strings.TrimSpace(legacyParts[1]) != "" {
		city = legacyParts[1]
	}
	return
}
