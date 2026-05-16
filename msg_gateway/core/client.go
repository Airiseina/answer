package core

import (
	"encoding/json"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/gorilla/websocket"
)

type Client struct {
	Manager *Manager
	UserId  uint
	Socket  *websocket.Conn
}

type WsMessage struct {
	Type      string `json:"type"`
	To        uint   `json:"to,omitempty"`
	From      uint   `json:"from,omitempty"`
	Content   string `json:"content,omitempty"`
	MsgID     int64  `json:"msg_id,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
	Success   bool   `json:"success,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type ClientMessage struct {
	Client  *Client
	Message *WsMessage
}

func (client *Client) ReadMessage() {
	defer func() {
		client.Manager.Unregister <- client
		client.Socket.Close()
	}()
	for {
		_, message, err := client.Socket.ReadMessage()
		if err != nil {
			klog.Errorf("读取%d用户消息失败: %v", client.UserId, err)
			break
		}
		var wsMsg WsMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			klog.Errorf("解析%d用户消息失败: %v", client.UserId, err)
			client.Send(&WsMessage{
				Type:    "system",
				Reason:  "消息格式错误",
				Success: false,
			})
			continue
		}
		client.Manager.Message <- &ClientMessage{
			Client:  client,
			Message: &wsMsg,
		}
	}
}

func (client *Client) Send(msg *WsMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return client.Socket.WriteMessage(websocket.TextMessage, data)
}
