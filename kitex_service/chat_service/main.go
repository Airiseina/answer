package main

import (
	"context"
	"net"
	"os"

	"github.com/Airiseina/answer/kitex_service/chat_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/chat_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/chat_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/chat_service/internal/service"
	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat/chatservice"
	"github.com/Airiseina/answer/kitex_service/chat_service/rpc"
	"github.com/Airiseina/answer/pkg/infra"
	"github.com/Airiseina/answer/pkg/observability/meter"
	"github.com/Airiseina/answer/pkg/observability/tracer"

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
			zap.AddCallerSkip(3),
			zap.Fields(zap.String("service", "chat_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	v := config.V
	otelAddr := v.GetString("otel.Addr")
	p := tracer.InitTracer("chat_service", otelAddr)
	defer p.Shutdown(context.Background())
	meter.InitMeter("chat_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
	}
	etcdAddr := v.GetString("etcd.Addr")
	r, err := etcd.NewEtcdRegistry([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("注册中心出错: %v", err)
	}
	rpc.ConnectGroupService()
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:4322")
	if err != nil {
		klog.Fatalf("监听地址出错:%v", err)
	}
	db, err := infra.ConnectPostgres(v)
	if err != nil {
		klog.Fatalf("连接PostgreSQL失败:%v", err)
	}
	db.Exec("UPDATE message_table SET conversation_id = 0 WHERE conversation_id IS NULL")
	err = db.AutoMigrate(&model.Message{}, &model.Conversation{}, &model.ConversationMember{}, &model.MessageEditHistory{})
	if err != nil {
		klog.Fatalf("数据库建表失败:%v", err)
	}
	rdb, err := infra.ConnectRedis(v)
	if err != nil {
		klog.Fatalf("连接Redis失败:%v", err)
	}
	chatDao := dal.NewChatDao(db)
	onlineDao := dal.NewOnlineDao(rdb)
	conversationDao := dal.NewConversationDao(db, rdb)
	chatService := service.NewChatService(chatDao, onlineDao, conversationDao, rdb)
	svr := chat.NewServer(&ChatServiceImpl{chatService: chatService},
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "chatservice"}),
		server.WithServiceAddr(addr),
		server.WithRegistry(r),
		server.WithLimit(&limit.Option{
			MaxConnections: 5000,
			MaxQPS:         5000,
		}))
	err = svr.Run()
	if err != nil {
		klog.Fatalf("服务启动失败:%v", err)
	}
}
