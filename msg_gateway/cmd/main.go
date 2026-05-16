package main

import (
	"answer_pkg/meter"
	"answer_pkg/tracer"
	"context"
	"msg_gateway/config"
	"msg_gateway/core"
	"net/http"
	"os"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
	kitexzap "github.com/kitex-contrib/obs-opentelemetry/logging/zap"
	"github.com/spf13/viper"
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
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "未授权", http.StatusUnauthorized)
		return
	}

	klog.Infof("WS 收到 token (前20字符): %s...", tokenStr[:min(20, len(tokenStr))])

	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, &claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil {
		klog.Warnf("JWT 验证失败: %v, token长度: %d", err, len(tokenStr))
		http.Error(w, "无效的令牌: "+err.Error(), http.StatusUnauthorized)
		return
	}

	userIdFloat, ok := claims["id"].(float64)
	if !ok {
		klog.Warnf("JWT claims 中缺少 id 字段, claims: %v", claims)
		http.Error(w, "无效的令牌", http.StatusUnauthorized)
		return
	}
	userId := uint(userIdFloat)
	klog.Infof("JWT 验证成功, userId: %d", userId)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		klog.Errorf("WebSocket 升级失败: %v", err)
		return
	}
	client := &core.Client{
		Manager: &core.GlobalManager,
		UserId:  userId,
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
			zap.AddCallerSkip(2),
			zap.Fields(zap.String("service", "msg_gateway")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	otelAddr := viper.GetString("otel.Addr")
	p := tracer.InitTracer("msg_gateway", otelAddr)
	defer p.Shutdown(context.Background())
	meter.InitMeter("msg_gateway")
	meter.RegisterOnlineUsers(func(ctx context.Context, observer metric.Int64Observer) error {
		observer.Observe(int64(len(core.GlobalManager.Clients)))
		return nil
	})
	jwtKey = []byte(viper.GetString("jwt.Key"))
	klog.Infof("JWT Key: %s", string(jwtKey))
	go core.GlobalManager.Start()
	wsHandler := otelhttp.NewHandler(http.HandlerFunc(handleWebSocket), "/ws")
	http.Handle("/ws", corsMiddleware(wsHandler))
	if err := http.ListenAndServe(":8081", nil); err != nil {
		klog.Fatalf("服务启动失败: %v", err)
	}
}
