package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	global *zap.Logger
	sugar  *zap.SugaredLogger
)

func init() {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderCfg),
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)

	global = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	sugar = global.Sugar()
}

func Init(l *zap.Logger) {
	if l != nil {
		global = l
		sugar = l.Sugar()
	}
}

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

func Debugf(template string, args ...interface{}) {
	sugar.Debugf(template, args...)
}

func Infof(template string, args ...interface{}) {
	sugar.Infof(template, args...)
}

func Warnf(template string, args ...interface{}) {
	sugar.Warnf(template, args...)
}

func Errorf(template string, args ...interface{}) {
	sugar.Errorf(template, args...)
}

func Fatalf(template string, args ...interface{}) {
	sugar.Fatalf(template, args...)
}

func Sync() error {
	return global.Sync()
}
