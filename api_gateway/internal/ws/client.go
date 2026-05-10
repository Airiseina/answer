package ws

import (
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/gorilla/websocket"
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
			hlog.Errorf("读取%d用户消息失败", client.UserId)
			break
		}
		//使用消息队列，之后我们会接入 Kafka 或者直接路由，不仅发给用户，也要发给自己
		hlog.Infof("收到%d用户消息:%s:格式：%d", client.UserId, message, messageType)
	}
	defer func() {
		client.Manager.Unregister <- client
		client.Socket.Close()
	}()
}
