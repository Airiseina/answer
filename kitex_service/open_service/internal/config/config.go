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
	V.SetDefault("redis.addr", "127.0.0.1:6379")
	V.SetDefault("kafka.brokers", "127.0.0.1:9094")
	// 开放平台默认取值（与规划 4.8 节一致），可通过配置文件覆盖。
	V.SetDefault("open.timestamp_window", 300)      // 时间戳窗口（秒）
	V.SetDefault("open.nonce_ttl", 300)             // nonce 去重 TTL（秒）
	V.SetDefault("open.default_qps", 20)            // 单 AppID 默认 QPS
	V.SetDefault("open.max_bots_per_app", 50)       // 单 App 创建 Bot 上限
	V.SetDefault("open.webhook_timeout", 10)        // 单次投递超时（秒）
	V.SetDefault("open.webhook_max_attempts", 5)    // 最大投递次数
	V.SetDefault("open.webhook_retry_backoff", []int{1, 5, 30, 120}) // 重试退避（秒）
	V.SetDefault("open.event_retention_days", 30)   // 事件保留天数
	// app_secret_key 用于加密存储 AppSecret（HMAC 验签需要还原明文，哈希无法满足）。
	V.SetDefault("open.app_secret_key", "answer-open-platform-default-key-please-change")
}
