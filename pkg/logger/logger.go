package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 全局 logger 实例，整个项目通过 logger.Info() 等方法直接调用
var global *zap.Logger

// Init 根据 debug 模式初始化全局 logger
// debug=true  → 彩色控制台输出，包含文件名和行号，适合开发
// debug=false → JSON 格式输出，适合生产环境日志收集
func Init(debug bool) {
	var core zapcore.Core

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "time"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder // 人类可读时间格式

	if debug {
		// 开发模式：彩色控制台
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderCfg),
			zapcore.AddSync(os.Stdout),
			zapcore.DebugLevel,
		)
	} else {
		// 生产模式：JSON 格式
		encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(os.Stdout),
			zapcore.InfoLevel,
		)
	}

	global = zap.New(core, zap.AddCaller()) // AddCaller 记录调用文件和行号
}

// 以下是对外暴露的快捷方法，项目中直接 logger.Info("xxx") 使用

func Debug(msg string, fields ...zap.Field) {
	global.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	global.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	global.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	global.Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	global.Fatal(msg, fields...)
}

// With 返回携带固定字段的子 logger，适合在某个模块内统一加上模块名
// 用法: log := logger.With(zap.String("module", "analyzer"))
func With(fields ...zap.Field) *zap.Logger {
	return global.With(fields...)
}
