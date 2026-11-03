package rpc

import (
	"context"
	"fmt"
	"time"

	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat"
	"github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat/chatservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

var chatCli chatservice.Client

func ConnectChatService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(2)
	fp.WithFixedBackOff(200)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.5,
		MinSample: 50,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("chatservice", cbConfig)
	c, err := chatservice.NewClient("chatservice",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(5*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		klog.Fatalf("连接chat_service失败: %v", err)
	}
	chatCli = c
}

// SentMessage 是发送成功后的回执，供开放平台回填给第三方。
type SentMessage struct {
	MsgID          int64
	ConversationID int64
	Timestamp      int64
}

func SendMessage(ctx context.Context, tenantID, senderID, conversationID, peerID int64, content string) (SentMessage, error) {
	resp, err := chatCli.SendMessage(ctx, &chat.SendMessageReq{
		SenderId:       senderID,
		ConversationId: conversationID,
		PeerId:         peerID,
		Content:        content,
		TenantId:       &tenantID,
	})
	if err != nil {
		return SentMessage{}, fmt.Errorf("发送消息失败: %w", err)
	}
	if !resp.Success {
		return SentMessage{}, fmt.Errorf("发送消息返回失败")
	}
	return SentMessage{
		MsgID:          resp.MsgId,
		ConversationID: resp.ConversationId,
		Timestamp:      resp.Timestamp,
	}, nil
}

// MessageItem 是开放平台对外暴露的消息视图。
type MessageItem struct {
	MsgID        int64
	SenderID     int64
	Conversation int64
	Content      string
	Timestamp    int64
	SenderName   string
}

func GetHistory(ctx context.Context, tenantID, userID, conversationID, beforeMsgID int64, limit int16) ([]MessageItem, error) {
	resp, err := chatCli.GetHistory(ctx, &chat.GetHistoryReq{
		UserId:         userID,
		ConversationId: conversationID,
		BeforeMsgId:    beforeMsgID,
		Limit:          limit,
		TenantId:       &tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("查询消息历史失败: %w", err)
	}
	list := make([]MessageItem, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		list = append(list, MessageItem{
			MsgID:        m.MsgId,
			SenderID:     m.SenderId,
			Conversation: m.ConversationId,
			Content:      m.Content,
			Timestamp:    m.Timestamp,
			SenderName:   m.GetSenderName(),
		})
	}
	return list, nil
}

// ConversationItem 是开放平台对外暴露的会话视图。
type ConversationItem struct {
	ConversationID int64
	Type           int16
	Name           string
	MemberIDs      []int64
	TenantID       int64
}

func GetConversations(ctx context.Context, tenantID, userID int64) ([]ConversationItem, error) {
	resp, err := chatCli.GetConversations(ctx, &chat.GetConversationsReq{
		UserId:   userID,
		TenantId: &tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("查询会话列表失败: %w", err)
	}
	list := make([]ConversationItem, 0, len(resp.Conversations))
	for _, c := range resp.Conversations {
		list = append(list, ConversationItem{
			ConversationID: c.ConversationId,
			Type:           c.Type,
			Name:           c.Name,
			MemberIDs:      c.MemberIds,
			TenantID:       c.GetTenantId(),
		})
	}
	return list, nil
}
