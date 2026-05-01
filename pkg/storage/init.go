package storage

import (
	"answer/pkg/logger"
	"context"

	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var user = viper.GetString("minio.user")
var password = viper.GetString("minio.password")
var BucketName string

func InitMinio() *minio.Client {
	client, err := minio.New("127.0.0.1:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("admin", "password", ""),
		Secure: false,
	})
	if err != nil {
		logger.Fatal("初始化minio失败", zap.Error(err))
	}
	BucketName = "rag-files"
	ctx := context.Background()
	if exists, _ := client.BucketExists(ctx, BucketName); !exists {
		if err = client.MakeBucket(ctx, BucketName, minio.MakeBucketOptions{}); err != nil {
			logger.Fatal("创建桶失败", zap.Error(err))
		}
	}
	return client
}
