package ai

import (
	"context"
	"errors"

	"github.com/Airiseina/answer/pkg/observability/logger"
	"go.uber.org/zap"
)

// errEmbedderNotReady 在 Embedder 未初始化时返回
var errEmbedderNotReady = errors.New("向量化客户端未初始化，请先调用 AiInit")

// toFloat32 将 eino 返回的 float64 向量转换为 Qdrant 所需的 float32 向量
func toFloat32(vec []float64) []float32 {
	out := make([]float32, len(vec))
	for i, v := range vec {
		out[i] = float32(v)
	}
	return out
}

// toFloat32Batch 批量转换 float64 向量列表为 float32
func toFloat32Batch(vectors [][]float64) [][]float32 {
	out := make([][]float32, len(vectors))
	for i, v := range vectors {
		out[i] = toFloat32(v)
	}
	return out
}

// GetEmbedding 向量化单条文本
// 兼容旧 API，内部委托给 eino Embedder
func GetEmbedding(ctx context.Context, sen string) ([]float32, error) {
	if embedder == nil {
		return nil, errEmbedderNotReady
	}
	vectors, err := embedder.EmbedStrings(ctx, []string{sen})
	if err != nil {
		logger.Error("调用 eino Embedder 失败", zap.Error(err))
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, errors.New("Embedder 返回空结果")
	}
	return toFloat32(vectors[0]), nil
}

// GetEmbeddings 批量向量化多条文本
// 关键改进：原实现串行循环 N 次 HTTP 调用，现改为单次 HTTP 批量调用
// 文档分块场景下（N=10~100+）延迟从 O(N) 降为 O(1) 次 RTT
func GetEmbeddings(ctx context.Context, msg []string) ([][]float32, error) {
	if embedder == nil {
		return nil, errEmbedderNotReady
	}
	if len(msg) == 0 {
		return nil, nil
	}
	// eino EmbedStrings 单次请求批量向量化全部文本
	vectors, err := embedder.EmbedStrings(ctx, msg)
	if err != nil {
		logger.Error("调用 eino Embedder 批量向量化失败",
			zap.Int("count", len(msg)),
			zap.Error(err))
		return nil, err
	}
	return toFloat32Batch(vectors), nil
}
