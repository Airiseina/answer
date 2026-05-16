package main

import (
	"answer_pkg/meter"
	"answer_pkg/tracer"
	"api_gateway/config"
	"api_gateway/middleware"
	"api_gateway/routes"
	"api_gateway/rpc"
	"context"
	"os"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/hertz-contrib/cors"
	hertzzap "github.com/hertz-contrib/logger/zap"
	hertztracing "github.com/hertz-contrib/obs-opentelemetry/tracing"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	hertzZapLogger := hertzzap.NewLogger(
		hertzzap.WithCoreWs(zapcore.AddSync(os.Stdout)),
		hertzzap.WithCoreLevel(zap.NewAtomicLevelAt(zap.DebugLevel)),
		hertzzap.WithZapOptions(
			zap.AddCaller(),
			zap.AddCallerSkip(4),
			zap.Fields(zap.String("service", "api_gateway")),
		),
	)
	hlog.SetLogger(hertzZapLogger)
	config.GetConfig()
	otelAddr := viper.GetString("otel.Addr")
	p := tracer.InitTracer("api_gateway", otelAddr)
	defer p.Shutdown(context.Background())
	meter.InitMeter("api_gateway")
	rpc.Connect()
	tracerOptions, cfg := hertztracing.NewServerTracer()
	h := server.New(server.WithHostPorts("0.0.0.0:1234"), tracerOptions)
	h.Use(hertztracing.ServerMiddleware(cfg))
	h.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH", "HEAD"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))
	middleware.JwtMiddleware()
	routes.Routes(h)
	h.Spin()
}
