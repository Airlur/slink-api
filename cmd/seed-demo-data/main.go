package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"slink-api/internal/bootstrap"
	"slink-api/internal/model"
	"slink-api/internal/pkg/config"
	"slink-api/internal/pkg/logger"

	"gorm.io/gorm"
)

type presetConfig struct {
	Name               string
	PrevMonthDailyMin  int
	PrevMonthDailyMax  int
	EarlyMonthDailyMin int
	EarlyMonthDailyMax int
}

type weightedValue[T any] struct {
	Value  T
	Weight int
}

type sourceProfile struct {
	Channel string
}

type deviceProfile struct {
	UserAgent  string
	DeviceType string
	OSVersion  string
	Browser    string
}

type locationProfile struct {
	IP       string
	Country  string
	Province string
	City     string
}

type dailyKey struct {
	ShortCode string
	Date      string
}

type regionKey struct {
	ShortCode string
	Date      string
	Country   string
	Province  string
	City      string
}

type deviceKey struct {
	ShortCode  string
	Date       string
	DeviceType string
	OSVersion  string
	Browser    string
}

var (
	shortCodesRaw string
	presetName    string
	seedValue     int64
)

func main() {
	flag.StringVar(&shortCodesRaw, "short-codes", "", "逗号分隔的短码列表，例如 52KD2a,nvGQWJQ0")
	flag.StringVar(&presetName, "preset", "demo-medium", "演示造数预设：demo-small | demo-medium | demo-large")
	flag.Int64Var(&seedValue, "seed", time.Now().UnixNano(), "随机种子，默认使用当前时间")
	flag.Parse()

	shortCodes := parseShortCodes(shortCodesRaw)
	if len(shortCodes) == 0 {
		fmt.Fprintln(os.Stderr, "short-codes 不能为空，例如 --short-codes 52KD2a,nvGQWJQ0")
		os.Exit(1)
	}

	preset, err := getPreset(presetName)
	if err != nil {
		logger.Fatal("invalid preset", "preset", presetName, "error", err)
	}

	config.InitConfig()
	logger.InitLogger(&config.GlobalConfig.Logger)
	defer logger.Log.Sync()

	db, err := bootstrap.OpenDatabase()
	if err != nil {
		logger.Fatal("failed to open database", "error", err)
	}

	if err := validateShortCodes(db, shortCodes); err != nil {
		logger.Fatal("failed to validate short codes", "error", err)
	}

	rng := rand.New(rand.NewSource(seedValue))
	now := time.Now()
	logs := generateHistoryLogs(shortCodes, preset, now, rng)
	if len(logs) == 0 {
		logger.Info("no history logs generated", "preset", preset.Name)
		return
	}

	if err := persistSeedLogs(db, logs); err != nil {
		logger.Fatal("failed to persist history logs", "error", err)
	}

	logger.Info(
		"history demo data seeded",
		"preset", preset.Name,
		"seed", seedValue,
		"short_codes", len(shortCodes),
		"logs", len(logs),
	)
}

