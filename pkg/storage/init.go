package storage

import "answer_pkg/logger"

func Init() {
	InitSeaweedFS()
	if !HealthCheck() {
		logger.Fatal("SeaweedFS健康检查失败")
	}
}
