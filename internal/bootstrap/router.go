package bootstrap

import (
	"context"
	"runtime"
	"time"

	"short-link/internal/api/middleware"
	v1 "short-link/internal/api/v1"

	"short-link/internal/model"
	"short-link/internal/pkg/config"
	"short-link/internal/pkg/cron"
	"short-link/internal/pkg/email"
	"short-link/internal/pkg/eventbus"
	"short-link/internal/pkg/geoip"
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/uaparser"
	"short-link/internal/repository"
	"short-link/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	// 导入 swagger
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "short-link/docs" // 导入生成的 docs 包
)

func initRouter(db *gorm.DB) *gin.Engine {
	// 初始化事件总线，用于服务间的异步解耦
	eventbus.InitEventBus()
	// 初始化 User-Agent 解析器
	uaparser.Init("./data/regexes.yaml")
	// 初始化 IP 地理位置数据库
	geoip.Init("./data/IP2LOCATION-LITE-DB3.IPV6.BIN")


	// =============== 开始依赖注入 ===============
	// 验证码模块依赖注入
	mockSmsClient := &service.MockSmsClient{}
	emailClient := email.NewSmtpClient(&config.GlobalConfig.Email)
	
	// 用户模块
	userRepo := repository.NewUserRepository(db)
	captchaService := service.NewCaptchaService(userRepo, mockSmsClient, emailClient)
	userService := service.NewUserService(db, userRepo, captchaService)
	userHandler := v1.NewUserHandler(userService)
	captchaHandler := v1.NewCaptchaHandler(captchaService)

	// 短链接模块
	slRepo := repository.NewShortlinkRepository(db)
	slService := service.NewShortlinkService(db, slRepo)
	slHandler := v1.NewShortlinkHandler(slService)

	// 分享模块
	shareRepo := repository.NewShareRepository(db)
	shareService := service.NewShareService(db, shareRepo, slRepo) // 注意：ShareService依赖slRepo
	shareHandler := v1.NewShareHandler(shareService)

	// 标签模块
	tagRepo := repository.NewTagRepository(db)
	tagService := service.NewTagService(db, tagRepo, slRepo)
	tagHandler := v1.NewTagHandler(tagService)

	// 访问数据日志统计
	statsRepo := repository.NewStatsRepository(db)
	logRepo := repository.NewLogRepository(db)
	// logService := service.NewLogService(logRepo, statsRepo)
	logService := service.NewLogService()
	statsService := service.NewStatsService(slRepo, statsRepo, logRepo)
	statsHandler := v1.NewStatsHandler(statsService)
	
	batchWriterService := service.NewBatchWriterService(db, logRepo, statsRepo)
	maintenanceService := service.NewMaintenanceService(db)
	// =============== 结束依赖注入 ===============

	// =============== 启动后台常驻任务 ===============
	// 启动一个goroutine，作为后台消费者，持续处理访问日志
	// go startLogConsumer(logService)

	// a. 启动一个 Worker Pool 而不是单个 Worker
	// Worker 的数量可以根据CPU核心数来设定，也可以做成可配置
	numWorkers := runtime.NumCPU() * 2 // 通常设置为CPU核心数的2-4倍
	logger.Info("启动访问日志消费者工作池", "worker数量", numWorkers)
	for i := 0; i < numWorkers; i++ {
		// 每个Worker都是一个独立的goroutine，它们会共同消费channel中的事件
		go startLogConsumer(logService)
	}

	// b. 启动定时任务调度器
	cron.InitCron(batchWriterService,maintenanceService)

	// =============== 路由注册 ===============
	// 设置路由
	r := gin.New()

	// 全局限流
	r.Use(middleware.RateLimitGlobal())
	// 防护恶意IP攻击
	r.Use(middleware.RateLimitIPBlock())

	// r.Use(gin.Logger(), gin.Recovery()) // gin 自带的日志中间件
	// 使用自定义的日志和恢复中间件
	r.Use(middleware.Logger(), middleware.Recovery())

	// =============== Swagger 文档路由 ===============
	// 访问地址: http://localhost:8080/swagger/index.html
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	
	// =================  公开路由 (无需认证) =================
	// 短链接跳转接口，放在根路径无前缀路由。使用令牌桶中间件实现访问限流
	r.GET("/:shortCode", middleware.RateLimitLinkAccess(), slHandler.Redirect)

	// API 分组
	v1 := r.Group("/api/v1")
	{	
		// 1. 用户模块路由
		{
			// 公开接口
			v1.POST("/register", middleware.RateLimitDevice(5, time.Hour), userHandler.Register)
			v1.POST("/login", userHandler.Login)
			v1.GET("/users/check", userHandler.CheckExistence)
			v1.POST("/users/token/refresh", userHandler.RefreshToken)
			// 用户账号找回密码流程
			v1.POST("/users/password/forgot", middleware.RateLimitDevice(5, time.Hour), userHandler.ForgotPassword)
			v1.POST("/users/password/verify", userHandler.VerifyPasswordResetCaptcha)
			v1.POST("/users/password/reset", userHandler.ResetPassword)
			// 用户账号自助恢复流程
			v1.POST("/users/recovery/request", middleware.RateLimitDevice(5, time.Hour), userHandler.RequestRecovery)
			v1.POST("/users/recovery/verify", userHandler.VerifyRecoveryCaptcha)
			v1.POST("/users/recovery/execute", userHandler.ExecuteRecovery)
			
			// 需要登录认证的接口
			usersAuth := v1.Group("/users", middleware.Auth())
			{
				// 普通认证用户即可访问的接口
				usersAuth.POST("/logout", userHandler.Logout)
				usersAuth.PUT("/:id", userHandler.Update)
				usersAuth.PUT("/:id/reset", userHandler.UpdatePassword)
				usersAuth.DELETE("/:id", userHandler.Delete)
				usersAuth.GET("/:id", userHandler.Get)

				// 需要管理员权限才能访问的接口
				// 在 Auth() 之后，再走 RoleAuth() 进行角色授权
				// usersAuth.GET("", middleware.RoleAuth(model.UserRoleAdmin), userHandler.List)
			}
		}

		// 2. 验证码模块路由
		captchaGroup := v1.Group("/captcha")
		{
			captchaGroup.POST("/send", middleware.AuthOptional(), captchaHandler.SendCaptcha)
			captchaGroup.POST("/verify", captchaHandler.VerifyCaptcha)
		}

		// --- 3. 管理员专属路由组 ---
		// 这个组的所有路由，都必须先通过登录认证，再通过管理员角色认证
		adminGroup := v1.Group("/admin", middleware.Auth(), middleware.RoleAuth(model.UserRoleAdmin))
		{
			// 管理员用户管理接口
			adminUsers := adminGroup.Group("/users")
			{
				adminUsers.GET("", userHandler.List)
				adminUsers.PUT("/:id/status", userHandler.UpdateUserStatus)
				adminUsers.DELETE("/:id/session", userHandler.ForceLogout)
			}
			
			// 未来所有其他管理员接口都应放在 adminGroup 之下
			adminStats := adminGroup.Group("/stats")
			{
				adminStats.GET("/global", statsHandler.GetGlobalStats)
			}

			// adminShortlinks := adminGroup.Group("/shortlinks") {}
		}

		// 4. 短链接模块路由
		{
			// 创建一个 /shortlinks 基础路由组
			slGroup := v1.Group("/shortlinks")

			// 应用 AuthOptional 和 RateLimitLinkCreate 中间件，可为游客或已登录用户创建短链接，同时进行限流
			slGroup.POST("", middleware.AuthOptional(), middleware.RateLimitLinkCreate(), slHandler.Create)

			// 需要登录认证的接口
			authSlRoutes := slGroup.Use(middleware.Auth())
			{
				// 列表接口
				authSlRoutes.GET("/my", slHandler.ListMy) 

				// 核心管理接口
				authSlRoutes.GET("/:short_code", slHandler.GetDetail)
				authSlRoutes.PUT("/:short_code", slHandler.Update)
				authSlRoutes.DELETE("/:short_code", slHandler.Delete)
				authSlRoutes.PUT("/:short_code/status", slHandler.UpdateStatus)
				authSlRoutes.PUT("/:short_code/expiration", slHandler.ExtendExpiration)

				// 统计子资源接口
				authSlRoutes.GET("/:short_code/stats/overview", statsHandler.GetOverview)
				authSlRoutes.GET("/:short_code/stats/trend", statsHandler.GetTrend)
				authSlRoutes.GET("/:short_code/stats/provinces", statsHandler.GetProvinces)
				authSlRoutes.GET("/:short_code/stats/cities", statsHandler.GetCities)
				authSlRoutes.GET("/:short_code/stats/devices", statsHandler.GetDevices)
				authSlRoutes.GET("/:short_code/stats/sources", statsHandler.GetSources)
				authSlRoutes.GET("/:short_code/logs", statsHandler.GetLogs)
			}
		}

		// 6. 分享信息管理路由
		sharesAuth := v1.Group("/shares", middleware.Auth())
		{
			sharesAuth.GET("/:short_code", shareHandler.Get)
			sharesAuth.PUT("/:short_code", shareHandler.Upsert)
		}

		// 6. 标签管理路由
		tagsAuth := v1.Group("/tags", middleware.Auth())
		{
			// 这个路由不带参数，是获取当前用户的所有标签
			tagsAuth.GET("", tagHandler.List) 
			tagsAuth.POST("/:short_code", tagHandler.Add)
			tagsAuth.DELETE("/:short_code", tagHandler.Remove)
		}
	}

	return r
}

// startLogConsumer 是后台消费者goroutine
func startLogConsumer(logSvc service.LogService) {
	ch := eventbus.SubscribeAccessLog()
	logger.Info("启动访问日志消费者...")
	for event := range ch {
		// 为每个事件创建一个带超时的context
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // 增加超时以防处理卡住
		// ProcessLog 内部会自行处理和记录错误
		logSvc.ProcessLog(ctx, event)
		
		cancel()
	}
}