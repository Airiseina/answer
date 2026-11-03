package main

import (
	"context"
	"net"
	"os"

	"github.com/Airiseina/answer/kitex_service/admin_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/admin_service/internal/dal/mysql"
	"github.com/Airiseina/answer/kitex_service/admin_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/admin_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/admin_service/kitex_gen/admin/adminservice"
	"github.com/Airiseina/answer/pkg/audit"
	"github.com/Airiseina/answer/pkg/infra"
	"github.com/Airiseina/answer/pkg/observability/meter"
	"github.com/Airiseina/answer/pkg/observability/tracer"
	"github.com/Airiseina/answer/pkg/snowflake"

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

// snowNode 是管理后台业务 ID 的生成器（分片 admin=9）。
var snowNode = snowflake.NewNode(snowflake.ShardAdmin)

func main() {
	kitexZapLogger := kitexzap.NewLogger(
		kitexzap.WithCoreWs(zapcore.AddSync(os.Stdout)),
		kitexzap.WithCoreLevel(zap.NewAtomicLevelAt(zap.DebugLevel)),
		kitexzap.WithZapOptions(
			zap.AddCaller(),
			zap.AddCallerSkip(3),
			zap.Fields(zap.String("service", "admin_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	v := config.V
	otelAddr := v.GetString("otel.Addr")
	p := tracer.InitTracer("admin_service", otelAddr)
	defer func() { _ = p.Shutdown(context.Background()) }()
	meter.InitMeter("admin_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		_ = os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
	}
	etcdAddr := v.GetString("etcd.Addr")
	r, err := etcd.NewEtcdRegistry([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("注册中心出错: %v", err)
	}
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:4329")
	if err != nil {
		klog.Fatalf("监听地址出错:%v", err)
	}
	db, err := infra.ConnectMysql(v)
	if err != nil {
		klog.Fatalf("连接数据库失败:%v", err)
	}
	if err = db.AutoMigrate(&model.AdminUser{}, &audit.Log{}); err != nil {
		klog.Fatalf("数据库建表失败:%v", err)
	}
	adminDao := mysql.NewAdminDao(db)
	adminService := service.NewAdminService(adminDao, audit.NewRecorder(db))
	if err = adminService.EnsureBootstrap(context.Background(), config.BootstrapAdmins()); err != nil {
		klog.Fatalf("初始化管理员账号失败:%v", err)
	}
	svr := adminservice.NewServer(&AdminServiceImpl{adminService: adminService},
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "adminservice"}),
		server.WithServiceAddr(addr),
		server.WithRegistry(r),
		server.WithLimit(&limit.Option{
			MaxConnections: 200,
			MaxQPS:         500,
		}))
	err = svr.Run()
	if err != nil {
		klog.Fatalf("服务启动失败:%v", err)
	}
}
