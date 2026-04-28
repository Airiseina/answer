package handle

import (
	"answer/internal/ws"
	"answer/pkg/logger"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func HandleWebsocket(w http.ResponseWriter, r *http.Request) {
	userIdStr := r.URL.Query().Get("user_id")
	userId, _ := strconv.ParseUint(userIdStr, 10, 64)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("ws协议升级失败", zap.Error(err))
		return
	}
	defer conn.Close()
	client := &ws.Client{
		Manager: &ws.GlobalManager,
		UserId:  uint(userId),
		Socket:  conn,
	}
	ws.GlobalManager.Register <- client
	client.ReadMessage()
}
