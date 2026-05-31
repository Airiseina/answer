package ai

import (
	"context"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"

	"github.com/Airiseina/answer/pkg/observability/logger"
	"go.uber.org/zap"
)

func GetEmbedding(ctx context.Context, sen string) ([]float32, error) {
	req := model.MultiModalEmbeddingRequest{
		Model: DouBaoModel,
		Input: []model.MultimodalEmbeddingInput{
			{
				Type: "text",
				Text: volcengine.String(sen),
			},
		},
	}
	res, err := douBaoClient.CreateMultiModalEmbeddings(ctx, req)
	if err != nil {
		logger.Error("调用向量模型失败", zap.Error(err))
		return nil, err
	}
	return res.Data.Embedding, nil
}

func GetEmbeddings(ctx context.Context, msg []string) ([][]float32, error) {
	var ress [][]float32
	for _, m := range msg {
		req := model.MultiModalEmbeddingRequest{
			Model: DouBaoModel,
			Input: []model.MultimodalEmbeddingInput{
				{
					Type: "text",
					Text: volcengine.String(m),
				},
			},
		}
		res, err := douBaoClient.CreateMultiModalEmbeddings(ctx, req)
		if err != nil {
			logger.Error("调用向量模型失败", zap.Error(err))
			return nil, err
		}
		ress = append(ress, res.Data.Embedding)
	}
	return ress, nil
}
