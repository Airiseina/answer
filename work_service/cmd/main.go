package main

import (
	"answer_pkg/config"
	"answer_pkg/connect"
	"answer_pkg/dal/mysql"
	"answer_pkg/dal/qdrant"
	"answer_pkg/logger"
	"answer_pkg/mq/consumer"
	"answer_pkg/service"
	"answer_pkg/storage"
	"context"
)

func main() {
	minio := storage.NewMinio(storage.InitMinio())
	config.GetConfig()
	logger.InitLogger("work_service", "debug")
	g, err := connect.ConnectMysql() //建表
	if err != nil {
		logger.Fatal("连接数据库失败")
	}
	db := mysql.NewServiceDao(g)
	q, err := qdrant.QdrantInit()
	if err != nil {
		logger.Fatal("连接向量库失败")
	}
	qc := qdrant.NewQdrantDao(q)
	service := service.NewServiceQdrant(db, qc, minio)
	con := consumer.NewConsumer(service)
	con.Consume(context.Background())
	//获得文件信息，取出文件并将其向量化存入数据库，调用ai将其向量化并存入向量库
}
