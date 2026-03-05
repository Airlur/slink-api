package main

import (
	"slink-api/internal/bootstrap"
	"slink-api/internal/pkg/logger"
)

// @title           短链接服务 API
// @version         1.0
// @description     短链接生成与管理服务的 API 文档
// @termsOfService  http://swagger.io/terms/
// @contact.name   API Support
// @contact.url    http://www.example.com/support
// @contact.email  support@example.com
// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html
// @host      localhost:8080
// @BasePath  /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 输入格式: Bearer {token}
func main() {
	// 创建并初始化 server
	server := bootstrap.New()
	if err := server.Initialize(); err != nil {
		// 如果初始化失败，此时日志系统可能已经启动
		// 使用我们自己的 logger 记录致命错误，以保持日志格式统一
		logger.Fatal("Failed to initialize server: %v", err)
	}

	// 延迟同步日志，确保所有缓冲的日志都被写入
	defer logger.Log.Sync()

	// 运行 server
	if err := server.Run(); err != nil {
		logger.Fatal("Failed to run server: %v", err)
	}
}
