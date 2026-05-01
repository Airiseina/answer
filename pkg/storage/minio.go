package storage

import (
	"answer/pkg/logger"
	"context"
	"io"

	"go.uber.org/zap"
)

type Minio struct {
	Client *minio.Client
}

func NewMinio(client *minio.Client) *Minio {
	return &Minio{client}
}

func (client *Minio) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64) error {
	_, err := client.Client.PutObject(ctx,
		bucketName,
		objectName,
		reader,
		objectSize,
		minio.PutObjectOptions{
			ContentType: "application/octet-stream",
		},
	)
	if err != nil {
		logger.Error("存储文件数据失败", zap.Error(err))
		return err
	}
	return nil
}

func (client *Minio) GetObject(ctx context.Context, bucketName, objectName string) ([]byte, error) {
	object, err := client.Client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		logger.Error("获取对象失败", zap.Error(err))
		return nil, err
	}
	defer object.Close()
	data, err := io.ReadAll(object)
	if err != nil {
		logger.Error("读取对象失败", zap.Error(err))
		return nil, err
	}
	return data, nil
}
