package storage

import (
	"github.com/Airiseina/answer/pkg/observability/logger"
	"github.com/spf13/viper"
)

func Init(v *viper.Viper) {
	InitSeaweedFS(v)
	if !HealthCheck() {
		logger.Fatal("SeaweedFS健康检查失败")
	}
}
