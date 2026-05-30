package config

import (
	pkgconfig "github.com/Airiseina/answer/pkg/config"
	"github.com/Airiseina/answer/pkg/storage"

	"github.com/spf13/viper"
)

var V *viper.Viper

func GetConfig() {
	V = pkgconfig.LoadConfig()
	V.SetDefault("jwt.Key", "Airiseina")
	V.SetDefault("etcd.Addr", "127.0.0.1:2379")
	V.SetDefault("otel.Addr", "localhost:4317")
	V.SetDefault("gateway.addr", "localhost:8082")
	V.SetDefault("seaweedfs.filer_url", "http://127.0.0.1:8888")
	V.SetDefault("seaweedfs.base_path", "/chat")
	V.SetDefault("seaweedfs.public_url", "/files")
	V.SetDefault("kafka.brokers", []string{"127.0.0.1:9094"})
	storage.FilerURL = V.GetString("seaweedfs.filer_url")
	storage.BasePath = V.GetString("seaweedfs.base_path")
	storage.PublicURL = V.GetString("seaweedfs.public_url")
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
