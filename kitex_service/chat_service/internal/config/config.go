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
}
