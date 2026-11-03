package consumer

import (
	"context"
	"encoding/json"
	"runtime/debug"
	"time"

	"github.com/Airiseina/answer/kitex_service/open_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/open_service/internal/service"
	openrpc "github.com/Airiseina/answer/kitex_service/open_service/rpc"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/segmentio/kafka-go"
)

// 话题名与 msg_gateway / work_service 保持一致。
const (
	botTaskTopic  = "bot-task-topic"  // 用户消息触发 Bot（message.received 来源）
	botReplyTopic = "bot-reply-topic" // Bot 生成回复（message.reply 来源）
)

// OpenDispatchConsumer 订阅 Bot 任务与回复话题，把与开放平台应用相关的事件落库并回调。
//
// 开放平台自身的 Bot 由 open_service 管理，但消息流转仍走 msg_gateway 的通用链路，
// 这里只做「归属识别 + 事件记录」，不介入站内 Bot 的正常回复。
type OpenDispatchConsumer struct {
	taskReader  *kafka.Reader
	replyReader *kafka.Reader
	openService *service.OpenService
}

func NewOpenDispatchConsumer(taskReader, replyReader *kafka.Reader, svc *service.OpenService) *OpenDispatchConsumer {
	return &OpenDispatchConsumer{
		taskReader:  taskReader,
		replyReader: replyReader,
		openService: svc,
	}
}

func (c *OpenDispatchConsumer) Start() {
	go c.consumeTasks()
	go c.consumeReplies()
}

func (c *OpenDispatchConsumer) Stop() {
	if c.taskReader != nil {
		_ = c.taskReader.Close()
	}
	if c.replyReader != nil {
		_ = c.replyReader.Close()
	}
}

// taskMessage 是 bot-task-topic 的消息体，bot_id 为真实 Bot ID。
type taskMessage struct {
	BotID          int64  `json:"bot_id"`
	ConversationID int64  `json:"conversation_id"`
	SenderID       int64  `json:"sender_id"`
	Content        string `json:"content"`
	QuoteMsgID     int64  `json:"quote_msg_id"`
	TenantID       int64  `json:"tenant_id"`
}

// replyMessage 是 bot-reply-topic 的消息体，sender_id 为 Bot 的用户 ID。
type replyMessage struct {
	MsgID            int64   `json:"msg_id"`
	Seq              int64   `json:"seq"`
	ConversationID   int64   `json:"conversation_id"`
	ConversationType int16   `json:"conversation_type"`
	SenderID         int64   `json:"sender_id"`
	Content          string  `json:"content"`
	Timestamp        int64   `json:"timestamp"`
	MemberIDs        []int64 `json:"member_ids"`
}

func (c *OpenDispatchConsumer) consumeTasks() {
	ctx := context.Background()
	for {
		msg, err := c.taskReader.FetchMessage(ctx)
		if err != nil {
			klog.Errorf("开放平台消费bot-task失败: %v", err)
			time.Sleep(time.Second)
			continue
		}
		var task taskMessage
		if err = json.Unmarshal(msg.Value, &task); err != nil {
			klog.Errorf("解析bot-task消息失败: %v, raw: %s", err, string(msg.Value))
			_ = c.taskReader.CommitMessages(ctx, msg)
			continue
		}
		c.safeRecord(ctx, task.BotID, model.EventMessageReceived, map[string]interface{}{
			"conversation_id": task.ConversationID,
			"conversation_type": 0,
			"sender_id":         task.SenderID,
			"content":           task.Content,
			"quote_msg_id":      task.QuoteMsgID,
			"tenant_id":         task.TenantID,
		})
		_ = c.taskReader.CommitMessages(ctx, msg)
	}
}

func (c *OpenDispatchConsumer) consumeReplies() {
	ctx := context.Background()
	for {
		msg, err := c.replyReader.FetchMessage(ctx)
		if err != nil {
			klog.Errorf("开放平台消费bot-reply失败: %v", err)
			time.Sleep(time.Second)
			continue
		}
		var reply replyMessage
		if err = json.Unmarshal(msg.Value, &reply); err != nil {
			klog.Errorf("解析bot-reply消息失败: %v, raw: %s", err, string(msg.Value))
			_ = c.replyReader.CommitMessages(ctx, msg)
			continue
		}
		// 回复消息只带 Bot 用户 ID，需要反查 Bot ID 才能定位开放平台应用。
		if _, botID, isErr := openrpc.IsBot(ctx, reply.SenderID); isErr == nil && botID != 0 {
			c.safeRecord(ctx, botID, model.EventMessageReply, map[string]interface{}{
				"msg_id":            reply.MsgID,
				"conversation_id":   reply.ConversationID,
				"conversation_type": reply.ConversationType,
				"content":           reply.Content,
				"timestamp":         reply.Timestamp,
				"member_ids":        reply.MemberIDs,
			})
		}
		_ = c.replyReader.CommitMessages(ctx, msg)
	}
}

// safeRecord 包裹事件记录，避免单条异常消息导致消费协程退出。
func (c *OpenDispatchConsumer) safeRecord(ctx context.Context, botID int64, eventType string, payload map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			klog.Errorf("开放平台事件记录panic, bot_id=%d, err=%v\n%s", botID, r, string(debug.Stack()))
		}
	}()
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	// 非开放平台 Bot 会被 RecordBotEvent 内部过滤掉，这里无需重复判断。
	if err = c.openService.RecordBotEvent(ctx, botID, eventType, string(raw)); err != nil {
		klog.Warnf("开放平台事件落库失败, bot_id=%d, type=%s, err=%v", botID, eventType, err)
	}
}
