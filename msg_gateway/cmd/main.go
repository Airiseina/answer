package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/Airiseina/answer/msg_gateway/config"
	"github.com/Airiseina/answer/msg_gateway/core"
	"github.com/Airiseina/answer/msg_gateway/rpc"
	"github.com/Airiseina/answer/pkg/infra"
	"github.com/Airiseina/answer/pkg/observability/meter"
	"github.com/Airiseina/answer/pkg/observability/tracer"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/gorilla/websocket"
	kitexzap "github.com/kitex-contrib/obs-opentelemetry/logging/zap"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var jwtKey []byte

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH, HEAD")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Length, Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "43200")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	identity, err := authenticateToken(tokenFromRequest(r), jwtKey)
	if err != nil {
		klog.Warnf("WebSocket 鉴权失败: %v", err)
		http.Error(w, "无效的令牌", http.StatusUnauthorized)
		return
	}
	klog.Infof("JWT 验证成功, userId: %d, tenantId: %d", identity.UserID, identity.TenantID)
	nameCtx, nameCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer nameCancel()
	userName := rpc.GetUserName(nameCtx, identity.UserID)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		klog.Errorf("WebSocket 升级失败: %v", err)
		return
	}
	client := &core.Client{
		Manager:     &core.GlobalManager,
		UserId:      identity.UserID,
		TenantID:    identity.TenantID,
		UserName:    userName,
		UserAccount: identity.Account,
		Socket:      conn,
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
			zap.AddCallerSkip(3),
			zap.Fields(zap.String("service", "msg_gateway")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	v := config.V
	otelAddr := v.GetString("otel.Addr")
	p := tracer.InitTracer("msg_gateway", otelAddr)
	defer func() { _ = p.Shutdown(context.Background()) }()
	meter.InitMeter("msg_gateway")
	meter.RegisterOnlineUsers(func(ctx context.Context, observer metric.Int64Observer) error {
		core.GlobalManager.Lock.RLock()
		count := len(core.GlobalManager.Clients)
		core.GlobalManager.Lock.RUnlock()
		observer.Observe(int64(count))
		return nil
	})
	jwtKey = []byte(v.GetString("jwt.Key"))
	if len(jwtKey) < minJWTKeyLen {
		klog.Warnf("jwt.Key 长度不足 %d 字节，生产环境请更换为强随机密钥", minJWTKeyLen)
	}
	core.InitPushSecret(v.GetString("jwt.Key"))
	gatewayAddr := v.GetString("gateway.addr")
	core.InitManager(gatewayAddr)
	kafkaWriter, err := infra.ConnectKafkaProducer(v)
	if err != nil {
		klog.Fatalf("连接Kafka失败: %v", err)
	}
	core.InitKafkaProducer(kafkaWriter)
	rpc.Connect(v)
	garnetClient, err := infra.ConnectRedis(v)
	if err != nil {
		klog.Warnf("连接Garnet失败，将降级使用RPC管理在线状态: %v", err)
	} else {
		core.InitRedis(garnetClient)
		klog.Infof("Garnet连接成功，在线状态将直接写入Garnet")
	}
	botReplyReader, err := infra.ConnectKafkaConsumerGroup(v, "bot-reply-group", "bot-reply-topic")
	if err != nil {
		klog.Fatalf("连接Bot回复Kafka ConsumerGroup失败: %v", err)
	}
	go core.GlobalManager.Start()
	core.StartBotReplyConsumer(botReplyReader)
	wsHandler := otelhttp.NewHandler(http.HandlerFunc(handleWebSocket), "/ws")
	http.Handle("/ws", corsMiddleware(wsHandler))
	http.Handle("/push", corsMiddleware(http.HandlerFunc(core.HandlePush)))
	klog.Infof("msg_gateway 启动, gatewayAddr=%s", gatewayAddr)
	if err := http.ListenAndServe(gatewayAddr, nil); err != nil {
		klog.Fatalf("服务启动失败: %v", err)
	}
}
