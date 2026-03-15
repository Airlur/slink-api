package bootstrap

import (
	"fmt"
	"strings"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

func EnsureStatsSchema(db *gorm.DB) error {
	logTables, err := listAccessLogTables(db)
	if err != nil {
		return err
	}

	for _, table := range append([]string{"access_logs_template"}, logTables...) {
		if err := ensureAccessLogTableSchema(db, table); err != nil {
			return err
		}
	}

	if err := ensureStatsRegionDailySchema(db); err != nil {
		return err
	}

	return nil
}

func ensureAccessLogTableSchema(db *gorm.DB, table string) error {
	statements := []string{
		fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `country` VARCHAR(80) NULL DEFAULT NULL AFTER `browser`", table),
		fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `channel` VARCHAR(100) NULL DEFAULT NULL", table),
		fmt.Sprintf("ALTER TABLE `%s` ADD INDEX `idx_country_province_city` (`country`, `province`, `city`)", table),
	}

	for _, statement := range statements {
		if err := execSchemaStatement(db, statement); err != nil {
			return err
		}
	}
	return nil
}

func ensureStatsRegionDailySchema(db *gorm.DB) error {
	statements := []string{
		"ALTER TABLE `stats_region_daily` ADD COLUMN `country` VARCHAR(80) NOT NULL DEFAULT 'Unknown' AFTER `date`",
		"ALTER TABLE `stats_region_daily` DROP INDEX `uk_code_date_province_city`",
		"ALTER TABLE `stats_region_daily` ADD UNIQUE KEY `uk_code_date_country_province_city` (`short_code`, `date`, `country`, `province`, `city`)",
		"ALTER TABLE `stats_region_daily` ADD INDEX `idx_country_province_city` (`country`, `province`, `city`)",
	}

	for _, statement := range statements {
		if err := execSchemaStatement(db, statement); err != nil {
			return err
		}
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
		WHERE table_schema = ? AND table_name LIKE 'access_logs\\_%' ESCAPE '\\'
		ORDER BY table_name
	`, databaseName).Scan(&tables).Error
	return tables, err
}

func execSchemaStatement(db *gorm.DB, statement string) error {
	err := db.Exec(statement).Error
	if err == nil || isIgnorableSchemaError(err) {
		return nil
	}
	return err
}

func isIgnorableSchemaError(err error) bool {
	mysqlErr, ok := err.(*mysqlDriver.MySQLError)
	if !ok {
		return false
	}

	switch mysqlErr.Number {
	case 1060, 1061, 1091:
		return true
	}

	message := strings.ToLower(mysqlErr.Message)
	return strings.Contains(message, "duplicate column") || strings.Contains(message, "duplicate key") || strings.Contains(message, "check that column/key exists")
}
