package qdrant

import (
	"context"
	"fmt"
	"github.com/Airiseina/answer/pkg/logger"

	"github.com/qdrant/go-client/qdrant"
	"go.uber.org/zap"
)

type QdrantDao struct {
	Client *qdrant.Client
}

const CollectionName = "answer"

func QdrantInit() (*qdrant.Client, error) {
	qdrantClient, err := qdrant.NewClient(&qdrant.Config{
		Host: "localhost",
		Port: 6334,
	})
	if err != nil {
		logger.Error("无法连接到向量库", zap.Error(err))
		return nil, err
	}
	return qdrantClient, nil
}

func NewQdrantDao(client *qdrant.Client) ServiceQdrant {
	return &QdrantDao{Client: client}
}

func (q *QdrantDao) CreateVectors(ctx context.Context, vectorSize int) error {
	exist, err := q.Client.CollectionExists(ctx, CollectionName)
	if err != nil {
		return err
	}
	if exist {
		err = q.Client.DeleteCollection(ctx, CollectionName)
		if err != nil {
			return fmt.Errorf("删除向量库失败: %v", err)
		}
	}
	err = q.Client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: CollectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     uint64(vectorSize),
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		logger.Error("创建向量库失败", zap.Error(err))
		return err
	}
	return nil
}

func (q *QdrantDao) InsertVectors(ctx context.Context, sessionId uint, text []string, vectors [][]float32) error {
	points := make([]*qdrant.PointStruct, 0, len(vectors))
	for i, vector := range vectors {
		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewIDNum(uint64(i + 1)),
			Vectors: qdrant.NewVectors(vector...),
			Payload: qdrant.NewValueMap(map[string]interface{}{
				"session_id": int64(sessionId),
				"content":    text[i],
			}),
		})
	}
	_, err := q.Client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: CollectionName,
		Points:         points,
	})
	if err != nil {
		logger.Error("插入数据失败", zap.Error(err))
		return err
	}
	return nil
}
