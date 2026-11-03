package main

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/Airiseina/answer/kitex_service/open_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/open_service/internal/consumer"
	"github.com/Airiseina/answer/kitex_service/open_service/internal/dal/mysql"
	"github.com/Airiseina/answer/kitex_service/open_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/open_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/open_service/kitex_gen/open/openservice"
	openrpc "github.com/Airiseina/answer/kitex_service/open_service/rpc"
	"github.com/Airiseina/answer/pkg/audit"
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

const (
	retryInterval = 10 * time.Second  // 回调重试扫描间隔
	purgeInterval = time.Hour         // 过期事件清理间隔
)

func main() {
	kitexZapLogger := kitexzap.NewLogger(
		kitexzap.WithCoreWs(zapcore.AddSync(os.Stdout)),
		kitexzap.WithCoreLevel(zap.NewAtomicLevelAt(zap.DebugLevel)),
		kitexzap.WithZapOptions(
			zap.AddCaller(),
			zap.AddCallerSkip(4),
			zap.Fields(zap.String("service", "open_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	v := config.V
	otelAddr := v.GetString("otel.Addr")
	p := tracer.InitTracer("open_service", otelAddr)
	defer func() { _ = p.Shutdown(context.Background()) }()
	meter.InitMeter("open_service")
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
	openrpc.ConnectBotService(resolver)
	openrpc.ConnectChatService(resolver)
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:4327")
	if err != nil {
		klog.Fatalf("监听地址出错:%v", err)
	}
	db, err := infra.ConnectMysql(v)
	if err != nil {
		klog.Fatalf("连接数据库失败:%v", err)
	}
	if err = db.AutoMigrate(&model.OpenApp{}, &model.OpenWebhook{}, &model.OpenEvent{}, &model.OpenWebhookDelivery{}, &audit.Log{}); err != nil {
		klog.Fatalf("数据库建表失败:%v", err)
	}
	rdb, err := infra.ConnectRedis(v)
	if err != nil {
		// nonce 去重强依赖 Garnet，缺失时降级为「拒绝所有请求」由 service 层控制。
		klog.Warnf("连接Garnet失败，开放平台防重放能力将降级: %v", err)
	}
	openDao := mysql.NewOpenDao(db)
	openService := service.NewOpenService(openDao, audit.NewRecorder(db), rdb)

	// 事件消费：订阅 Bot 任务与回复话题，把开放平台相关事件落库并回调。
	taskReader, err := infra.ConnectKafkaConsumerGroup(v, "open-dispatch-task-group", "bot-task-topic")
	if err != nil {
		klog.Fatalf("连接Bot任务Kafka ConsumerGroup失败: %v", err)
	}
	replyReader, err := infra.ConnectKafkaConsumerGroup(v, "open-dispatch-reply-group", "bot-reply-topic")
	if err != nil {
		klog.Fatalf("连接Bot回复Kafka ConsumerGroup失败: %v", err)
	}
	dispatcher := consumer.NewOpenDispatchConsumer(taskReader, replyReader, openService)
	dispatcher.Start()
	defer dispatcher.Stop()

	// 后台协程：投递重试与过期事件清理。
	go func() {
		ticker := time.NewTicker(retryInterval)
		defer ticker.Stop()
		for range ticker.C {
			openService.RetryDueDeliveries(context.Background())
		}
	}()
	go func() {
		ticker := time.NewTicker(purgeInterval)
		defer ticker.Stop()
		for range ticker.C {
			openService.PurgeExpiredEvents(context.Background())
		}
	}()

	svr := openservice.NewServer(&OpenServiceImpl{openService: openService},
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "openservice"}),
		server.WithServiceAddr(addr),
		server.WithRegistry(r),
		server.WithLimit(&limit.Option{
			MaxConnections: 500,
			MaxQPS:         1000,
		}))
	err = svr.Run()
	if err != nil {
		klog.Fatalf("服务启动失败:%v", err)
	}
}
