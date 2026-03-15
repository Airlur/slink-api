package main

import (
	"fmt"
	"os"
	"time"

	"slink-api/internal/bootstrap"
	"slink-api/internal/pkg/config"
	"slink-api/internal/pkg/geoip"
	"slink-api/internal/pkg/logger"

	"gorm.io/gorm"
)

type accessLogBackfillRow struct {
	ID uint64 `gorm:"column:id"`
	IP string `gorm:"column:ip"`
}

func main() {
	config.InitConfig()
	logger.InitLogger(&config.GlobalConfig.Logger)
	defer logger.Log.Sync()

	geoip.Init("./data/IP2LOCATION-LITE-DB3.IPV6.BIN")

	db, err := bootstrap.OpenDatabase()
	if err != nil {
		logger.Fatal("failed to open database", "error", err)
	}

	tables, err := listAccessLogTables(db)
	if err != nil {
		logger.Fatal("failed to list access log tables", "error", err)
	}
	if len(tables) == 0 {
		logger.Info("no access log tables found")
		return
	}

	logger.Info("starting country backfill", "tables", len(tables))
	for _, table := range tables {
		if err := backfillTableCountry(db, table); err != nil {
			logger.Fatal("failed to backfill table", "table", table, "error", err)
		}
	}

	if err := rebuildRegionDailyStats(db, tables); err != nil {
		logger.Fatal("failed to rebuild stats_region_daily", "error", err)
	}

	logger.Info("country backfill completed")
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
		  AND table_name LIKE 'access_logs\\_%' ESCAPE '\\'
		  AND table_name <> 'access_logs_template'
		ORDER BY table_name
	`, databaseName).Scan(&tables).Error
	return tables, err
}

func backfillTableCountry(db *gorm.DB, table string) error {
	const batchSize = 500
	var lastID uint64
	var totalProcessed int
	var totalResolved int

	for {
		var rows []accessLogBackfillRow
		err := db.Table(table).
			Select("id, ip").
			Where("(country IS NULL OR country = '' OR country = 'Unknown') AND id > ?", lastID).
			Order("id ASC").
			Limit(batchSize).
			Scan(&rows).Error
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			country, province, city := geoip.Parse(row.IP)
			if err := db.Table(table).
				Where("id = ?", row.ID).
				Updates(map[string]any{
					"country":  country,
					"province": province,
					"city":     city,
				}).Error; err != nil {
				return err
			}
			totalProcessed++
			if country != "Unknown" || province != "Unknown" || city != "Unknown" {
				totalResolved++
			}
		}

		lastID = rows[len(rows)-1].ID
		logger.Info("backfilled access log batch", "table", table, "processed", totalProcessed, "resolved", totalResolved)
	}

	logger.Info("finished access log table backfill", "table", table, "processed", totalProcessed, "resolved", totalResolved)
	return nil
}

func rebuildRegionDailyStats(db *gorm.DB, tables []string) error {
	logger.Info("rebuilding stats_region_daily")
	if err := db.Exec("TRUNCATE TABLE stats_region_daily").Error; err != nil {
		return err
	}

	for _, table := range tables {
		statement := fmt.Sprintf(`
			INSERT INTO stats_region_daily (short_code, date, country, province, city, clicks)
			SELECT
				short_code,
				DATE(accessed_at) AS date,
				COALESCE(NULLIF(country, ''), 'Unknown') AS country,
				COALESCE(NULLIF(province, ''), 'Unknown') AS province,
				COALESCE(NULLIF(city, ''), 'Unknown') AS city,
				COUNT(*) AS clicks
			FROM %s
			GROUP BY short_code, DATE(accessed_at), country, province, city
		`, table)
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
		logger.Info("reloaded regional aggregates", "table", table)
	}
	return nil
}

func init() {
	if _, err := os.Stat("./data/IP2LOCATION-LITE-DB3.IPV6.BIN"); err != nil {
		fmt.Fprintf(os.Stderr, "missing GeoIP database: %v\n", err)
		os.Exit(1)
	}
	_ = time.Local
}
