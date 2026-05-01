package config

import (
	"answer_pkg/config"

	"github.com/spf13/viper"
)

func GetConfig() {
	viper.SetDefault("jwt.Key", "Airiseina")
	config.LoadConfig()
}
