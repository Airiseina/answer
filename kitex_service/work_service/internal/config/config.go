package config

import (
	pkgconfig "github.com/Airiseina/answer/pkg/config"
	"github.com/spf13/viper"
)

var V *viper.Viper

func GetConfig() {
	V = pkgconfig.LoadConfig()
	V.SetDefault("mysql.host", "localhost")
	V.SetDefault("mysql.port", "3306")
	V.SetDefault("mysql.user", "root")
	V.SetDefault("mysql.password", "123456")
	V.SetDefault("mysql.name", "answer")
	V.SetDefault("etcd.Addr", "127.0.0.1:2379")
	V.SetDefault("otel.Addr", "localhost:4317")
	V.SetDefault("gateway.addr", "127.0.0.1:8082")
	V.SetDefault("kafka.brokers", "127.0.0.1:9094")
	V.SetDefault("mcp.mem0_url", "http://localhost:9004/sse")
	// knowledge已从MCP改为直接RPC调用knowledge_service
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
	V.SetDefault("image.max_size", 5242880) // 图片最大5MB
	V.SetDefault("ai.user_bot.safety_prompt_file", "prompt/user_bot_safety_prompt.md")
	V.SetDefault("ai.summary.daily_prompt_file", "prompt/daily_summary.md")   // 日总结提示词
	V.SetDefault("ai.summary.weekly_prompt_file", "prompt/weekly_summary.md") // 周总结提示词
	V.SetDefault("ai.agent_graph.enabled", true)                              // 多 Agent 协作编排开关
	V.SetDefault("ai.agent_graph.timeout_seconds", 240)                       // 编排图单次执行总超时(秒)
	V.SetDefault("ai.agent_graph.max_agent_nodes", 12)                        // 编排图单次执行的 agent 节点上限
	V.SetDefault("ai.bot_task.max_concurrent", 10)                            // Bot 任务全局并发上限（跨 Bot 共享信号量）
	V.SetDefault("ai.bot_task.per_bot_queue", 50)                             // 单个 Bot 的任务排队长度
	V.SetDefault("summary.tick_interval", 60)                                 // 总结调度扫描间隔(秒)
	V.SetDefault("summary.send_hour", 21)                                     // 每日/每周总结发送时刻(小时, 本地时区)
	V.SetDefault("summary.stagger_minutes", 30)                               // 用户分桶错峰窗口(分钟)
	V.SetDefault("summary.page_size", 500)                                    // 调度每页枚举的用户数
	V.SetDefault("redis.addr", "127.0.0.1:6379")                              // 总结调度跨实例去重锁
	V.SetDefault("ragas_eval.url", "http://localhost:8090")                   // RAGAS评估服务地址
}
