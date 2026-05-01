package ws

import (
	"sync"

	"github.com/cloudwego/hertz/pkg/common/hlog"
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
	hlog.Infof("WebSocket 管理器启动...")
	for {
		select {
		case client := <-manager.Register:
			manager.Lock.Lock()
			manager.Clients[client.UserId] = client
			hlog.Infof("%d用户上线", client.UserId)
			manager.Lock.Unlock()
		case client := <-manager.Unregister:
			manager.Lock.Lock()
			if _, ok := manager.Clients[client.UserId]; ok {
				delete(manager.Clients, client.UserId)
				client.Socket.Close()
				hlog.Infof("用户下线")
			}
			manager.Lock.Unlock()
		}
	}
}
