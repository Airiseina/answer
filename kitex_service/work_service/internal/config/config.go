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
	viper.SetDefault("redis.addr", "127.0.0.1:6379")
}
