package logger

import (
	"os"
	"path/filepath"
	"time"

	"short-link/internal/pkg/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Log *zap.Logger

// InitLogger 初始化日志
func InitLogger(cfg *config.LoggerConfig) {
	// 确保日志目录存在
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0744); err != nil {
		panic(err)
	}

	// 日志切割配置 - 现在完全由配置文件驱动
	hook := &lumberjack.Logger{
		Filename:   cfg.Path,       // 日志文件路径
		MaxSize:    cfg.MaxSize,    // 每个日志文件保存的最大尺寸 单位：M
		MaxBackups: cfg.MaxBackups, // 日志文件最多保存多少个备份
		MaxAge:     cfg.MaxAge,     // 文件最多保存多少天
		Compress:   cfg.Compress,   // 是否压缩
	}

	// 公共编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     timeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 控制台编码器配置
	consoleEncoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder, // 带颜色的日志级别
		EncodeTime:     timeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	})

	// 文件编码器配置
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

	// 设置日志级别
	atomicLevel := zap.NewAtomicLevel()
	atomicLevel.SetLevel(zap.InfoLevel)

	// 创建核心
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), atomicLevel),
		zapcore.NewCore(fileEncoder, zapcore.AddSync(hook), atomicLevel),
	)

	// 创建logger
	Log = zap.New(
		core,
		zap.AddCaller(),      // 添加调用者信息
		zap.AddCallerSkip(1), // 跳过封装函数
	)
}

// Debug 输出debug级别的日志
func Debug(msg string, fields ...interface{}) {
	Log.Debug(msg, parseFields(fields...)...)
}

// Info 输出info级别的日志
func Info(msg string, fields ...interface{}) {
	Log.Info(msg, parseFields(fields...)...)
}

// Warn 输出warn级别的日志
func Warn(msg string, fields ...interface{}) {
	Log.Warn(msg, parseFields(fields...)...)
}

// Error 输出error级别的日志
func Error(msg string, fields ...interface{}) {
	Log.Error(msg, parseFields(fields...)...)
}

// Fatal 输出fatal级别的日志
func Fatal(msg string, fields ...interface{}) {
	Log.Fatal(msg, parseFields(fields...)...)
}

// parseFields 解析字段
func parseFields(fields ...interface{}) []zap.Field {
	if len(fields)%2 != 0 {
		Log.Error("fields must be key-value pairs")
		return nil
	}

	var zapFields []zap.Field
	for i := 0; i < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			Log.Error("field key must be string")
			return nil
		}
		value := fields[i+1]
		switch v := value.(type) {
		case bool:
			zapFields = append(zapFields, zap.Bool(key, v))
		case int:
			zapFields = append(zapFields, zap.Int(key, v))
		case int8:
			zapFields = append(zapFields, zap.Int8(key, v))
		case int16:
			zapFields = append(zapFields, zap.Int16(key, v))
		case int32:
			zapFields = append(zapFields, zap.Int32(key, v))
		case int64:
			zapFields = append(zapFields, zap.Int64(key, v))
		case uint:
			zapFields = append(zapFields, zap.Uint(key, v))
		case uint8:
			zapFields = append(zapFields, zap.Uint8(key, v))
		case uint16:
			zapFields = append(zapFields, zap.Uint16(key, v))
		case uint32:
			zapFields = append(zapFields, zap.Uint32(key, v))
		case uint64:
			zapFields = append(zapFields, zap.Uint64(key, v))
		case float32:
			zapFields = append(zapFields, zap.Float32(key, v))
		case float64:
			zapFields = append(zapFields, zap.Float64(key, v))
		case string:
			zapFields = append(zapFields, zap.String(key, v))
		case time.Time:
			zapFields = append(zapFields, zap.Time(key, v))
		case time.Duration:
			zapFields = append(zapFields, zap.Duration(key, v))
		case error:
			zapFields = append(zapFields, zap.Error(v))
		default:
			zapFields = append(zapFields, zap.Any(key, v))
		}
	}
	return zapFields
}

// timeEncoder 自定义时间格式
func timeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}
