package core

import (
	"context"
	"sync"
	"time"

	"answer_pkg/meter"
	"answer_pkg/snowflake"

	"github.com/cloudwego/kitex/pkg/klog"
)

var snowNode = snowflake.NewNode(1)

type Manager struct {
	Clients    map[uint]*Client
	Register   chan *Client
	Unregister chan *Client
	Message    chan *ClientMessage
	Lock       sync.RWMutex
}

var GlobalManager = Manager{
	Clients:    make(map[uint]*Client),
	Register:   make(chan *Client),
	Unregister: make(chan *Client),
	Message:    make(chan *ClientMessage, 256),
}

func (manager *Manager) Start() {
	klog.Infof("WebSocket 管理器启动...")
	for {
		select {
		case client := <-manager.Register:
			manager.Lock.Lock()
			if old, ok := manager.Clients[client.UserId]; ok {
				old.Socket.Close()
				klog.Infof("用户%d重复连接，关闭旧连接", client.UserId)
			}
			manager.Clients[client.UserId] = client
			meter.M.WsConnectTotal.Add(context.Background(), 1)
			klog.Infof("用户%d上线, 当前在线: %d", client.UserId, len(manager.Clients))
			manager.Lock.Unlock()
		case client := <-manager.Unregister:
			manager.Lock.Lock()
			if _, ok := manager.Clients[client.UserId]; ok {
				delete(manager.Clients, client.UserId)
				meter.M.WsDisconnectTotal.Add(context.Background(), 1)
				klog.Infof("用户%d下线, 当前在线: %d", client.UserId, len(manager.Clients))
			}
			manager.Lock.Unlock()
		case clientMsg := <-manager.Message:
			manager.handleMessage(clientMsg)
		}
	}
}

func (manager *Manager) handleMessage(clientMsg *ClientMessage) {
	wsMsg := clientMsg.Message
	sender := clientMsg.Client

	if wsMsg.Type != "chat" {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "未知消息类型",
			Success: false,
		})
		return
	}
	if wsMsg.To == 0 || wsMsg.Content == "" {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "参数错误",
			Success: false,
		})
		return
	}
	msgID := snowNode.Generate()
	now := time.Now().UnixMilli()
	chatMsg := &WsMessage{
		Type:      "chat",
		From:      sender.UserId,
		To:        wsMsg.To,
		Content:   wsMsg.Content,
		MsgID:     msgID,
		Timestamp: now,
	}
	sender.Send(chatMsg)
	meter.M.MessageSentTotal.Add(context.Background(), 1)
	manager.Lock.RLock()
	receiver, online := manager.Clients[wsMsg.To]
	manager.Lock.RUnlock()
	if online {
		if err := receiver.Send(chatMsg); err != nil {
			klog.Errorf("转发消息给用户%d失败: %v", wsMsg.To, err)
		}
	}
}
