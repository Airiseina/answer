package ws

import (
	"answer/pkg/logger"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Client struct {
	Manager *Manager
	UserId  uint
	Socket  *websocket.Conn
}

func (client *Client) ReadMessage() {
	for {
		messageType, message, err := client.Socket.ReadMessage()
		if err != nil {
			logger.Error("读取用户消息失败", zap.Any("用户id", client.UserId))
			break
		}
		//使用消息队列，之后我们会接入 Kafka 或者直接路由，不仅发给用户，也要发给自己
		logger.Info("收到用户消息", zap.Any("用户id", client.UserId), zap.Any("用户消息", message), zap.Any("type", messageType))
	}
	defer func() {
		client.Manager.Unregister <- client
		client.Socket.Close()
	}()
}
