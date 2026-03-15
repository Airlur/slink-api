package bootstrap

import (
	"fmt"
	"slink-api/internal/model"
	"slink-api/internal/pkg/config"
	"slink-api/internal/pkg/logger"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gl "gorm.io/gorm/logger"
)

func initDB() (*gorm.DB, error) {
	conf := config.GlobalConfig.Database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		conf.Username,
		conf.Password,
		conf.Host,
		conf.Port,
		conf.DBName,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gl.Default.LogMode(gl.Info), // 开启所有 SQL 日志（INFO级别包含成功的查询）
	})
	if err != nil {
		return nil, err
	}

	// 自动迁移
	if err := db.AutoMigrate(&model.User{}); err != nil {
		return nil, err
	}
	if err := EnsureStatsSchema(db); err != nil {
		return nil, err
	}
	logger.Log.Info("Database connection successful")
	return db, nil
}

func OpenDatabase() (*gorm.DB, error) {
	return initDB()
}
