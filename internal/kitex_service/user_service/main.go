package main

import (
	"answer/internal/connect"
	"answer/internal/kitex_service/user_service/dal/mysql"
	"answer/internal/kitex_service/user_service/kitex_gen/user/loginservice"
	"answer/internal/kitex_service/user_service/service"
	"answer/pkg/config"
	"answer/pkg/logger"
	"net"

	"github.com/cloudwego/kitex/pkg/limit"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	etcd "github.com/kitex-contrib/registry-etcd"
	"go.uber.org/zap"
)

func main() {
	config.GetConfig()
	r, err := etcd.NewEtcdRegistry([]string{"127.0.0.1:2379"})
	if err != nil {
		logger.Fatal("注册中心出错", zap.Error(err))
	}
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:4321")
	if err != nil {
		logger.Fatal("监听地址出错", zap.Error(err))
	}
	db, err := connect.Connect()
	if err != nil {
		logger.Fatal("连接数据库失败", zap.Error(err))
	}
	userService := service.NewUserService(mysql.NewUserDao(db))
	svr := loginservice.NewServer(&LoginServiceImpl{userService: userService}, server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "userservice"}), server.WithServiceAddr(addr),
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
