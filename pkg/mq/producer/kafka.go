package producer

import (
	"answer/internal/mq"
	"answer/pkg/logger"
	"context"

	"github.com/cloudwego/hertz/pkg/common/json"
	"go.uber.org/zap"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer() *Producer {
	w := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9094", "127.0.0.1:9095"),
		Topic:    mq.TopicName,
		Balancer: &kafka.Hash{},
	}
	return &Producer{w}
}

func (p *Producer) WriteMessage(ctx context.Context, message interface{}) error {
	bytes, err := json.Marshal(message)
	if err != nil {
		logger.Error("序列化失败")
		return err
	}
	kafkaMessage := kafka.Message{
		Value: bytes,
	}
	err = p.writer.WriteMessages(ctx, kafkaMessage)
	if err != nil {
		logger.Error("消息发送失败", zap.Error(err))
		return err
	}
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
