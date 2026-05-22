package config

import (
	"answer_pkg/config"

	"github.com/spf13/viper"
)

func GetConfig() {
	viper.SetDefault("jwt.Key", "Airiseina")
	viper.SetDefault("etcd.Addr", "127.0.0.1:2379")
	viper.SetDefault("otel.Addr", "localhost:4317")
	viper.SetDefault("seaweedfs.filer_url", "http://127.0.0.1:8888")
	viper.SetDefault("seaweedfs.base_path", "/chat")
	config.LoadConfig()
}
