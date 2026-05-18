package config

import (
	"answer_pkg/config"

	"github.com/spf13/viper"
)

func GetConfig() {
	viper.SetDefault("postgres.host", "localhost")
	viper.SetDefault("postgres.port", "5432")
	viper.SetDefault("postgres.user", "postgres")
	viper.SetDefault("postgres.password", "123456")
	viper.SetDefault("postgres.dbname", "answer_chat")
	viper.SetDefault("postgres.sslmode", "disable")
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("etcd.Addr", "127.0.0.1:2379")
	viper.SetDefault("otel.Addr", "localhost:4317")
	config.LoadConfig()
}
