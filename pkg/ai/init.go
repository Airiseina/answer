package ai

import (
	"github.com/spf13/viper"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
)

var (
	douBaoModel  string
	douBaoClient *arkruntime.Client
)

func AiInit(v *viper.Viper) {
	apiKey := v.GetString("embedding.api_key")
	baseURL := v.GetString("embedding.base_url")
	douBaoModel = v.GetString("embedding.model")
	if baseURL != "" {
		douBaoClient = arkruntime.NewClientWithApiKey(apiKey, arkruntime.WithBaseUrl(baseURL))
	} else {
		douBaoClient = arkruntime.NewClientWithApiKey(apiKey)
	}
}
