package config

import (
	pkgconfig "github.com/Airiseina/answer/pkg/config"
	"github.com/spf13/viper"
)

var V *viper.Viper

func GetConfig() {
	V = pkgconfig.LoadConfig()
	V.SetDefault("postgres.host", "localhost")
	V.SetDefault("postgres.port", "5432")
	V.SetDefault("postgres.user", "postgres")
	V.SetDefault("postgres.password", "123456")
	V.SetDefault("postgres.dbname", "answer_chat")
	V.SetDefault("postgres.sslmode", "disable")
	V.SetDefault("redis.addr", "127.0.0.1:6379")
	V.SetDefault("etcd.Addr", "127.0.0.1:2379")
	V.SetDefault("otel.Addr", "localhost:4317")
	// ClickHouse 冷库配置
	V.SetDefault("clickhouse.addr", "localhost:9000")
	V.SetDefault("clickhouse.username", "default")
	V.SetDefault("clickhouse.password", "")
	V.SetDefault("clickhouse.database", "answer_cold")
	// 冷热分离配置
	V.SetDefault("cold_storage.enabled", false)
	V.SetDefault("cold_storage.hot_months", 6)
	V.SetDefault("cold_storage.archive_cron", "0 0 3 * * ?") // 每天凌晨3点归档
	// 消息缓存配置
	V.SetDefault("cache.message_count", 50) // 每个会话缓存最近N条消息
}
