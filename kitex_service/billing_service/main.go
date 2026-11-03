package main

import (
	"context"
	"net"
	"os"

	"github.com/Airiseina/answer/kitex_service/billing_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/billing_service/internal/dal/mysql"
	"github.com/Airiseina/answer/kitex_service/billing_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/billing_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/billing_service/kitex_gen/billing/billingservice"
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

func main() {
	kitexZapLogger := kitexzap.NewLogger(
		kitexzap.WithCoreWs(zapcore.AddSync(os.Stdout)),
		kitexzap.WithCoreLevel(zap.NewAtomicLevelAt(zap.DebugLevel)),
		kitexzap.WithZapOptions(
			zap.AddCaller(),
			zap.AddCallerSkip(4),
			zap.Fields(zap.String("service", "billing_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	v := config.V
	otelAddr := v.GetString("otel.Addr")
	p := tracer.InitTracer("billing_service", otelAddr)
	defer func() { _ = p.Shutdown(context.Background()) }()
	meter.InitMeter("billing_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		_ = os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
	}
	etcdAddr := v.GetString("etcd.Addr")
	r, err := etcd.NewEtcdRegistry([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("注册中心出错: %v", err)
	}
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:4328")
	if err != nil {
		klog.Fatalf("监听地址出错:%v", err)
	}
	db, err := infra.ConnectMysql(v)
	if err != nil {
		klog.Fatalf("连接数据库失败:%v", err)
	}
	if err = db.AutoMigrate(&model.TokenUsage{}, &model.ModelPrice{}, &model.UsageQuota{}, &audit.Log{}); err != nil {
		klog.Fatalf("数据库建表失败:%v", err)
	}
	billingDao := mysql.NewBillingDao(db)
	billingService := service.NewBillingService(billingDao, audit.NewRecorder(db))

	svr := billingservice.NewServer(&BillingServiceImpl{billingService: billingService},
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "billingservice"}),
		server.WithServiceAddr(addr),
		server.WithRegistry(r),
		server.WithLimit(&limit.Option{
			MaxConnections: 500,
			MaxQPS:         2000,
		}))
	err = svr.Run()
	if err != nil {
		klog.Fatalf("服务启动失败:%v", err)
	}
}
