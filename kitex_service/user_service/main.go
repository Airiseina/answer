package main

import (
	"answer_pkg/connect"
	"answer_pkg/logger"
	"net"
	"os"
	"user_service/internal/config"
	"user_service/internal/dal/mysql"
	"user_service/internal/model"
	"user_service/internal/service"
	"user_service/kitex_gen/user/loginservice"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/limit"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	kitexzap "github.com/kitex-contrib/obs-opentelemetry/logging/zap"
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
			zap.AddCallerSkip(1),
			zap.Fields(zap.String("service", "chat_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	r, err := etcd.NewEtcdRegistry([]string{"127.0.0.1:2379"})
	if err != nil {
		logger.Fatal("注册中心出错", zap.Error(err))
	}
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:4321")
	if err != nil {
		logger.Fatal("监听地址出错", zap.Error(err))
	}
	db, err := connect.ConnectMysql()
	if err != nil {
		logger.Fatal("连接数据库失败", zap.Error(err))
	}
	err = db.AutoMigrate(&model.User{}) //建表
	if err != nil {
		logger.Fatal("数据库建表失败", zap.Error(err))
	}
	userService := service.NewUserService(mysql.NewUserDao(db))
	svr := loginservice.NewServer(&LoginServiceImpl{userService: userService},
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "userservice"}),
		server.WithServiceAddr(addr),
		server.WithRegistry(r),
		server.WithLimit(&limit.Option{
			MaxConnections: 1000,
			MaxQPS:         2000,
		}))
	err = svr.Run()
	if err != nil {
		logger.Fatal("服务启动失败", zap.Error(err))
	}
}
