package bootstrap

import (
	"short-link/internal/pkg/config"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Bootstrap struct {
	engine *gin.Engine
	db     *gorm.DB
}

func New() *Bootstrap {
	return &Bootstrap{}
}

func (b *Bootstrap) Initialize() error {
	// 1. 初始化配置
	config.InitConfig()

	// 2. 初始化日志 - 使用配置文件中的路径
	logger.InitLogger(&config.GlobalConfig.Logger)

	// 3. 初始化校验器翻译器
	if err := validator.InitTranslator("zh"); err != nil {
		return err
	}

	// 4. 初始化数据库
	db, err := initDB()
	if err != nil {
		return err
	}
	b.db = db

	// 5. 初始化Redis
	if err := InitRedis(); err != nil {
		return err
	}

	// 6. 初始化路由
	b.engine = initRouter(db)

	return nil
}

func (b *Bootstrap) Run() error {
	return b.engine.Run(config.GlobalConfig.Server.Port)
}
