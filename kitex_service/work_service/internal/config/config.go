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
	V.SetDefault("mcp.searxng_url", "http://localhost:9008/sse")
	V.SetDefault("mcp.weather_url", "http://localhost:9001/sse")
	V.SetDefault("mcp.timeserver_url", "http://localhost:9005/sse")
	V.SetDefault("mcp.call_timeout", 20)          // MCP工具调用超时(秒)
	V.SetDefault("mcp.retry.max_attempts", 2)     // MCP调用最大重试次数(不含首次调用)
	V.SetDefault("mcp.retry.initial_interval", 1) // MCP重试初始间隔(秒)
	V.SetDefault("mcp.fallback.enabled", true)    // MCP降级开关
	V.SetDefault("seaweedfs.filer_url", "http://127.0.0.1:8888")
	V.SetDefault("seaweedfs.base_path", "/chat")
	V.SetDefault("seaweedfs.public_url", "/files")
	V.SetDefault("image.max_size", 5242880)                      // 图片最大5MB
	V.SetDefault("ai.user_bot.safety_prompt_file", "prompt/user_bot_safety_prompt.md")
	V.SetDefault("ragas_eval.url", "http://localhost:8090") // RAGAS评估服务地址
}
