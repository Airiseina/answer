package config

import (
	"github.com/Airiseina/answer/pkg/config"
	"github.com/spf13/viper"
)

func GetConfig() {
	viper.SetDefault("mysql.host", "localhost")
	viper.SetDefault("mysql.port", "3306")
	viper.SetDefault("mysql.user", "root")
	viper.SetDefault("mysql.password", "123456")
	viper.SetDefault("mysql.name", "answer")
	viper.SetDefault("etcd.Addr", "127.0.0.1:2379")
	viper.SetDefault("otel.Addr", "localhost:4317")
	viper.SetDefault("ai.system.bot_name", "AIM 助手")
	viper.SetDefault("ai.system.bot_prompt", "你是AIM助手，一个智能AI助手。你可以回答问题、总结对话、查询天气、翻译文本等。请用中文回复。")
	viper.SetDefault("ai.system.bot_prompt_file", "prompt/system_bot_prompt.md")
	viper.SetDefault("ai.system.bot_model", "glm-4.7-flash")
	viper.SetDefault("ai.system.bot_base_url", "https://open.bigmodel.cn/api/paas/v4")
	viper.SetDefault("ai.system.bot_api_key", "")
	config.LoadConfig()
}