func parseShortCodes(raw string) []string {
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		code := strings.TrimSpace(part)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

func getPreset(name string) (presetConfig, error) {
	switch name {
	case "demo-small":
		return presetConfig{
			Name:               name,
			PrevMonthDailyMin:  6,
			PrevMonthDailyMax:  12,
			EarlyMonthDailyMin: 12,
			EarlyMonthDailyMax: 24,
		}, nil
	case "demo-medium":
		return presetConfig{
			Name:               name,
			PrevMonthDailyMin:  10,
			PrevMonthDailyMax:  18,
			EarlyMonthDailyMin: 20,
			EarlyMonthDailyMax: 38,
		}, nil
	case "demo-large":
		return presetConfig{
			Name:               name,
			PrevMonthDailyMin:  16,
			PrevMonthDailyMax:  30,
			EarlyMonthDailyMin: 30,
			EarlyMonthDailyMax: 56,
		}, nil
	default:
		return presetConfig{}, fmt.Errorf("unsupported preset %q", name)
	}
}

func validateShortCodes(db *gorm.DB, shortCodes []string) error {
	var existing []string
	if err := db.Model(&model.Shortlink{}).
		Where("short_code IN ?", shortCodes).
		Pluck("short_code", &existing).Error; err != nil {
		return err
	}

	exists := make(map[string]struct{}, len(existing))
	for _, code := range existing {
		exists[code] = struct{}{}
	}

	var missing []string
	for _, code := range shortCodes {
		if _, ok := exists[code]; !ok {
			missing = append(missing, code)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("这些短码不存在: %s", strings.Join(missing, ", "))
	}
	return nil
}

func generateHistoryLogs(shortCodes []string, preset presetConfig, now time.Time, rng *rand.Rand) []model.AccessLog {
	codeProfiles := buildShortCodeProfiles(shortCodes)
	sourceProfiles := buildSourceProfiles()
	deviceProfiles := buildDeviceProfiles()
	locationProfiles := buildLocationProfiles()

	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	prevMonthStart := currentMonthStart.AddDate(0, -1, 0)
	prevMonthEnd := currentMonthStart.AddDate(0, 0, -1)
	earlyMonthEnd := currentMonthStart.AddDate(0, 0, 9)
	if earlyMonthEnd.After(now.AddDate(0, 0, -1)) {
		earlyMonthEnd = now.AddDate(0, 0, -1)
	}

	var logs []model.AccessLog
	logs = append(logs, generateStageLogs(prevMonthStart, prevMonthEnd, preset.PrevMonthDailyMin, preset.PrevMonthDailyMax, codeProfiles, sourceProfiles, deviceProfiles, locationProfiles, rng)...)
	logs = append(logs, generateStageLogs(currentMonthStart, earlyMonthEnd, preset.EarlyMonthDailyMin, preset.EarlyMonthDailyMax, codeProfiles, sourceProfiles, deviceProfiles, locationProfiles, rng)...)
	return logs
}

func generateStageLogs(
	start time.Time,
	end time.Time,
	dailyMin int,
	dailyMax int,
	codeProfiles []weightedValue[string],
	sourceProfiles []weightedValue[sourceProfile],
	deviceProfiles []weightedValue[deviceProfile],
	locationProfiles []weightedValue[locationProfile],
	rng *rand.Rand,
) []model.AccessLog {
	if end.Before(start) {
		return nil
	}

	logs := make([]model.AccessLog, 0)
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		dailyCount := randBetween(rng, dailyMin, dailyMax)
		for i := 0; i < dailyCount; i++ {
			shortCode := pickWeighted(rng, codeProfiles)
			source := pickWeighted(rng, sourceProfiles)
			device := pickWeighted(rng, deviceProfiles)
			location := pickWeighted(rng, locationProfiles)
			accessedAt := randomTimeOnDate(day, rng)

			logs = append(logs, model.AccessLog{
				ShortCode:  shortCode,
				UserID:     0,
				IP:         location.IP,
				UserAgent:  device.UserAgent,
				DeviceType: device.DeviceType,
				OsVersion:  device.OSVersion,
				Browser:    device.Browser,
				Country:    location.Country,
				Province:   location.Province,
				City:       location.City,
				Channel:    source.Channel,
				AccessedAt: accessedAt,
			})
		}
	}
	return logs
}

func buildShortCodeProfiles(shortCodes []string) []weightedValue[string] {
	baseWeights := []int{26, 21, 14, 10, 8, 6, 5, 4, 4, 3, 3, 3, 2, 2, 2, 2, 1, 1, 1}
	profiles := make([]weightedValue[string], 0, len(shortCodes))
	for index, code := range shortCodes {
		weight := 1
		if index < len(baseWeights) {
			weight = baseWeights[index]
		}
		profiles = append(profiles, weightedValue[string]{
			Value:  code,
			Weight: weight,
		})
	}
	return profiles
}

func buildSourceProfiles() []weightedValue[sourceProfile] {
	return []weightedValue[sourceProfile]{
		{Value: sourceProfile{Channel: "direct"}, Weight: 6},
		{Value: sourceProfile{Channel: "weixin.qq.com"}, Weight: 16},
		{Value: sourceProfile{Channel: "weibo.com"}, Weight: 14},
		{Value: sourceProfile{Channel: "zhihu.com"}, Weight: 13},
		{Value: sourceProfile{Channel: "xiaohongshu.com"}, Weight: 11},
		{Value: sourceProfile{Channel: "douyin.com"}, Weight: 11},
		{Value: sourceProfile{Channel: "bilibili.com"}, Weight: 9},
		{Value: sourceProfile{Channel: "qq.com"}, Weight: 9},
		{Value: sourceProfile{Channel: "xiaoheihe.cn"}, Weight: 8},
		{Value: sourceProfile{Channel: "baidu.com"}, Weight: 8},
		{Value: sourceProfile{Channel: "tieba.baidu.com"}, Weight: 6},
		{Value: sourceProfile{Channel: "google.com"}, Weight: 9},
		{Value: sourceProfile{Channel: "bing.com"}, Weight: 5},
		{Value: sourceProfile{Channel: "reddit.com"}, Weight: 4},
		{Value: sourceProfile{Channel: "x.com"}, Weight: 4},
		{Value: sourceProfile{Channel: "facebook.com"}, Weight: 3},
		{Value: sourceProfile{Channel: "instagram.com"}, Weight: 3},
		{Value: sourceProfile{Channel: "youtube.com"}, Weight: 3},
	}
}

func buildDeviceProfiles() []weightedValue[deviceProfile] {
	return []weightedValue[deviceProfile]{
		{
			Value: deviceProfile{
				UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
				DeviceType: "PC",
				OSVersion:  "Windows 10",
				Browser:    "Chrome",
			},
			Weight: 24,
		},
		{
			Value: deviceProfile{
				UserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_4) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15",
				DeviceType: "PC",
				OSVersion:  "macOS 14.4",
				Browser:    "Safari",
			},
			Weight: 10,
		},
		{
			Value: deviceProfile{
				UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Edge/124.0.0.0 Safari/537.36",
				DeviceType: "PC",
				OSVersion:  "Windows 10",
				Browser:    "Edge",
			},
			Weight: 12,
		},
		{
			Value: deviceProfile{
				UserAgent:  "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 Mobile Safari/537.36",
				DeviceType: "Mobile",
				OSVersion:  "Android 14",
				Browser:    "Android",
			},
			Weight: 16,
		},
		{
			Value: deviceProfile{
				UserAgent:  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
				DeviceType: "Mobile",
				OSVersion:  "iOS 17.4",
				Browser:    "Mobile Safari UI/WKWebView",
			},
			Weight: 14,
		},
		{
			Value: deviceProfile{
				UserAgent:  "Mozilla/5.0 (iPad; CPU OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
				DeviceType: "Tablet",
				OSVersion:  "iOS 17.4",
				Browser:    "Mobile Safari UI/WKWebView",
			},
			Weight: 6,
		},
		{
			Value: deviceProfile{
				UserAgent:  "Mozilla/5.0 (Linux; Android 13; SM-T970) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.6312.99 Safari/537.36",
				DeviceType: "Tablet",
				OSVersion:  "Android 13",
				Browser:    "Chrome",
			},
			Weight: 4,
		},
		{
			Value: deviceProfile{
				UserAgent:  "Mozilla/5.0 (compatible; DemoCrawler/1.0; +https://slink.dev/bot)",
				DeviceType: "Other",
				OSVersion:  "Other",
				Browser:    "Other",
			},
			Weight: 2,
		},
	}
}

