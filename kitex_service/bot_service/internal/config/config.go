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
	V.SetDefault("ai.system.bot_name", "AIM 助手")
	V.SetDefault("ai.system.bot_prompt", "你是AIM助手，一个智能AI助手。你可以回答问题、总结对话、查询天气、翻译文本等。请用中文回复。")
	V.SetDefault("ai.system.bot_prompt_file", "prompt/system_bot_prompt.md")
	V.SetDefault("ai.system.bot_skill_dir", "prompt/skills/kiana.skill")
	V.SetDefault("ai.system.bot_model", "glm-4.7-flash")
	V.SetDefault("ai.system.bot_base_url", "https://open.bigmodel.cn/api/paas/v4")
	V.SetDefault("ai.system.bot_api_key", "")
	V.SetDefault("ai.user_bot.safety_prompt_file", "prompt/user_bot_safety_prompt.md")
}
