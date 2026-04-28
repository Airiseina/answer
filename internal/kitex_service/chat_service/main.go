package main

import (
	"answer/internal/kitex_service/chat_service/kitex_gen/chat/chat"
	"answer/pkg/logger"

	"go.uber.org/zap"
)

func main() {
	svr := chat.NewServer(&ChatImpl{})
	err := svr.Run()
	if err != nil {
		logger.Fatal("服务启动失败", zap.Error(err))
	}
}
