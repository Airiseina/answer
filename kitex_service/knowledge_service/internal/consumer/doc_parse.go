package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/service"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/segmentio/kafka-go"
)

const (
	docParseWorkers   = 4
	docParseTimeout   = 10 * time.Minute
	docParseRetryWait = 3 * time.Second
)

type DocParseMessage struct {
	DocID int64 `json:"doc_id"`
}

type DocParseConsumer struct {
	reader  *kafka.Reader
	service *service.KnowledgeService
}

func NewDocParseConsumer(brokers []string, topic, groupID string, svc *service.KnowledgeService) *DocParseConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	return &DocParseConsumer{
		reader:  reader,
		service: svc,
	}
}

func (c *DocParseConsumer) Start() {
	ctx := context.Background()
	sem := make(chan struct{}, docParseWorkers)
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			klog.Errorf("Kafka FetchMessage失败: %v", err)
			time.Sleep(docParseRetryWait)
			continue
		}
		var parseMsg DocParseMessage
		if err := json.Unmarshal(msg.Value, &parseMsg); err != nil {
			klog.Errorf("解析文档解析消息失败: %v, raw: %s", err, string(msg.Value))
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}
		sem <- struct{}{}
		go func(docID int64, m kafka.Message) {
			defer func() { <-sem }()
			c.processDoc(ctx, docID)
			_ = c.reader.CommitMessages(ctx, m)
		}(parseMsg.DocID, msg)
	}
}

func (c *DocParseConsumer) processDoc(ctx context.Context, docID int64) {
	parseCtx, cancel := context.WithTimeout(ctx, docParseTimeout)
	defer cancel()
	if err := c.service.ProcessDocument(parseCtx, docID); err != nil {
		klog.Errorf("文档[%d]异步解析失败: %v", docID, err)
		return
	}
	klog.Infof("文档[%d]异步解析完成", docID)
}

func (c *DocParseConsumer) Stop() {
	if err := c.reader.Close(); err != nil {
		klog.Errorf("关闭Kafka Reader失败: %v", err)
	}
}

func ProduceDocParseMessage(brokers []string, topic string, docID int64) error {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
	}
	defer writer.Close()
	msg := DocParseMessage{DocID: docID}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化文档解析消息失败: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return writer.WriteMessages(ctx, kafka.Message{Value: data})
}
