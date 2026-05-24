package config

import (
	"strings"

	"github.com/spf13/viper"
)

func GetConfig() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	viper.SetDefault("mysql.host", "localhost")
	viper.SetDefault("mysql.port", "3306")
	viper.SetDefault("mysql.user", "root")
	viper.SetDefault("mysql.password", "123456")
	viper.SetDefault("mysql.name", "answer")
	viper.SetDefault("etcd.Addr", "127.0.0.1:2379")
	viper.SetDefault("otel.Addr", "localhost:4317")
	viper.SetDefault("ai.system_bot_name", "AIM 助手")
	viper.SetDefault("ai.system_bot_prompt", "你是AIM助手，一个智能AI助手。你可以回答问题、总结对话、查询天气、翻译文本等。请用中文回复。")
	viper.SetDefault("ai.system_bot_model", "glm-4-flash")
	viper.SetDefault("ai.system_bot_api_key", "")
}
