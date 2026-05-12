package main

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"answer_pkg/meter"
	"answer_pkg/tracer"
	"msg_gateway/core"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/gorilla/websocket"
	kitexzap "github.com/kitex-contrib/obs-opentelemetry/logging/zap"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "未授权", http.StatusUnauthorized)
		return
	}
	userId, err := strconv.ParseUint(token, 10, 32)
	if err != nil {
		http.Error(w, "无效的令牌", http.StatusUnauthorized)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		klog.Errorf("WebSocket 升级失败: %v", err)
		return
	}
	client := &core.Client{
		Manager: &core.GlobalManager,
		UserId:  uint(userId),
		Socket:  conn,
	}
	core.GlobalManager.Register <- client
	go client.ReadMessage()
}

func main() {
	kitexZapLogger := kitexzap.NewLogger(
		kitexzap.WithCoreWs(zapcore.AddSync(os.Stdout)),
		kitexzap.WithCoreLevel(zap.NewAtomicLevelAt(zap.DebugLevel)),
		kitexzap.WithZapOptions(
			zap.AddCaller(),
			zap.AddCallerSkip(1),
			zap.Fields(zap.String("service", "msg_gateway")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	p := tracer.InitTracer("msg_gateway", "localhost:4317")
	defer p.Shutdown(context.Background())
	meter.InitMeter("msg_gateway")
	meter.RegisterOnlineUsers(func(ctx context.Context, observer metric.Int64Observer) error {
		observer.Observe(int64(len(core.GlobalManager.Clients)))
		return nil
	})
	go core.GlobalManager.Start()
	http.Handle("/ws", otelhttp.NewHandler(http.HandlerFunc(handleWebSocket), "/ws"))
	if err := http.ListenAndServe(":8081", nil); err != nil {
		klog.Fatalf("服务启动失败: %v", err)
	}
}
