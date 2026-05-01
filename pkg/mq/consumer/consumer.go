package consumer

import (
	"answer/internal/service"
	"answer/pkg/logger"
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/common/json"
	"go.uber.org/zap"
)

type UploadTaskMsg struct {
	SessionId  uint   `json:"session_id"`
	FileId     uint   `json:"file_id"`
	ObjectName string `json:"object_name"`
}

type Consumer struct {
	Reader  *kafka.Reader
	Service *service.Service
}

func NewConsumer(service *service.Service) *Consumer {
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"127.0.0.1:9094", "127.0.0.1:9095"},
		Topic:    "answer",
		GroupID:  "consumer",
		Dialer:   dialer,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})
	return &Consumer{reader, service}
}

func (consumer *Consumer) Consume(ctx context.Context) {
	for {
		m, err := consumer.Reader.FetchMessage(ctx)
		if err != nil {
			logger.Warn("消息读取失败", zap.Error(err))
			continue
		}
		var taskMsg UploadTaskMsg
		if err = json.Unmarshal(m.Value, &taskMsg); err != nil {
			logger.Error("序列化失败", zap.Error(err))
			_ = consumer.Reader.CommitMessages(ctx, m)
			continue
		}
		logger.Info("文件接收成功", zap.String("objectName", taskMsg.ObjectName))
		err = consumer.Service.FileProcess(ctx, taskMsg.SessionId, taskMsg.FileId, taskMsg.ObjectName)
		if err != nil {
			logger.Error("向量化失败", zap.Error(err))
			err = consumer.Service.FailQdrant(taskMsg.SessionId, taskMsg.FileId)
			if err != nil {
				break
			}
			_ = consumer.Reader.CommitMessages(ctx, m)
			continue
		}
		_ = consumer.Reader.CommitMessages(ctx, m)
	}
}
