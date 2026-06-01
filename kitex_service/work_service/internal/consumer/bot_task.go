package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Airiseina/answer/kitex_service/work_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/work_service/rpc"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/segmentio/kafka-go"
)

const (
	maxConcurrentBots = 10
	perBotQueueSize   = 50
	botIdleTimeout    = 10 * time.Minute
	handleTimeout     = 300 * time.Second
)

type FlexInt int64

func (f *FlexInt) UnmarshalJSON(data []byte) error {
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	parsed, err := n.Int64()
	if err != nil {
		return fmt.Errorf("cannot convert %s to int64", n.String())
	}
	*f = FlexInt(parsed)
	return nil
}

type BotTask struct {
	BotID          FlexInt `json:"bot_id"`
	ConversationID FlexInt `json:"conversation_id"`
	SenderID       FlexInt `json:"sender_id"`
	Content        string  `json:"content"`
	QuoteMsgID     FlexInt `json:"quote_msg_id"`
}

type BotTaskConsumer struct {
	reader      *kafka.Reader
	workService *service.WorkService
	botChans    sync.Map
	sem         chan struct{}
	mu          sync.Mutex
}

func NewBotTaskConsumer(reader *kafka.Reader, workService *service.WorkService) *BotTaskConsumer {
	return &BotTaskConsumer{
		reader:      reader,
		workService: workService,
		sem:         make(chan struct{}, maxConcurrentBots),
	}
}

func (c *BotTaskConsumer) Start(ctx context.Context) {
	go func() {
		for {
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				klog.Errorf("Kafka FetchMessage失败: %v", err)
				time.Sleep(time.Second)
				continue
			}
			c.routeMessage(ctx, msg)
		}
	}()
	klog.Infof("Bot任务Kafka消费者已启动")
}

func (c *BotTaskConsumer) getOrCreateBotChan(botID int64) chan kafka.Message {
	if ch, ok := c.botChans.Load(botID); ok {
		return ch.(chan kafka.Message)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ch, ok := c.botChans.Load(botID); ok {
		return ch.(chan kafka.Message)
	}
	ch := make(chan kafka.Message, perBotQueueSize)
	c.botChans.Store(botID, ch)
	go c.botWorker(botID, ch)
	return ch
}

func (c *BotTaskConsumer) botWorker(botID int64, ch chan kafka.Message) {
	idleTimer := time.NewTimer(botIdleTimeout)
	defer idleTimer.Stop()
	for {
		select {
		case msg := <-ch:
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(botIdleTimeout)
			select {
			case c.sem <- struct{}{}:
				c.processMessage(msg)
				<-c.sem
			default:
				c.processMessage(msg)
			}
		case <-idleTimer.C:
			c.mu.Lock()
			select {
			case msg := <-ch:
				c.mu.Unlock()
				idleTimer.Reset(botIdleTimeout)
				select {
				case c.sem <- struct{}{}:
					c.processMessage(msg)
					<-c.sem
				default:
					c.processMessage(msg)
				}
			default:
				c.botChans.Delete(botID)
				c.mu.Unlock()
				return
			}
		}
	}
}

func (c *BotTaskConsumer) routeMessage(ctx context.Context, msg kafka.Message) {
	var task BotTask
	if err := json.Unmarshal(msg.Value, &task); err != nil {
		klog.Errorf("解析Bot任务失败[partition=%d,offset=%d]: %v", msg.Partition, msg.Offset, err)
		c.commitMessage(msg) //提交任务
		return
	}
	ch := c.getOrCreateBotChan(int64(task.BotID))
	select {
	case ch <- msg:
	case <-ctx.Done():
		return
	}
}

func (c *BotTaskConsumer) processMessage(msg kafka.Message) {
	var task BotTask
	if err := json.Unmarshal(msg.Value, &task); err != nil {
		klog.Errorf("解析Bot任务失败[partition=%d,offset=%d]: %v", msg.Partition, msg.Offset, err)
		c.commitMessage(msg)
		return
	}
	valid, ackOnFailure := c.validateTask(context.Background(), &task)
	if !valid {
		if ackOnFailure {
			c.commitMessage(msg)
		}
		return
	}
	klog.Infof("开始处理Bot任务: botId=%d, convId=%d, senderId=%d", task.BotID, task.ConversationID, task.SenderID)

	handleCtx, handleCancel := context.WithTimeout(context.Background(), handleTimeout)
	defer handleCancel()

	_, err := c.workService.HandleMessage(handleCtx, int64(task.BotID), int64(task.ConversationID), int64(task.SenderID), task.Content, int64(task.QuoteMsgID))
	if err != nil {
		if errors.Is(handleCtx.Err(), context.DeadlineExceeded) {
			klog.Errorf("Bot[%d]处理消息超时(>%v): convId=%d, senderId=%d", task.BotID, handleTimeout, task.ConversationID, task.SenderID)
		} else {
			klog.Errorf("Bot[%d]处理消息失败: %v", task.BotID, err)
		}
	}
	c.commitMessage(msg)
}

func (c *BotTaskConsumer) commitMessage(msg kafka.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.reader.CommitMessages(ctx, msg); err != nil {
		klog.Errorf("提交Kafka消息偏移量失败[partition=%d,offset=%d]: %v", msg.Partition, msg.Offset, err)
	}
}

func (c *BotTaskConsumer) validateTask(ctx context.Context, task *BotTask) (valid bool, ackOnFailure bool) {
	members, err := rpc.GetConversationMembers(ctx, int64(task.ConversationID))
	if err != nil {
		klog.Errorf("查询会话[%d]成员失败: %v", task.ConversationID, err)
		return false, false
	}
	senderInConv := false
	for _, m := range members {
		if m == int64(task.SenderID) {
			senderInConv = true
			break
		}
	}
	if !senderInConv {
		klog.Errorf("鉴权失败: 发送者[%d]不在会话[%d]成员列表中", task.SenderID, task.ConversationID)
		return false, true
	}
	return true, false
}
