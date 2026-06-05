package main

import (
	"context"

	"github.com/Airiseina/answer/kitex_service/group_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/group_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/group_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/group_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/group_service/rpc"
	"github.com/Airiseina/answer/pkg/infra"
	"github.com/Airiseina/answer/pkg/observability/meter"
	"github.com/Airiseina/answer/pkg/observability/tracer"

	"net"
	"os"

	"github.com/Airiseina/answer/kitex_service/group_service/kitex_gen/group/groupservice"

	"github.com/cloudwego/hertz/pkg/common/hlog"
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
			zap.Fields(zap.String("service", "group_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	v := config.V
	otelAddr := v.GetString("otel.Addr")
	p := tracer.InitTracer("group_service", otelAddr)
	defer func() { _ = p.Shutdown(context.Background()) }()
	meter.InitMeter("group_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		_ = os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
	}
	etcdAddr := v.GetString("etcd.Addr")
	r, err := etcd.NewEtcdRegistry([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("注册中心出错: %v", err)
	}
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:4321")
	if err != nil {
		klog.Fatalf("监听地址出错:%v", err)
	}
	db, err := infra.ConnectMysql(v)
	if err != nil {
		klog.Fatalf("连接数据库失败:%v", err)
	}
	err = db.AutoMigrate(&model.Group{}, &model.GroupMember{}, &model.GroupJoinRequest{}, &model.GroupNotice{})
	if err != nil {
		klog.Fatalf("数据库建表失败:%v", err)
	}
	groupService := service.NewGroupService(dal.NewGroupDao(db))
	svr := groupservice.NewServer(&GroupServiceImpl{groupService: groupService},
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "groupservice"}),
		server.WithServiceAddr(addr),
		server.WithRegistry(r),
		server.WithLimit(&limit.Option{
			MaxConnections: 500,
			MaxQPS:         1000,
		}))
	r1, err := etcd.NewEtcdResolver([]string{etcdAddr})
	if err != nil {
		hlog.Fatalf("连接etcd出错:%v", err)
	}
	rpc.ConnectUserService(r1)
	rpc.ConnectChatService(r1)
	err = svr.Run()
	if err != nil {
		klog.Fatalf("服务启动失败:%v", err)
	}
}
