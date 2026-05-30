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
	V.SetDefault("kafka.brokers", "localhost:9094")
	V.SetDefault("kafka.topic.doc_parse", "doc-parse")
	V.SetDefault("kafka.group.knowledge", "knowledge-group")
	V.SetDefault("qdrant.host", "localhost")
	V.SetDefault("qdrant.port", 6334)
}
