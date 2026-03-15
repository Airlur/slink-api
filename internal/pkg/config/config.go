package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	// 基础配置
	App      AppConfig      `mapstructure:"app"`
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Logger   LoggerConfig   `mapstructure:"logger"`
	Security SecurityConfig `mapstructure:"security"`
	Cache    CacheConfig    `mapstructure:"cache"`
	Captcha  CaptchaConfig  `mapstructure:"captcha"`
	Email    EmailConfig    `mapstructure:"email"`
	// 业务配置
	User      UserConfig      `mapstructure:"user"`
	Shortlink ShortlinkConfig `mapstructure:"shortlink"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Lifecycle LifecycleConfig `mapstructure:"lifecycle"`
}

type AppConfig struct {
	Scheme string `mapstructure:"scheme"`
	Domain string `mapstructure:"domain"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type JWTConfig struct {
	Secret              string `mapstructure:"secret"`
	Issuer              string `mapstructure:"issuer"`
	AccessExpireMinutes int    `mapstructure:"access_expire_minutes"`
	RefreshExpireHours  int    `mapstructure:"refresh_expire_hours"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type LoggerConfig struct {
	Path       string `mapstructure:"path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

type SecurityConfig struct {
	AccountLock     AccountLockConfig `mapstructure:"account_lock"`
	OneTimeTokenTTL TokenTTLConfig    `mapstructure:"one_time_token_ttl_minutes"`
}

type AccountLockConfig struct {
	MaxLoginFailures int `mapstructure:"max_login_failures"`
	LockoutMinutes   int `mapstructure:"lockout_minutes"`
}

type TokenTTLConfig struct {
	PasswordReset   int `mapstructure:"password_reset"`
	AccountRecovery int `mapstructure:"account_recovery"`
}

type CacheConfig struct {
	DefaultExpirationMinutes   int `mapstructure:"default_expiration_minutes"`
	RandomJitterSeconds        int `mapstructure:"random_jitter_seconds"`
	NullCacheExpirationSeconds int `mapstructure:"null_cache_expiration_seconds"`
}

type CaptchaConfig struct {
	ExpirationMinutes int `mapstructure:"expiration_minutes"`
	CooldownSeconds   int `mapstructure:"cooldown_seconds"`
	LimitPerMinute    int `mapstructure:"limit_per_minute"`
	LimitPerHour      int `mapstructure:"limit_per_hour"`
	LimitPerDay       int `mapstructure:"limit_per_day"`
}

type EmailConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	SenderName string `mapstructure:"sender_name"`
}

// ======= 业务模块配置 =======

// UserConfig 用户模块业务规则配置
type UserConfig struct {
	RecoveryGracePeriodDays int `mapstructure:"recovery_grace_period_days"`
}

// ShortlinkConfig 短链接模块业务规则配置
type ShortlinkConfig struct {
	ReservedWords []string `mapstructure:"reserved_words"`
}

// RateLimitConfig 短链接创建限流规则配置
type RateLimitConfig struct {
	CreatePerDayUser       int `mapstructure:"create_per_day_user"`
	CreatePerDayGuest      int `mapstructure:"create_per_day_guest"`
	CustomPerDayUser       int `mapstructure:"custom_per_day_user"`
	AccessPerSecondIP      int `mapstructure:"access_per_second_ip"`
	AccessBurstIP          int `mapstructure:"access_burst_ip"`
	BlockIPWindowSeconds   int `mapstructure:"block_ip_window_seconds"`
	BlockIPMaxRequests     int `mapstructure:"block_ip_max_requests"`
	BlockIPDurationMinutes int `mapstructure:"block_ip_duration_minutes"`
	GlobalQPS              int `mapstructure:"global_qps"`
	GlobalBurst            int `mapstructure:"global_burst"`
	AccountPerMinute       int `mapstructure:"account_per_minute"`
	DevicePerHour          int `mapstructure:"device_per_hour"`
}

// LifecycleConfig 统计数据生命周期管理配置
type LifecycleConfig struct {
	LogRetentionDays int  `mapstructure:"log_retention_days"`
	EnableLogCleanup bool `mapstructure:"enable_log_cleanup"`
}

var GlobalConfig Config

// InitConfig 初始化配置
func InitConfig() {
	viper.SetConfigName("config")  // 配置文件名称(无扩展名)
	viper.SetConfigType("yaml")    // 如果配置文件的名称中没有扩展名，则需要配置此项
	viper.AddConfigPath("configs") // 查找配置文件所在的路径

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}

	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}
}
