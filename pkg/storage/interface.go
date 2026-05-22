package storage

import (
	"context"
	"io"
)

type Storage interface {
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, contentType string) error
	GetObject(ctx context.Context, bucketName, objectName string) ([]byte, error)
	DeleteObject(ctx context.Context, bucketName, objectName string) error
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expires int64) (string, error)
}