func buildLocationProfiles() []weightedValue[locationProfile] {
	return []weightedValue[locationProfile]{
		{Value: locationProfile{IP: "120.24.64.170", Country: "China", Province: "Guangdong", City: "Shenzhen"}, Weight: 14},
		{Value: locationProfile{IP: "183.60.92.44", Country: "China", Province: "Guangdong", City: "Guangzhou"}, Weight: 10},
		{Value: locationProfile{IP: "123.125.114.61", Country: "China", Province: "Beijing", City: "Beijing"}, Weight: 12},
		{Value: locationProfile{IP: "139.196.12.88", Country: "China", Province: "Shanghai", City: "Shanghai"}, Weight: 9},
		{Value: locationProfile{IP: "223.5.5.5", Country: "China", Province: "Zhejiang", City: "Hangzhou"}, Weight: 8},
		{Value: locationProfile{IP: "221.228.32.13", Country: "China", Province: "Jiangsu", City: "Nanjing"}, Weight: 7},
		{Value: locationProfile{IP: "36.112.14.119", Country: "China", Province: "Sichuan", City: "Chengdu"}, Weight: 6},
		{Value: locationProfile{IP: "111.13.101.51", Country: "China", Province: "Hubei", City: "Wuhan"}, Weight: 5},
		{Value: locationProfile{IP: "91.198.174.32", Country: "Netherlands", Province: "Noord-Holland", City: "Amsterdam"}, Weight: 4},
		{Value: locationProfile{IP: "94.140.14.119", Country: "Cyprus", Province: "Limassol", City: "Limassol"}, Weight: 4},
		{Value: locationProfile{IP: "208.67.222.164", Country: "United States", Province: "California", City: "San Jose"}, Weight: 7},
		{Value: locationProfile{IP: "9.9.9.6", Country: "United States", Province: "California", City: "Berkeley"}, Weight: 6},
		{Value: locationProfile{IP: "8.8.8.8", Country: "United States", Province: "California", City: "Mountain View"}, Weight: 6},
		{Value: locationProfile{IP: "1.1.1.1", Country: "Australia", Province: "New South Wales", City: "Sydney"}, Weight: 4},
		{Value: locationProfile{IP: "216.58.200.14", Country: "Japan", Province: "Tokyo", City: "Tokyo"}, Weight: 3},
		{Value: locationProfile{IP: "149.112.112.112", Country: "Singapore", Province: "Central Singapore", City: "Singapore"}, Weight: 3},
		{Value: locationProfile{IP: "80.80.80.80", Country: "United Kingdom", Province: "England", City: "London"}, Weight: 3},
		{Value: locationProfile{IP: "64.6.64.6", Country: "Canada", Province: "Ontario", City: "Toronto"}, Weight: 2},
	}
}

