package config

import (
	pkgconfig "github.com/Airiseina/answer/pkg/config"
	"github.com/spf13/viper"
)

var V *viper.Viper

func GetConfig() {
	V = pkgconfig.LoadConfig()
	V.SetDefault("jwt.Key", "")
	V.SetDefault("jwt.AdminKey", "")
	V.SetDefault("etcd.Addr", "127.0.0.1:2379")
	V.SetDefault("otel.Addr", "localhost:4317")
	V.SetDefault("seaweedfs.filer_url", "http://127.0.0.1:8888")
	V.SetDefault("seaweedfs.base_path", "/chat")
	V.SetDefault("seaweedfs.public_url", "/files")
	// 开放平台接口文档：网关按该路径读取 OpenAPI 规范原文并对外提供 /open/openapi.yaml。
	V.SetDefault("open.docs_path", "docs/openapi/open_platform.yaml")
}
