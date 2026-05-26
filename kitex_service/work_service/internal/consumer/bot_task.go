package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Airiseina/answer/kitex_service/work_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/work_service/rpc"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/redis/go-redis/v9"
)

const (
	botTaskStream     = "bot:task:stream"
	consumerGroup     = "bot-worker-group"
	consumerName      = "worker-1"
	pendingIdleTime   = 5 * time.Minute
	maxConcurrentBots = 10
	perBotQueueSize   = 50
	botIdleTimeout    = 10 * time.Minute
)

type BotTask struct {
	BotID          int64  `json:"bot_id"`
	ConversationID int64  `json:"conversation_id"`
	SenderID       int64  `json:"sender_id"`
	Content        string `json:"content"`
}

type BotTaskConsumer struct {
	rdb         *redis.Client
	workService *service.WorkService
	botChans    sync.Map
	sem         chan struct{}
	mu          sync.Mutex
}

func NewBotTaskConsumer(rdb *redis.Client, workService *service.WorkService) *BotTaskConsumer {
	return &BotTaskConsumer{
		rdb:         rdb,
		workService: workService,
		sem:         make(chan struct{}, maxConcurrentBots),
	}
}

func (c *BotTaskConsumer) Start(ctx context.Context) {
	c.ensureGroup(ctx)
	go c.consumeNew(ctx)
	go c.consumePending(ctx)
}

func (c *BotTaskConsumer) ensureGroup(ctx context.Context) {
	err := c.rdb.XGroupCreateMkStream(ctx, botTaskStream, consumerGroup, "0").Err()
	if err != nil {
		klog.Infof("创建消费者组[%s]（可能已存在）: %v", consumerGroup, err)
	}
}

func (c *BotTaskConsumer) getOrCreateBotChan(ctx context.Context, botID int64) chan redis.XMessage {
	if ch, ok := c.botChans.Load(botID); ok {
		return ch.(chan redis.XMessage)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ch, ok := c.botChans.Load(botID); ok {
		return ch.(chan redis.XMessage)
	}
	ch := make(chan redis.XMessage, perBotQueueSize)
	c.botChans.Store(botID, ch)
	go c.botWorker(ctx, botID, ch)
	return ch
}

func (c *BotTaskConsumer) botWorker(ctx context.Context, botID int64, ch chan redis.XMessage) {
	idleTimer := time.NewTimer(botIdleTimeout)
	defer idleTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
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
				c.processMessage(ctx, msg)
				<-c.sem
			case <-ctx.Done():
				return
			}
		case <-idleTimer.C:
			c.mu.Lock()
			select {
			case msg := <-ch:
				c.mu.Unlock()
				idleTimer.Reset(botIdleTimeout)
				select {
				case c.sem <- struct{}{}:
					c.processMessage(ctx, msg)
					<-c.sem
				case <-ctx.Done():
					return
				}
			default:
				c.botChans.Delete(botID)
				c.mu.Unlock()
				return
			}
		}
	}
}

func (c *BotTaskConsumer) routeMessage(ctx context.Context, msg redis.XMessage) {
	taskJSON, ok := msg.Values["task"].(string)
	if !ok {
		klog.Errorf("消息[%s]缺少task字段", msg.ID)
		c.ack(ctx, msg.ID)
		return
	}
	var task BotTask
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		klog.Errorf("解析Bot任务失败[%s]: %v", msg.ID, err)
		c.ack(ctx, msg.ID)
		return
	}

	ch := c.getOrCreateBotChan(ctx, task.BotID)
	select {
	case ch <- msg:
	case <-ctx.Done():
		return
	}
}

func (c *BotTaskConsumer) consumeNew(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: consumerName,
			Streams:  []string{botTaskStream, ">"},
			Count:    1,
			Block:    5 * time.Second,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			klog.Errorf("XReadGroup读取新消息失败: %v", err)
			time.Sleep(time.Second)
			continue
		}
		for _, stream := range streams {
			for _, msg := range stream.Messages {
				c.routeMessage(ctx, msg)
			}
		}
	}
}

func (c *BotTaskConsumer) consumePending(ctx context.Context) {
	ticker := time.NewTicker(pendingIdleTime)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		pendings, err := c.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
			Stream: botTaskStream,
			Group:  consumerGroup,
			Start:  "-",
			End:    "+",
			Count:  10,
			Idle:   pendingIdleTime,
		}).Result()
		if err != nil {
			if !errors.Is(err, redis.Nil) {
				klog.Errorf("XPendingExt查询失败: %v", err)
			}
			continue
		}
		for _, p := range pendings {
			msgs, err := c.rdb.XClaim(ctx, &redis.XClaimArgs{
				Stream:   botTaskStream,
				Group:    consumerGroup,
				Consumer: consumerName,
				MinIdle:  pendingIdleTime,
				Messages: []string{p.ID},
			}).Result()
			if err != nil {
				klog.Errorf("XClaim认领消息[%s]失败: %v", p.ID, err)
				continue
			}
			for _, msg := range msgs {
				c.routeMessage(ctx, msg)
			}
		}
	}
}

func (c *BotTaskConsumer) processMessage(ctx context.Context, msg redis.XMessage) {
	taskJSON, ok := msg.Values["task"].(string)
	if !ok {
		klog.Errorf("消息[%s]缺少task字段", msg.ID)
		c.ack(ctx, msg.ID)
		return
	}
	var task BotTask
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		klog.Errorf("解析Bot任务失败[%s]: %v", msg.ID, err)
		c.ack(ctx, msg.ID)
		return
	}

	valid, ackOnFailure := c.validateTask(ctx, &task)
	if !valid {
		if ackOnFailure {
			c.ack(ctx, msg.ID)
		}
		return
	}

	klog.Infof("开始处理Bot任务: botId=%d, convId=%d, senderId=%d", task.BotID, task.ConversationID, task.SenderID)
	_, err := c.workService.HandleMessage(ctx, task.BotID, task.ConversationID, task.SenderID, task.Content, nil)
	if err != nil {
		klog.Errorf("Bot[%d]处理消息失败: %v", task.BotID, err)
		return
	}
	c.ack(ctx, msg.ID)
}

func (c *BotTaskConsumer) validateTask(ctx context.Context, task *BotTask) (valid bool, ackOnFailure bool) {
	members, err := rpc.GetConversationMembers(ctx, task.ConversationID)
	if err != nil {
		klog.Errorf("查询会话[%d]成员失败: %v", task.ConversationID, err)
		return false, false
	}
	senderInConv := false
	for _, m := range members {
		if m == task.SenderID {
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

func (c *BotTaskConsumer) ack(ctx context.Context, msgID string) {
	_, err := c.rdb.XAck(ctx, botTaskStream, consumerGroup, msgID).Result()
	if err != nil {
		klog.Errorf("ACK消息[%s]失败: %v", msgID, err)
	}
}

func ParseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func FormatInt64(n int64) string {
	return fmt.Sprintf("%d", n)
}
