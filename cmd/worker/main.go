package main

import (
	"answer/internal/connect"
	"answer/internal/dal/mysql"
	"answer/internal/dal/qdrant"
	"answer/internal/mq/consumer"
	service2 "answer/internal/service"
	"answer/internal/storage"
	"answer/pkg/config"
	"answer/pkg/logger"
	"context"
)

func main() {
	minio := storage.NewMinio(storage.InitMinio())
	config.GetConfig()
	g, err := connect.Connect()
	if err != nil {
		logger.Fatal("连接数据库失败")
	}
	db := mysql.NewServiceDao(g)
	q, err := qdrant.QdrantInit()
	if err != nil {
		logger.Fatal("连接向量库失败")
	}
	qc := qdrant.NewQdrantDao(q)
	service := service2.NewServiceQdrant(db, qc, minio)
	con := consumer.NewConsumer(service)
	con.Consume(context.Background())
	//获得文件信息，取出文件并将其向量化存入数据库，调用ai将其向量化并存入向量库
}
