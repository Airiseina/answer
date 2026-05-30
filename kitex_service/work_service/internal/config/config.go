package config

import (
	pkgconfig "github.com/Airiseina/answer/pkg/config"
	"github.com/spf13/viper"
)

var V *viper.Viper

func GetConfig() {
	V = pkgconfig.LoadConfig()
	V.SetDefault("etcd.Addr", "127.0.0.1:2379")
	V.SetDefault("otel.Addr", "localhost:4317")
	V.SetDefault("gateway.addr", "127.0.0.1:8082")
	V.SetDefault("kafka.brokers", "127.0.0.1:9094")
	V.SetDefault("mcp.mem0_url", "http://localhost:9004/sse")
	V.SetDefault("mcp.knowledge_url", "http://localhost:9006/sse")
	V.SetDefault("mcp.weather_url", "http://localhost:9001/sse")
	V.SetDefault("mcp.brave_search_url", "")
	V.SetDefault("mcp.translate_url", "http://localhost:9003/sse")
	V.SetDefault("mcp.timeserver_url", "http://localhost:9005/sse")
}
