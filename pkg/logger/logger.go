package logger

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func init() {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)
	writeSyncer := zapcore.AddSync(os.Stdout)
	core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)
	logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
}

func Info(message string, fields ...zap.Field) {
	logger.Info(message, fields...)
}

func Debug(message string, fields ...zap.Field) {
	logger.Debug(message, fields...)
}

func Error(message string, fields ...zap.Field) {
	logger.Error(message, fields...)
}

func Warn(message string, fields ...zap.Field) {
	logger.Warn(message, fields...)
}

func Fatal(message string, fields ...zap.Field) {
	logger.Fatal(message, fields...)
}

func CtxInfo(ctx context.Context, message string, fields ...zap.Field) {
	// 【未来占位】在这里从 ctx 中提取 TraceID 并塞入 fields
	// traceID := ctx.Value("trace_id")
	// fields = append(fields, zap.String("trace_id", traceID.(string)))
	logger.Info(message, fields...)
}

func CtxError(ctx context.Context, message string, fields ...zap.Field) {
	logger.Error(message, fields...)
}
