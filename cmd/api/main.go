package main

import (
	"answer/internal/api/handle"
	"answer/internal/api/middleware"
	"answer/internal/api/rpc"
	"answer/internal/mq"
	"answer/internal/mq/producer"
	"answer/internal/storage"

	"github.com/cloudwego/hertz/pkg/app/server"

	"answer/internal/api/routes"
)

func main() {
	rpc.Connect()
	middleware.JwtMiddleware()
	mq.KafkaInit()
	stoClient := handle.NewUploadController(storage.NewMinio(storage.InitMinio()), producer.NewProducer())
	h := server.New(server.WithHostPorts("127.0.0.1:1234"))
	routes.Routes(h, stoClient)
	h.Spin()
}
