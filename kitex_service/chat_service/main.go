package main

import (
	"answer_pkg/connect"
	"answer_pkg/meter"
	"answer_pkg/tracer"
	"chat_service/internal/config"
	"chat_service/internal/dal"
	"chat_service/internal/model"
	"chat_service/internal/service"
	chat "chat_service/kitex_gen/chat/chatservice"
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
			zap.Fields(zap.String("service", "chat_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	otelAddr := viper.GetString("otel.Addr")
	p := tracer.InitTracer("chat_service", otelAddr)
	defer p.Shutdown(context.Background())
	meter.InitMeter("chat_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
	}
	etcdAddr := viper.GetString("etcd.Addr")
	r, err := etcd.NewEtcdRegistry([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("注册中心出错: %v", err)
	}
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:4322")
	if err != nil {
		klog.Fatalf("监听地址出错:%v", err)
	}
	db, err := connect.ConnectPostgres()
	if err != nil {
		klog.Fatalf("连接PostgreSQL失败:%v", err)
	}
	err = db.AutoMigrate(&model.Message{}, &model.Conversation{}, &model.ConversationMember{})
	if err != nil {
		klog.Fatalf("数据库建表失败:%v", err)
	}
	rdb, err := connect.ConnectRedis()
	if err != nil {
		klog.Fatalf("连接Redis失败:%v", err)
	}
	chatDao := dal.NewChatDao(db)
	onlineDao := dal.NewOnlineDao(rdb)
	conversationDao := dal.NewConversationDao(db, rdb)
	chatService := service.NewChatService(chatDao, onlineDao, conversationDao)
	svr := chat.NewServer(&ChatServiceImpl{chatService: chatService},
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "chatservice"}),
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
