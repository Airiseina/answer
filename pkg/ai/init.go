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
	douBaoModel = v.GetString("embedding.model")
	douBaoClient = arkruntime.NewClientWithApiKey(apiKey)
}
