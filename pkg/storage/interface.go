package storage

import (
	"context"
	"io"
)

type Storage interface {
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64) error
	GetObject(ctx context.Context, bucketName, objectName string) ([]byte, error)
}
