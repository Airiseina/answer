package main

import (
	"answer_pkg/connect"
	"answer_pkg/meter"
	"answer_pkg/tracer"
	"context"
	"net"
	"os"
	"work_service/internal/config"
	"work_service/internal/consumer"
	"work_service/internal/llm"
	"work_service/internal/service"
	"work_service/kitex_gen/work/workservice"
	"work_service/rpc"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/limit"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	kitexzap "github.com/kitex-contrib/obs-opentelemetry/logging/zap"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	kitexZapLogger := kitexzap.NewLogger(
		kitexzap.WithCoreWs(zapcore.AddSync(os.Stdout)),
		kitexzap.WithCoreLevel(zap.NewAtomicLevelAt(zap.DebugLevel)),
		kitexzap.WithZapOptions(
			zap.AddCaller(),
			zap.AddCallerSkip(4),
			zap.Fields(zap.String("service", "work_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	otelAddr := viper.GetString("otel.Addr")
	p := tracer.InitTracer("work_service", otelAddr)
	defer p.Shutdown(context.Background())
	meter.InitMeter("work_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
	}
	etcdAddr := viper.GetString("etcd.Addr")
	r, err := etcd.NewEtcdRegistry([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("注册中心出错: %v", err)
	}
	rpc.Connect()
	llmClient := llm.NewClient(viper.GetString("llm.base_url"))
	workService := service.NewWorkService(llmClient)
	rdb, err := connect.ConnectRedis()
	if err != nil {
		klog.Fatalf("连接Redis失败: %v", err)
	}
	botConsumer := consumer.NewBotTaskConsumer(rdb, workService)
	go botConsumer.Start(context.Background())
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:4324")
	if err != nil {
		klog.Fatalf("监听地址出错:%v", err)
	}
	svr := workservice.NewServer(&WorkServiceImpl{workService: workService},
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "workservice"}),
		server.WithServiceAddr(addr),
		server.WithRegistry(r),
		server.WithLimit(&limit.Option{
			MaxConnections: 1000,
			MaxQPS:         2000,
		}))
	err = svr.Run()
	if err != nil {
		klog.Fatalf("服务启动失败:%v", err)
	}
}