func persistSeedLogs(db *gorm.DB, logs []model.AccessLog) error {
	logsByTable := make(map[string][]model.AccessLog)
	dailyCounts := make(map[dailyKey]int)
	regionCounts := make(map[regionKey]int)
	deviceCounts := make(map[deviceKey]int)
	shortCodeCounts := make(map[string]int)

	for _, logItem := range logs {
		tableName := fmt.Sprintf("access_logs_%s", logItem.AccessedAt.Format("200601"))
		logsByTable[tableName] = append(logsByTable[tableName], logItem)

		dateKey := logItem.AccessedAt.Format("2006-01-02")
		dailyCounts[dailyKey{ShortCode: logItem.ShortCode, Date: dateKey}]++
		regionCounts[regionKey{
			ShortCode: logItem.ShortCode,
			Date:      dateKey,
			Country:   normalizeStatValue(logItem.Country),
			Province:  normalizeStatValue(logItem.Province),
			City:      normalizeStatValue(logItem.City),
		}]++
		deviceCounts[deviceKey{
			ShortCode:  logItem.ShortCode,
			Date:       dateKey,
			DeviceType: normalizeStatValue(logItem.DeviceType),
			OSVersion:  normalizeStatValue(logItem.OsVersion),
			Browser:    normalizeStatValue(logItem.Browser),
		}]++
		shortCodeCounts[logItem.ShortCode]++
	}

	return db.Transaction(func(tx *gorm.DB) error {
		tableNames := sortedKeys(logsByTable)
		for _, tableName := range tableNames {
			if err := ensureAccessLogTable(tx, tableName); err != nil {
				return err
			}
			if err := tx.Table(tableName).CreateInBatches(logsByTable[tableName], 500).Error; err != nil {
				return err
			}
		}

		if err := upsertStatsDaily(tx, dailyCounts); err != nil {
			return err
		}
		if err := upsertStatsRegionDaily(tx, regionCounts); err != nil {
			return err
		}
		if err := upsertStatsDeviceDaily(tx, deviceCounts); err != nil {
			return err
		}
		if err := incrementShortlinkClicks(tx, shortCodeCounts); err != nil {
			return err
		}

		return nil
	})
}

