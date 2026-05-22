package mq

import (
	"answer_pkg/logger"
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

var TopicName = "answer"

func KafkaInit() {
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}
	conn, err := dialer.DialContext(context.Background(), "tcp", "localhost:9094")
	if err != nil {
		logger.Fatal("连接消息队列失败", zap.String("err", err.Error()))
	}
	defer conn.Close()
	err = conn.CreateTopics(kafka.TopicConfig{
		Topic:             TopicName,
		NumPartitions:     3,
		ReplicationFactor: 2,
	})
	if err != nil {
		logger.Fatal("创建主题失败", zap.String("err", err.Error()))
	}
}
