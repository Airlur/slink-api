package main

import (
	"flag"
	"fmt"
	"math"
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

type countRow struct {
	ShortCode string `gorm:"column:short_code"`
	Clicks    int64  `gorm:"column:clicks"`
}

type stagePlan struct {
	Name    string
	Weights []dateWeight
}

type dateWeight struct {
	Date   time.Time
	Weight int
}

var (
	shortCodesRaw string
	targetsRaw    string
	seedValue     int64
	cleanupOnly   bool
	seedOnly      bool
	rebuildOnly   bool
)

func main() {
	flag.StringVar(&shortCodesRaw, "short-codes", "", "comma-separated short codes")
	flag.StringVar(&targetsRaw, "targets", "", "comma-separated target counts, e.g. 52KD2a=820,nvGQWJQ0=620")
	flag.Int64Var(&seedValue, "seed", 20260315, "random seed")
	flag.BoolVar(&cleanupOnly, "cleanup-only", false, "only cleanup and rebuild remaining baseline")
	flag.BoolVar(&seedOnly, "seed-only", false, "skip cleanup and only seed exact demo data")
	flag.BoolVar(&rebuildOnly, "rebuild-only", false, "skip cleanup and seed; only rebuild aggregates from raw logs")
	flag.Parse()

	if (cleanupOnly && seedOnly) || (cleanupOnly && rebuildOnly) || (seedOnly && rebuildOnly) {
		fmt.Fprintln(os.Stderr, "cleanup-only, seed-only and rebuild-only are mutually exclusive")
		os.Exit(1)
	}

	shortCodes := parseShortCodes(shortCodesRaw)
	if len(shortCodes) == 0 {
		fmt.Fprintln(os.Stderr, "short-codes cannot be empty")
		os.Exit(1)
	}

	targets, err := parseTargets(targetsRaw)
	if err != nil {
		logger.Fatal("invalid targets", "error", err)
	}

	if !cleanupOnly && !rebuildOnly {
		if len(targets) == 0 {
			logger.Fatal("targets are required unless cleanup-only is set")
		}
		if err := validateTargetCoverage(shortCodes, targets); err != nil {
			logger.Fatal("target coverage mismatch", "error", err)
		}
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

	now := time.Now()
	tables, err := listAccessLogTables(db)
	if err != nil {
		logger.Fatal("failed to list access log tables", "error", err)
	}

	if !seedOnly {
		if rebuildOnly {
			if err := db.Transaction(func(tx *gorm.DB) error {
				return rebuildAggregates(tx, tables, shortCodes)
			}); err != nil {
				logger.Fatal("failed to rebuild aggregates", "error", err)
			}
			logger.Info("aggregate rebuild completed", "short_codes", len(shortCodes))
		} else {
			if err := cleanupAndRebuild(db, tables, shortCodes, now); err != nil {
				logger.Fatal("failed to cleanup demo data", "error", err)
			}
			logger.Info("cleanup completed", "short_codes", len(shortCodes))
		}
	}

	if cleanupOnly || rebuildOnly {
		return
	}

	rng := rand.New(rand.NewSource(seedValue))
	logs := generateTargetedLogs(shortCodes, targets, now, rng)
	if len(logs) == 0 {
		logger.Info("no logs generated")
		return
	}

	if err := persistSeedLogs(db, logs); err != nil {
		logger.Fatal("failed to persist exact demo logs", "error", err)
	}

	logger.Info("exact demo data seeded", "short_codes", len(shortCodes), "logs", len(logs), "seed", seedValue)
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

func parseTargets(raw string) (map[string]int, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]int{}, nil
	}

	targets := make(map[string]int)
	for _, item := range strings.Split(raw, ",") {
		part := strings.TrimSpace(item)
		if part == "" {
			continue
		}
		pieces := strings.SplitN(part, "=", 2)
		if len(pieces) != 2 {
			return nil, fmt.Errorf("invalid target item %q", part)
		}
		code := strings.TrimSpace(pieces[0])
		var count int
		if _, err := fmt.Sscanf(strings.TrimSpace(pieces[1]), "%d", &count); err != nil {
			return nil, fmt.Errorf("invalid target count in %q", part)
		}
		if count < 0 {
			return nil, fmt.Errorf("target count must be >= 0 for %s", code)
		}
		targets[code] = count
	}
	return targets, nil
}

func validateTargetCoverage(shortCodes []string, targets map[string]int) error {
	if len(shortCodes) != len(targets) {
		return fmt.Errorf("short-codes count %d does not match targets count %d", len(shortCodes), len(targets))
	}
	for _, code := range shortCodes {
		if _, ok := targets[code]; !ok {
			return fmt.Errorf("missing target for %s", code)
		}
	}
	for code := range targets {
		found := false
		for _, shortCode := range shortCodes {
			if shortCode == code {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("target provided for unknown short code %s", code)
		}
	}
	return nil
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
		return fmt.Errorf("missing short codes: %s", strings.Join(missing, ", "))
	}
	return nil
}

func listAccessLogTables(db *gorm.DB) ([]string, error) {
	var databaseName string
	if err := db.Raw("SELECT DATABASE()").Scan(&databaseName).Error; err != nil {
		return nil, err
	}

	var tables []string
	err := db.Raw(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = ?
		  AND table_name REGEXP '^access_logs_[0-9]{6}$'
		ORDER BY table_name
	`, databaseName).Scan(&tables).Error
	return tables, err
}

func cleanupAndRebuild(db *gorm.DB, tables []string, shortCodes []string, now time.Time) error {
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	prevTableName := fmt.Sprintf("access_logs_%s", currentMonthStart.AddDate(0, -1, 0).Format("200601"))
	currentTableName := fmt.Sprintf("access_logs_%s", now.Format("200601"))
	earlyMonthEnd := currentMonthStart.AddDate(0, 0, 9)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	tableSet := make(map[string]struct{}, len(tables))
	for _, table := range tables {
		tableSet[table] = struct{}{}
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if _, ok := tableSet[prevTableName]; ok {
			if err := tx.Exec(
				fmt.Sprintf("DELETE FROM `%s` WHERE short_code IN ?", prevTableName),
				shortCodes,
			).Error; err != nil {
				return err
			}
		}

		if _, ok := tableSet[currentTableName]; ok {
			if err := tx.Exec(
				fmt.Sprintf("DELETE FROM `%s` WHERE short_code IN ? AND (DATE(accessed_at) BETWEEN ? AND ? OR DATE(accessed_at) = ?)", currentTableName),
				shortCodes,
				currentMonthStart.Format("2006-01-02"),
				earlyMonthEnd.Format("2006-01-02"),
				today.Format("2006-01-02"),
			).Error; err != nil {
				return err
			}
		}

		if err := rebuildAggregates(tx, tables, shortCodes); err != nil {
			return err
		}
		return nil
	})
}

func rebuildAggregates(db *gorm.DB, tables []string, shortCodes []string) error {
	if err := db.Exec("DELETE FROM stats_daily WHERE short_code IN ?", shortCodes).Error; err != nil {
		return err
	}
	if err := db.Exec("DELETE FROM stats_region_daily WHERE short_code IN ?", shortCodes).Error; err != nil {
		return err
	}
	if err := db.Exec("DELETE FROM stats_device_daily WHERE short_code IN ?", shortCodes).Error; err != nil {
		return err
	}
	if err := db.Model(&model.Shortlink{}).Where("short_code IN ?", shortCodes).Update("click_count", 0).Error; err != nil {
		return err
	}

	totalClicks := make(map[string]int64)
	tableSet := make(map[string]struct{}, len(tables))
	for _, table := range tables {
		tableSet[table] = struct{}{}
	}

	sortedTables := make([]string, 0, len(tableSet))
	for table := range tableSet {
		sortedTables = append(sortedTables, table)
	}
	sort.Strings(sortedTables)

	for _, table := range sortedTables {
		if err := db.Exec(fmt.Sprintf(`
			INSERT INTO stats_daily (short_code, date, clicks)
			SELECT short_code, DATE(accessed_at) AS date, COUNT(*) AS clicks
			FROM %s
			WHERE short_code IN ?
			GROUP BY short_code, DATE(accessed_at)
		`, table), shortCodes).Error; err != nil {
			return err
		}

		if err := db.Exec(fmt.Sprintf(`
			INSERT INTO stats_region_daily (short_code, date, country, province, city, clicks)
			SELECT
				short_code,
				DATE(accessed_at) AS date,
				COALESCE(NULLIF(country, ''), 'Unknown') AS country,
				COALESCE(NULLIF(province, ''), 'Unknown') AS province,
				COALESCE(NULLIF(city, ''), 'Unknown') AS city,
				COUNT(*) AS clicks
			FROM %s
			WHERE short_code IN ?
			GROUP BY short_code, DATE(accessed_at), country, province, city
		`, table), shortCodes).Error; err != nil {
			return err
		}

		if err := db.Exec(fmt.Sprintf(`
			INSERT INTO stats_device_daily (short_code, date, device_type, os_version, browser, clicks)
			SELECT
				short_code,
				DATE(accessed_at) AS date,
				COALESCE(NULLIF(device_type, ''), 'Unknown') AS device_type,
				COALESCE(NULLIF(os_version, ''), 'Unknown') AS os_version,
				COALESCE(NULLIF(browser, ''), 'Unknown') AS browser,
				COUNT(*) AS clicks
			FROM %s
			WHERE short_code IN ?
			GROUP BY short_code, DATE(accessed_at), device_type, os_version, browser
		`, table), shortCodes).Error; err != nil {
			return err
		}

		var rows []countRow
		if err := db.Raw(fmt.Sprintf(`
			SELECT short_code, COUNT(*) AS clicks
			FROM %s
			WHERE short_code IN ?
			GROUP BY short_code
		`, table), shortCodes).Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			totalClicks[row.ShortCode] += row.Clicks
		}
	}

	for _, code := range shortCodes {
		if err := db.Model(&model.Shortlink{}).
			Where("short_code = ?", code).
			Update("click_count", totalClicks[code]).Error; err != nil {
			return err
		}
	}
	return nil
}

func generateTargetedLogs(shortCodes []string, targets map[string]int, now time.Time, rng *rand.Rand) []model.AccessLog {
	sourceProfiles := buildSourceProfiles()
	deviceProfiles := buildDeviceProfiles()
	locationProfiles := buildLocationProfiles()
	stagePlans := buildStagePlans(now)
	stageWeights := []int{900, 1100, 600, 300}

	logs := make([]model.AccessLog, 0)
	for _, code := range shortCodes {
		totalTarget := targets[code]
		stageTotals := distributeByWeights(totalTarget, stageWeights)
		for index, stage := range stagePlans {
			dailyCounts := distributeDates(stageTotals[index], stage.Weights)
			for _, dayCount := range dailyCounts {
				for i := 0; i < dayCount.Weight; i++ {
					source := pickWeighted(rng, sourceProfiles)
					device := pickWeighted(rng, deviceProfiles)
					location := pickWeighted(rng, locationProfiles)
					logs = append(logs, model.AccessLog{
						ShortCode:  code,
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
						AccessedAt: randomTimeOnDate(dayCount.Date, rng),
					})
				}
			}
			_ = stage
		}
	}
	return logs
}

func buildStagePlans(now time.Time) []stagePlan {
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	prevMonthStart := currentMonthStart.AddDate(0, -1, 0)

	return []stagePlan{
		{
			Name: "previous-month",
			Weights: []dateWeight{
				{Date: prevMonthStart.AddDate(0, 0, 2), Weight: 1},
				{Date: prevMonthStart.AddDate(0, 0, 7), Weight: 2},
				{Date: prevMonthStart.AddDate(0, 0, 12), Weight: 3},
				{Date: prevMonthStart.AddDate(0, 0, 17), Weight: 3},
				{Date: prevMonthStart.AddDate(0, 0, 22), Weight: 2},
				{Date: prevMonthStart.AddDate(0, 0, 26), Weight: 1},
			},
		},
		{
			Name: "current-month-early",
			Weights: []dateWeight{
				{Date: currentMonthStart.AddDate(0, 0, 0), Weight: 1},
				{Date: currentMonthStart.AddDate(0, 0, 1), Weight: 1},
				{Date: currentMonthStart.AddDate(0, 0, 2), Weight: 2},
				{Date: currentMonthStart.AddDate(0, 0, 3), Weight: 2},
				{Date: currentMonthStart.AddDate(0, 0, 4), Weight: 3},
				{Date: currentMonthStart.AddDate(0, 0, 5), Weight: 4},
				{Date: currentMonthStart.AddDate(0, 0, 6), Weight: 4},
				{Date: currentMonthStart.AddDate(0, 0, 7), Weight: 3},
				{Date: currentMonthStart.AddDate(0, 0, 8), Weight: 2},
				{Date: currentMonthStart.AddDate(0, 0, 9), Weight: 1},
			},
		},
		{
			Name: "current-month-mid",
			Weights: []dateWeight{
				{Date: currentMonthStart.AddDate(0, 0, 11), Weight: 2},
				{Date: currentMonthStart.AddDate(0, 0, 12), Weight: 3},
				{Date: currentMonthStart.AddDate(0, 0, 13), Weight: 1},
			},
		},
		{
			Name: "today",
			Weights: []dateWeight{
				{Date: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), Weight: 1},
			},
		},
	}
}

func distributeDates(total int, weights []dateWeight) []dateWeight {
	values := make([]int, len(weights))
	intWeights := make([]int, len(weights))
	for index, item := range weights {
		intWeights[index] = item.Weight
	}
	values = distributeByWeights(total, intWeights)

	result := make([]dateWeight, len(weights))
	for index, item := range weights {
		result[index] = dateWeight{Date: item.Date, Weight: values[index]}
	}
	return result
}

func distributeByWeights(total int, weights []int) []int {
	result := make([]int, len(weights))
	if total <= 0 || len(weights) == 0 {
		return result
	}

	sumWeights := 0
	for _, weight := range weights {
		sumWeights += weight
	}
	if sumWeights == 0 {
		return result
	}

	type remainder struct {
		Index int
		Value float64
	}

	remainders := make([]remainder, 0, len(weights))
	allocated := 0
	for index, weight := range weights {
		exact := float64(total) * float64(weight) / float64(sumWeights)
		base := int(math.Floor(exact))
		result[index] = base
		allocated += base
		remainders = append(remainders, remainder{Index: index, Value: exact - float64(base)})
	}

	sort.SliceStable(remainders, func(i, j int) bool {
		return remainders[i].Value > remainders[j].Value
	})

	for i := 0; i < total-allocated; i++ {
		result[remainders[i%len(remainders)].Index]++
	}
	return result
}

func buildSourceProfiles() []weightedValue[sourceProfile] {
	return []weightedValue[sourceProfile]{
		{Value: sourceProfile{Channel: "direct"}, Weight: 5},
		{Value: sourceProfile{Channel: "weixin.qq.com"}, Weight: 15},
		{Value: sourceProfile{Channel: "weibo.com"}, Weight: 13},
		{Value: sourceProfile{Channel: "zhihu.com"}, Weight: 12},
		{Value: sourceProfile{Channel: "xiaohongshu.com"}, Weight: 10},
		{Value: sourceProfile{Channel: "douyin.com"}, Weight: 10},
		{Value: sourceProfile{Channel: "bilibili.com"}, Weight: 8},
		{Value: sourceProfile{Channel: "qq.com"}, Weight: 8},
		{Value: sourceProfile{Channel: "xiaoheihe.cn"}, Weight: 7},
		{Value: sourceProfile{Channel: "baidu.com"}, Weight: 7},
		{Value: sourceProfile{Channel: "tieba.baidu.com"}, Weight: 5},
		{Value: sourceProfile{Channel: "google.com"}, Weight: 8},
		{Value: sourceProfile{Channel: "bing.com"}, Weight: 4},
		{Value: sourceProfile{Channel: "reddit.com"}, Weight: 3},
		{Value: sourceProfile{Channel: "x.com"}, Weight: 3},
		{Value: sourceProfile{Channel: "facebook.com"}, Weight: 2},
		{Value: sourceProfile{Channel: "instagram.com"}, Weight: 2},
		{Value: sourceProfile{Channel: "youtube.com"}, Weight: 2},
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
			Weight: 22,
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
		{Value: locationProfile{IP: "120.24.64.170", Country: "China", Province: "Guangdong", City: "Shenzhen"}, Weight: 13},
		{Value: locationProfile{IP: "183.60.92.44", Country: "China", Province: "Guangdong", City: "Guangzhou"}, Weight: 9},
		{Value: locationProfile{IP: "123.125.114.61", Country: "China", Province: "Beijing", City: "Beijing"}, Weight: 11},
		{Value: locationProfile{IP: "139.196.12.88", Country: "China", Province: "Shanghai", City: "Shanghai"}, Weight: 9},
		{Value: locationProfile{IP: "223.5.5.5", Country: "China", Province: "Zhejiang", City: "Hangzhou"}, Weight: 7},
		{Value: locationProfile{IP: "221.228.32.13", Country: "China", Province: "Jiangsu", City: "Nanjing"}, Weight: 6},
		{Value: locationProfile{IP: "36.112.14.119", Country: "China", Province: "Sichuan", City: "Chengdu"}, Weight: 5},
		{Value: locationProfile{IP: "111.13.101.51", Country: "China", Province: "Hubei", City: "Wuhan"}, Weight: 4},
		{Value: locationProfile{IP: "27.38.5.15", Country: "China", Province: "Tianjin", City: "Tianjin"}, Weight: 3},
		{Value: locationProfile{IP: "91.198.174.32", Country: "Netherlands", Province: "Noord-Holland", City: "Amsterdam"}, Weight: 4},
		{Value: locationProfile{IP: "94.140.14.119", Country: "Cyprus", Province: "Limassol", City: "Limassol"}, Weight: 4},
		{Value: locationProfile{IP: "208.67.222.164", Country: "United States", Province: "California", City: "San Jose"}, Weight: 6},
		{Value: locationProfile{IP: "9.9.9.6", Country: "United States", Province: "California", City: "Berkeley"}, Weight: 5},
		{Value: locationProfile{IP: "8.8.8.8", Country: "United States", Province: "California", City: "Mountain View"}, Weight: 5},
		{Value: locationProfile{IP: "1.1.1.1", Country: "Australia", Province: "New South Wales", City: "Sydney"}, Weight: 4},
		{Value: locationProfile{IP: "216.58.200.14", Country: "Japan", Province: "Tokyo", City: "Tokyo"}, Weight: 3},
		{Value: locationProfile{IP: "149.112.112.112", Country: "Singapore", Province: "Central Singapore", City: "Singapore"}, Weight: 3},
		{Value: locationProfile{IP: "80.80.80.80", Country: "United Kingdom", Province: "England", City: "London"}, Weight: 2},
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

	tableNames := sortedTableKeys(logsByTable)
	for _, tableName := range tableNames {
		if err := ensureAccessLogTable(db, tableName); err != nil {
			return err
		}
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, tableName := range tableNames {
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

func sortedTableKeys(values map[string][]model.AccessLog) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
