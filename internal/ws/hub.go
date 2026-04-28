package ws

import (
	"answer/pkg/logger"
	"sync"

	"go.uber.org/zap"
)

type Manager struct {
	Clients    map[uint]*Client
	Register   chan *Client
	Unregister chan *Client
	Lock       sync.RWMutex
}

var GlobalManager = Manager{
	Clients:    make(map[uint]*Client),
	Register:   make(chan *Client),
	Unregister: make(chan *Client),
}

func (manager *Manager) Start() {
	logger.Info("WebSocket 管理器启动...")
	for {
		select {
		case client := <-manager.Register:
			manager.Lock.Lock()
			manager.Clients[client.UserId] = client
			logger.Info("用户上线", zap.Any("client", client.UserId))
			manager.Lock.Unlock()
		case client := <-manager.Unregister:
			manager.Lock.Lock()
			if _, ok := manager.Clients[client.UserId]; ok {
				delete(manager.Clients, client.UserId)
				client.Socket.Close()
				logger.Info("用户下线", zap.Any("client", client.UserId))
			}
			manager.Lock.Unlock()
		}
	}
}
