package main

import (
	"context"
	"net"
	"os"

	"github.com/Airiseina/answer/kitex_service/work_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/consumer"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/llm"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/mcp"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/scheduler"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/work_service/kitex_gen/work/workservice"
	"github.com/Airiseina/answer/kitex_service/work_service/rpc"
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
			zap.AddCallerSkip(4),
			zap.Fields(zap.String("service", "work_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	v := config.V
	otelAddr := v.GetString("otel.Addr")
	p := tracer.InitTracer("work_service", otelAddr)
	defer func() { _ = p.Shutdown(context.Background()) }()
	meter.InitMeter("work_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		_ = os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
	}
	etcdAddr := v.GetString("etcd.Addr")
	r, err := etcd.NewEtcdRegistry([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("注册中心出错: %v", err)
	}
	rpc.Connect(v)
	llmClient := llm.NewClient()
	kafkaWriter, err := infra.ConnectKafkaProducer(v)
	if err != nil {
		klog.Fatalf("连接Kafka Producer失败: %v", err)
	}
	mcpPool := mcp.NewPool()
	mcpPool.StartHealthCheck()
	defer mcpPool.Close()

	// 日/周总结依赖 MySQL 持久化报告与订阅；Redis 仅用于调度跨实例去重（可降级）。
	db, err := infra.ConnectMysql(v)
	if err != nil {
		klog.Fatalf("连接数据库失败:%v", err)
	}
	if err = db.AutoMigrate(&model.SummaryReport{}, &model.SummarySchedule{}); err != nil {
		klog.Fatalf("数据库建表失败:%v", err)
	}
	rdb, err := infra.ConnectRedis(v)
	if err != nil {
		klog.Warnf("连接Garnet失败，总结调度去重将降级为单实例执行: %v", err)
	}
	summaryDao := dal.NewSummaryDao(db)
	workService := service.NewWorkService(llmClient, kafkaWriter, mcpPool, summaryDao)

	// 定时生成日/周总结：每天 send_hour 触发日总结，周日同时触发周总结。
	summaryScheduler := scheduler.NewSummaryScheduler(workService, rdb)
	summaryScheduler.Start(context.Background())

	kafkaReader, err := infra.ConnectKafkaConsumerGroup(v, "bot-worker-group", "bot-task-topic")
	if err != nil {
		klog.Fatalf("连接Kafka ConsumerGroup失败: %v", err)
	}
	botConsumer := consumer.NewBotTaskConsumer(kafkaReader, workService)
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
			MaxConnections: 200,
			MaxQPS:         200,
		}))
	err = svr.Run()
	if err != nil {
		klog.Fatalf("服务启动失败:%v", err)
	}
}
