package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

func LoadConfig() {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		log.Println("未找到配置文件，先使用默认值")
	}
}

//func GetConfig() {
//	viper.SetDefault("redis.host", "localhost")
//	viper.SetDefault("redis.port", "6379")
//}
