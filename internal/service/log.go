package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"slink-api/internal/model"
	"slink-api/internal/pkg/constant"
	"slink-api/internal/pkg/eventbus"
	"slink-api/internal/pkg/geoip"
	"slink-api/internal/pkg/logger"
	"slink-api/internal/pkg/redis"
	"slink-api/internal/pkg/uaparser"
)

type LogService interface {
	ProcessLog(ctx context.Context, event eventbus.AccessLogEvent)
}

type logService struct{}

func NewLogService() LogService {
	return &logService{}
}

func (s *logService) ProcessLog(ctx context.Context, event eventbus.AccessLogEvent) {
	deviceType, osVersion, browser := uaparser.Parse(event.UserAgent)
	country, province, city := geoip.Parse(event.IP)

	log := &model.AccessLog{
		ShortCode:  event.ShortCode,
		UserID:     event.UserID,
		IP:         event.IP,
		UserAgent:  event.UserAgent,
		DeviceType: deviceType,
		OsVersion:  osVersion,
		Browser:    browser,
		Country:    country,
		Province:   province,
		City:       city,
		Channel:    detectChannel(event.Referer),
		AccessedAt: event.Timestamp,
	}

	logJSON, err := json.Marshal(log)
	if err != nil {
		logger.Error("failed to marshal access log", "error", err, "shortCode", log.ShortCode)
		return
	}

	dateStr := log.AccessedAt.Format("2006-01-02")
	pipe := redis.Client.TxPipeline()
	pipe.LPush(ctx, constant.LogBufferKey, logJSON)
	pipe.HIncrBy(ctx, fmt.Sprintf("stats:total:%s", log.ShortCode), "clicks", 1)
	pipe.HIncrBy(ctx, fmt.Sprintf("stats:daily:%s", log.ShortCode), dateStr, 1)

	if log.Country != "Unknown" || log.Province != "Unknown" || log.City != "Unknown" {
		regionKey := buildRegionCounterKey(log.Country, log.Province, log.City)
		pipe.HIncrBy(ctx, fmt.Sprintf("stats:region:%s:%s", log.ShortCode, dateStr), regionKey, 1)
	}

	if log.DeviceType != "" && log.OsVersion != "" && log.Browser != "" {
		deviceKey := fmt.Sprintf("%s:%s:%s", log.DeviceType, log.OsVersion, log.Browser)
		pipe.HIncrBy(ctx, fmt.Sprintf("stats:device:%s:%s", log.ShortCode, dateStr), deviceKey, 1)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		logger.Error("failed to write access log into Redis", "error", err)
	}
}

func detectChannel(referer string) string {
	if referer == "" {
		return "direct"
	}
	u, err := url.Parse(referer)
	if err != nil {
		return "direct"
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "direct"
	}
	return host
}

func buildRegionCounterKey(country, province, city string) string {
	return strings.Join([]string{normalizeRegionValue(country), normalizeRegionValue(province), normalizeRegionValue(city)}, "|")
}

func normalizeRegionValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Unknown"
	}
	return value
}
