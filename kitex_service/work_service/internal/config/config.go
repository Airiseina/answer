package config

import (
	"strings"

	"github.com/spf13/viper"
)

func GetConfig() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	viper.SetDefault("etcd.Addr", "127.0.0.1:2379")
	viper.SetDefault("otel.Addr", "localhost:4317")
	viper.SetDefault("gateway.addr", "127.0.0.1:8082")
	viper.SetDefault("kafka.brokers", "127.0.0.1:9094")
	viper.SetDefault("mcp.mem0_url", "http://answer_mcp_mem0:8000/sse")
	viper.SetDefault("mcp.weather_url", "http://answer_mcp_weather:8080/sse")
	viper.SetDefault("mcp.brave_search_url", "http://answer_mcp_brave_search:8000/sse")
	viper.SetDefault("mcp.translate_url", "http://answer_mcp_translate:8000/sse")
	viper.SetDefault("mcp.timeserver_url", "http://answer_mcp_timeserver:8000/sse")
}
