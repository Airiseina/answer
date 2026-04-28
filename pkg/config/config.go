package config

import (
	"answer/pkg/logger"

	"github.com/spf13/viper"
)

func GetConfig() {
	viper.SetDefault("mysql.host", "localhost")
	viper.SetDefault("mysql.port", "3306")
	viper.SetDefault("mysql.user", "root")
	viper.SetDefault("mysql.password", "123456")
	viper.SetDefault("mysql.name", "answer")
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6379")
	viper.SetDefault("jwt.Key", "Airiseina")
	viper.SetDefault("minio.user", "admin")
	viper.SetDefault("minio.password", "password")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		logger.Info("未找到配置文件，先使用默认值")
	}
}