func ensureAccessLogTable(db *gorm.DB, tableName string) error {
	statement := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` LIKE `access_logs_template`", tableName)
	return db.Exec(statement).Error
}

func upsertStatsDaily(db *gorm.DB, counts map[dailyKey]int) error {
	keys := make([]dailyKey, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].ShortCode == keys[j].ShortCode {
			return keys[i].Date < keys[j].Date
		}
		return keys[i].ShortCode < keys[j].ShortCode
	})

	for _, key := range keys {
		if err := db.Exec(`
			INSERT INTO stats_daily (short_code, date, clicks)
			VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE clicks = clicks + VALUES(clicks)
		`, key.ShortCode, key.Date, counts[key]).Error; err != nil {
			return err
		}
	}
	return nil
}

func upsertStatsRegionDaily(db *gorm.DB, counts map[regionKey]int) error {
	keys := make([]regionKey, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].ShortCode != keys[j].ShortCode {
			return keys[i].ShortCode < keys[j].ShortCode
		}
		if keys[i].Date != keys[j].Date {
			return keys[i].Date < keys[j].Date
		}
		if keys[i].Country != keys[j].Country {
			return keys[i].Country < keys[j].Country
		}
		if keys[i].Province != keys[j].Province {
			return keys[i].Province < keys[j].Province
		}
		return keys[i].City < keys[j].City
	})

	for _, key := range keys {
		if err := db.Exec(`
			INSERT INTO stats_region_daily (short_code, date, country, province, city, clicks)
			VALUES (?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE clicks = clicks + VALUES(clicks)
		`, key.ShortCode, key.Date, key.Country, key.Province, key.City, counts[key]).Error; err != nil {
			return err
		}
	}
	return nil
}

func upsertStatsDeviceDaily(db *gorm.DB, counts map[deviceKey]int) error {
	keys := make([]deviceKey, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].ShortCode != keys[j].ShortCode {
			return keys[i].ShortCode < keys[j].ShortCode
		}
		if keys[i].Date != keys[j].Date {
			return keys[i].Date < keys[j].Date
		}
		if keys[i].DeviceType != keys[j].DeviceType {
			return keys[i].DeviceType < keys[j].DeviceType
		}
		if keys[i].OSVersion != keys[j].OSVersion {
			return keys[i].OSVersion < keys[j].OSVersion
		}
		return keys[i].Browser < keys[j].Browser
	})

	for _, key := range keys {
		if err := db.Exec(`
			INSERT INTO stats_device_daily (short_code, date, device_type, os_version, browser, clicks)
			VALUES (?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE clicks = clicks + VALUES(clicks)
		`, key.ShortCode, key.Date, key.DeviceType, key.OSVersion, key.Browser, counts[key]).Error; err != nil {
			return err
		}
	}
	return nil
}

func incrementShortlinkClicks(db *gorm.DB, counts map[string]int) error {
	shortCodes := make([]string, 0, len(counts))
	for code := range counts {
		shortCodes = append(shortCodes, code)
	}
	sort.Strings(shortCodes)

	for _, code := range shortCodes {
		if err := db.Model(&model.Shortlink{}).
			Where("short_code = ?", code).
			UpdateColumn("click_count", gorm.Expr("click_count + ?", counts[code])).Error; err != nil {
			return err
		}
	}
	return nil
}

func normalizeStatValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "Unknown"
	}
	return trimmed
}

func sortedKeys(values map[string][]model.AccessLog) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func randBetween(rng *rand.Rand, minValue int, maxValue int) int {
	if maxValue <= minValue {
		return minValue
	}
	return minValue + rng.Intn(maxValue-minValue+1)
}

func randomTimeOnDate(day time.Time, rng *rand.Rand) time.Time {
	hours := []int{8, 9, 10, 11, 13, 14, 15, 18, 20, 21, 22}
	hour := hours[rng.Intn(len(hours))]
	minute := rng.Intn(60)
	second := rng.Intn(60)
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, second, 0, day.Location())
}

func pickWeighted[T any](rng *rand.Rand, items []weightedValue[T]) T {
	totalWeight := 0
	for _, item := range items {
		totalWeight += item.Weight
	}

	target := rng.Intn(totalWeight) + 1
	current := 0
	for _, item := range items {
		current += item.Weight
		if target <= current {
			return item.Value
		}
	}

	return items[len(items)-1].Value
}
