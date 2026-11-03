package main

import (
	"context"
	"os"
	"time"

	"github.com/Airiseina/answer/api_gateway/config"
	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/middleware/ratelimit"
	"github.com/Airiseina/answer/api_gateway/routes"
	"github.com/Airiseina/answer/api_gateway/rpc"
	"github.com/Airiseina/answer/pkg/observability/meter"
	"github.com/Airiseina/answer/pkg/observability/tracer"
	"github.com/Airiseina/answer/pkg/storage"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/hertz-contrib/cors"
	hertzzap "github.com/hertz-contrib/logger/zap"
	hertztracing "github.com/hertz-contrib/obs-opentelemetry/tracing"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	hertzZapLogger := hertzzap.NewLogger(
		hertzzap.WithCoreWs(zapcore.AddSync(os.Stdout)),
		hertzzap.WithCoreLevel(zap.NewAtomicLevelAt(zap.DebugLevel)),
		hertzzap.WithZapOptions(
			zap.AddCaller(),
			zap.AddCallerSkip(3),
			zap.Fields(zap.String("service", "api_gateway")),
		),
	)
	hlog.SetLogger(hertzZapLogger)
	config.GetConfig()
	v := config.V
	storage.Init(v)
	otelAddr := v.GetString("otel.Addr")
	p := tracer.InitTracer("api_gateway", otelAddr)
	defer func() { _ = p.Shutdown(context.Background()) }()
	meter.InitMeter("api_gateway")
	rpc.Connect(v)
	tracerOptions, cfg := hertztracing.NewServerTracer()
	h := server.New(server.WithHostPorts("0.0.0.0:1234"), server.WithMaxRequestBodySize(50*1024*1024), tracerOptions)

	h.Use(hertztracing.ServerMiddleware(cfg))
	h.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH", "HEAD"},
		// 开放平台签名请求头需显式放行，否则浏览器跨域调用 /open/v1 会被预检拦截。
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-App-Id", "X-Timestamp", "X-Nonce", "X-Signature"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))
	middleware.JwtMiddleware()
	middleware.AdminJwtMiddleware()
	ratelimit.Default.StartCleanup(5 * time.Minute)
	routes.Routes(h)
	h.Spin()
}
