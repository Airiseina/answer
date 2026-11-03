package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/service"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/segmentio/kafka-go"
)

const (
	docParseTimeout   = 10 * time.Minute
	docParseRetryWait = 3 * time.Second
)

type DocParseMessage struct {
	DocID    int64 `json:"doc_id"`
	KbID     int64 `json:"kb_id"`
	TenantID int64 `json:"tenant_id"`
}

// docTask 单个待解析文档任务，携带租户以在 DAO 层强制隔离。
type docTask struct {
	TenantID int64
	DocID    int64
}

type DocParseConsumer struct {
	reader  *kafka.Reader
	service *service.KnowledgeService
	docDao  dal.DocumentDao

	mu     sync.Mutex
	queues map[int64]chan docTask
	wg     sync.WaitGroup
}

func NewDocParseConsumer(brokers []string, topic, groupID string, svc *service.KnowledgeService, docDao dal.DocumentDao) *DocParseConsumer {
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
		docDao:  docDao,
		queues:  make(map[int64]chan docTask),
	}
}

func (c *DocParseConsumer) Start() {
	ctx := context.Background()
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
		groupKey := parseMsg.KbID
		if groupKey == 0 {
			doc, err := c.docDao.GetByID(parseMsg.TenantID, parseMsg.DocID)
			if err == nil && doc.ID != 0 {
				groupKey = doc.KBID
			}
		}
		c.dispatch(groupKey, docTask{TenantID: parseMsg.TenantID, DocID: parseMsg.DocID})
		_ = c.reader.CommitMessages(ctx, msg)
	}
}

func (c *DocParseConsumer) dispatch(groupKey int64, task docTask) {
	c.mu.Lock()
	q, exists := c.queues[groupKey]
	if !exists {
		q = make(chan docTask, 256)
		c.queues[groupKey] = q
		c.wg.Add(1)
		go c.runGroupQueue(groupKey, q)
	}
	c.mu.Unlock()
	q <- task
}

func (c *DocParseConsumer) runGroupQueue(groupKey int64, q chan docTask) {
	defer c.wg.Done()
	for task := range q {
		c.processDoc(context.Background(), task.TenantID, task.DocID)
	}
	c.mu.Lock()
	delete(c.queues, groupKey)
	c.mu.Unlock()
}

func (c *DocParseConsumer) processDoc(ctx context.Context, tenantID, docID int64) {
	defer func() {
		if r := recover(); r != nil {
			klog.Errorf("文档[%d]解析panic: %v\n%s", docID, r, string(debug.Stack()))
			_ = c.docDao.UpdateStatus(tenantID, docID, model.DocStatusFailed, 0, fmt.Sprintf("解析异常: %v", r))
		}
	}()
	parseCtx, cancel := context.WithTimeout(ctx, docParseTimeout)
	defer cancel()
	if err := c.service.ProcessDocument(parseCtx, tenantID, docID); err != nil {
		klog.Errorf("文档[%d]异步解析失败: %v", docID, err)
		return
	}
	klog.Infof("文档[%d]异步解析完成", docID)
}

func (c *DocParseConsumer) Stop() {
	_ = c.reader.Close()
	c.mu.Lock()
	for _, q := range c.queues {
		close(q)
	}
	c.mu.Unlock()
	c.wg.Wait()
}
