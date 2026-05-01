package main

import (
	"api_gateway/config"
	"api_gateway/middleware"
	"api_gateway/routes"
	"api_gateway/rpc"
	"os"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	hertzzap "github.com/hertz-contrib/logger/zap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	hertzZapLogger := hertzzap.NewLogger(
		hertzzap.WithCoreWs(zapcore.AddSync(os.Stdout)),
		hertzzap.WithCoreLevel(zap.NewAtomicLevelAt(zap.DebugLevel)),
		hertzzap.WithZapOptions(
			zap.AddCaller(), // 输出是在哪行代码打的日志
			zap.AddCallerSkip(1),
			zap.Fields(zap.String("service", "api_gateway")),
		),
	)
	hlog.SetLogger(hertzZapLogger)
	config.GetConfig()
	rpc.Connect()
	middleware.JwtMiddleware()
	//mq.KafkaInit()
	//stoClient := handle.NewUploadController(storage.NewMinio(storage.InitMinio()), producer.NewProducer())
	h := server.New(server.WithHostPorts("127.0.0.1:1234"))
	routes.Routes(h)
	h.Spin()
}
