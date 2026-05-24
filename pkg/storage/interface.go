package storage

import (
	"context"
	"io"
)

type Storage interface {
	PutObject(ctx context.Context, objectName string, reader io.Reader, contentType string) error
	GetObject(ctx context.Context, objectName string) ([]byte, error)
	DeleteObject(ctx context.Context, objectName string) error
	PresignedGetObject(ctx context.Context, objectName string) (string, error)
	ObjectExists(ctx context.Context, objectName string) bool
}
