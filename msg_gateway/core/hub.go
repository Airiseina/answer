package core

import (
	"context"
	"sync"

	"answer_pkg/meter"

	"github.com/cloudwego/kitex/pkg/klog"
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
	klog.Infof("WebSocket 管理器启动...")
	for {
		select {
		case client := <-manager.Register:
			manager.Lock.Lock()
			manager.Clients[client.UserId] = client
			meter.M.WsConnectTotal.Add(context.Background(), 1)
			klog.Infof("%d用户上线", client.UserId)
			manager.Lock.Unlock()
		case client := <-manager.Unregister:
			manager.Lock.Lock()
			if _, ok := manager.Clients[client.UserId]; ok {
				delete(manager.Clients, client.UserId)
				meter.M.WsDisconnectTotal.Add(context.Background(), 1)
				client.Socket.Close()
				klog.Infof("用户下线")
			}
			manager.Lock.Unlock()
		}
	}
}
