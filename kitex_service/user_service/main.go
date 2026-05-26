package main

import (
	"context"
	"net"
	"os"

	"github.com/Airiseina/answer/kitex_service/user_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/user_service/internal/dal/mysql"
	"github.com/Airiseina/answer/kitex_service/user_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/user_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user/loginservice"
	"github.com/Airiseina/answer/pkg/connect"
	"github.com/Airiseina/answer/pkg/meter"
	"github.com/Airiseina/answer/pkg/tracer"

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
			zap.AddCallerSkip(3),
			zap.Fields(zap.String("service", "user_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	otelAddr := viper.GetString("otel.Addr")
	p := tracer.InitTracer("user_service", otelAddr)
	defer p.Shutdown(context.Background())
	meter.InitMeter("user_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
	}
	etcdAddr := viper.GetString("etcd.Addr")
	r, err := etcd.NewEtcdRegistry([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("注册中心出错: %v", err)
	}
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:4320")
	if err != nil {
		klog.Fatalf("监听地址出错:%v", err)
	}
	db, err := connect.ConnectMysql()
	if err != nil {
		klog.Fatalf("连接数据库失败:%v", err)
	}
	err = db.AutoMigrate(&model.User{}, &model.FriendRequest{}, &model.Friend{}, &model.FriendGroup{})
	if err != nil {
		klog.Fatalf("数据库建表失败:%v", err)
	}
	userDao := mysql.NewUserDao(db)
	userService := service.NewUserService(userDao)
	friendService := service.NewFriendService(userDao)
	svr := loginservice.NewServer(&LoginServiceImpl{userService: userService, friendService: friendService},
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "userservice"}),
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
