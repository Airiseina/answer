package main

import (
	"context"
	"net"
	"os"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/bot_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/bot_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/bot_service/kitex_gen/bot/botservice"
	"github.com/Airiseina/answer/kitex_service/bot_service/rpc"
	"github.com/Airiseina/answer/pkg/infra"
	"github.com/Airiseina/answer/pkg/observability/meter"
	"github.com/Airiseina/answer/pkg/observability/tracer"
	"github.com/Airiseina/answer/pkg/storage"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/limit"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	kitexzap "github.com/kitex-contrib/obs-opentelemetry/logging/zap"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	etcd "github.com/kitex-contrib/registry-etcd"
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
	v := config.V
	otelAddr := v.GetString("otel.Addr")
	p := tracer.InitTracer("bot_service", otelAddr)
	defer func() { _ = p.Shutdown(context.Background()) }()
	meter.InitMeter("bot_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		_ = os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
	}
	etcdAddr := v.GetString("etcd.Addr")
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
	rpc.ConnectKnowledgeService(resolver)
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:4323")
	if err != nil {
		klog.Fatalf("监听地址出错:%v", err)
	}
	db, err := infra.ConnectMysql(v)
	if err != nil {
		klog.Fatalf("连接数据库失败:%v", err)
	}
	storage.Init(v)
	err = db.AutoMigrate(&model.Bot{}, &model.McpServer{})
	if err != nil {
		klog.Fatalf("数据库建表失败:%v", err)
	}
	botDao := dal.NewBotDao(db)
	mcpServerDao := dal.NewMcpServerDao(db)
	botService := service.NewBotService(botDao)
	mcpServerService := service.NewMcpServerService(mcpServerDao, botDao)
	systemBotId, err := botService.InitSystemBot(context.Background())
	if err != nil {
		klog.Fatalf("系统Bot初始化失败:%v", err)
	}
	klog.Infof("系统Bot ID: %d", systemBotId)
	svr := botservice.NewServer(&BotServiceImpl{botService: botService, mcpServerService: mcpServerService},
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "botservice"}),
		server.WithServiceAddr(addr),
		server.WithRegistry(r),
		server.WithLimit(&limit.Option{
			MaxConnections: 500,
			MaxQPS:         500,
		}))
	err = svr.Run()
	if err != nil {
		klog.Fatalf("服务启动失败:%v", err)
	}
}
