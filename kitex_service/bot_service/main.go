package main

import (
	"answer_pkg/connect"
	"answer_pkg/meter"
	"answer_pkg/tracer"
	"bot_service/internal/config"
	"bot_service/internal/dal"
	"bot_service/internal/model"
	"bot_service/internal/service"
	"bot_service/kitex_gen/bot/botservice"
	"bot_service/rpc"
	"context"
	"net"
	"os"

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
			zap.Fields(zap.String("service", "bot_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	otelAddr := viper.GetString("otel.Addr")
	p := tracer.InitTracer("bot_service", otelAddr)
	defer p.Shutdown(context.Background())
	meter.InitMeter("bot_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
	}
	etcdAddr := viper.GetString("etcd.Addr")
	r, err := etcd.NewEtcdRegistry([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("注册中心出错: %v", err)
	}
	resolver, err := etcd.NewEtcdResolver([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("服务发现出错: %v", err)
	}
	rpc.ConnectUserService(resolver)
	rpc.ConnectChatService(resolver)
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:4323")
	if err != nil {
		klog.Fatalf("监听地址出错:%v", err)
	}
	db, err := connect.ConnectMysql()
	if err != nil {
		klog.Fatalf("连接数据库失败:%v", err)
	}
	err = db.AutoMigrate(&model.Bot{})
	if err != nil {
		klog.Fatalf("数据库建表失败:%v", err)
	}
	botDao := dal.NewBotDao(db)
	botService := service.NewBotService(botDao)
	systemBotId, err := botService.InitSystemBot(context.Background())
	if err != nil {
		klog.Fatalf("系统Bot初始化失败:%v", err)
	}
	klog.Infof("系统Bot ID: %d", systemBotId)
	svr := botservice.NewServer(&BotServiceImpl{botService: botService},
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "botservice"}),
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
