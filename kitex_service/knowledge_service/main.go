package main

import (
	"context"
	"net"
	"os"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/consumer"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/kitex_gen/knowledge/knowledgeservice"
	"github.com/Airiseina/answer/pkg/ai"
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
	"github.com/qdrant/go-client/qdrant"
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
			zap.Fields(zap.String("service", "knowledge_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	v := config.V
	otelAddr := v.GetString("otel.Addr")
	p := tracer.InitTracer("knowledge_service", otelAddr)
	defer p.Shutdown(context.Background())
	meter.InitMeter("knowledge_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
	}
	etcdAddr := v.GetString("etcd.Addr")
	r, err := etcd.NewEtcdRegistry([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("注册中心出错: %v", err)
	}
	db, err := infra.ConnectMysql(v)
	if err != nil {
		klog.Fatalf("连接数据库失败:%v", err)
	}
	err = db.AutoMigrate(&model.KnowledgeBase{}, &model.KbDocument{}, &model.BotKnowledge{})
	if err != nil {
		klog.Fatalf("数据库建表失败:%v", err)
	}
	storage.Init(v)
	ai.AiInit()
	qdrantClient, err := initQdrant()
	if err != nil {
		klog.Fatalf("连接Qdrant失败:%v", err)
	}
	kbDao := dal.NewKnowledgeBaseDao(db)
	docDao := dal.NewDocumentDao(db)
	bkDao := dal.NewBotKnowledgeDao(db)
	kafkaBroker := v.GetString("kafka.brokers")
	kafkaTopic := v.GetString("kafka.topic.doc_parse")
	knowledgeService := service.NewKnowledgeService(kbDao, docDao, bkDao, qdrantClient, kafkaBroker, kafkaTopic)
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:4326")
	if err != nil {
		klog.Fatalf("监听地址出错:%v", err)
	}
	kafkaBrokers := []string{v.GetString("kafka.brokers")}
	docParseTopic := v.GetString("kafka.topic.doc_parse")
	kafkaGroup := v.GetString("kafka.group.knowledge")
	docConsumer := consumer.NewDocParseConsumer(kafkaBrokers, docParseTopic, kafkaGroup, knowledgeService, docDao)
	stuckDocs, err := docDao.GetStuckDocuments()
	if err == nil && len(stuckDocs) > 0 {
		for _, doc := range stuckDocs {
			_ = docDao.UpdateStatus(doc.ID, model.DocStatusPending, 0, "")
			_ = knowledgeService.RepublishDocParse(doc.ID, doc.KBID)
		}
		klog.Infof("已重置%d个卡在parsing状态的文档", len(stuckDocs))
	}
	go docConsumer.Start()
	defer docConsumer.Stop()
	svr := knowledgeservice.NewServer(
		&KnowledgeServiceImpl{knowledgeService: knowledgeService},
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "knowledge_service"}),
		server.WithServiceAddr(addr),
		server.WithRegistry(r),
		server.WithLimit(&limit.Option{
			MaxConnections: 1000,
			MaxQPS:         2000,
		}),
	)
	err = svr.Run()
	if err != nil {
		klog.Fatalf("服务启动失败:%v", err)
	}
}

func initQdrant() (*qdrant.Client, error) {
	v := config.V
	host := v.GetString("qdrant.host")
	port := v.GetInt("qdrant.port")
	return qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
}
