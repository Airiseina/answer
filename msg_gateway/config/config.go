package config

import (
	"github.com/Airiseina/answer/pkg/config"
	"github.com/Airiseina/answer/pkg/storage"

	"github.com/spf13/viper"
)

func GetConfig() {
	viper.SetDefault("jwt.Key", "Airiseina")
	viper.SetDefault("etcd.Addr", "127.0.0.1:2379")
	viper.SetDefault("otel.Addr", "localhost:4317")
	viper.SetDefault("gateway.addr", "localhost:8082")
	viper.SetDefault("seaweedfs.filer_url", "http://127.0.0.1:8888")
	viper.SetDefault("seaweedfs.base_path", "/chat")
	viper.SetDefault("seaweedfs.public_url", "/files")
	viper.SetDefault("redis.addr", "127.0.0.1:6379")
	config.LoadConfig()
	storage.FilerURL = viper.GetString("seaweedfs.filer_url")
	storage.BasePath = viper.GetString("seaweedfs.base_path")
	storage.PublicURL = viper.GetString("seaweedfs.public_url")
	if storage.FilerURL == "" {
		storage.FilerURL = "http://127.0.0.1:8888"
	}
	if storage.BasePath == "" {
		storage.BasePath = "/chat"
	}
	if storage.PublicURL == "" {
		storage.PublicURL = "/files"
	}
}
