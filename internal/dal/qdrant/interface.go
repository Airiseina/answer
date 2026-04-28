package qdrant

import (
	"context"
)

type ServiceQdrant interface {
	CreateVectors(ctx context.Context, vectorSize int) error
	InsertVectors(ctx context.Context, sessionId uint, text []string, vectors [][]float32) error
}
