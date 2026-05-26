package storage

import "github.com/Airiseina/answer/pkg/logger"

func Init() {
	InitSeaweedFS()
	if !HealthCheck() {
		logger.Fatal("SeaweedFS健康检查失败")
	}
}
