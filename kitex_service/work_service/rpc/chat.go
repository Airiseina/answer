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

func SendMessage(ctx context.Context, req *chat.SendMessageReq) (*chat.SendMessageRes, error) {
	return chatCli.SendMessage(ctx, req)
}

func GetConversationMembers(ctx context.Context, conversationId int64) ([]int64, error) {
	resp, err := chatCli.GetConversationMembers(ctx, &chat.GetConversationMembersReq{
		ConversationId: conversationId,
	})
	if err != nil {
		return nil, err
	}
	return resp.MemberIds, nil
}

const (
	ConvTypePrivate int16 = 1
	ConvTypeGroup   int16 = 2
)

func GetHistory(ctx context.Context, userId, conversationId int64, limit int16) ([]*chat.Message, error) {
	resp, err := chatCli.GetHistory(ctx, &chat.GetHistoryReq{
		UserId:         userId,
		ConversationId: conversationId,
		BeforeMsgId:    0,
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	return resp.Messages, nil
}

func GetConversationType(ctx context.Context, conversationId, userId int64) (int16, error) {
	resp, err := chatCli.GetConversations(ctx, &chat.GetConversationsReq{
		UserId: userId,
	})
	if err != nil {
		return 0, err
	}
	for _, conv := range resp.Conversations {
		if conv.ConversationId == conversationId {
			return conv.Type, nil
		}
	}
	return 0, fmt.Errorf("会话[%d]未找到", conversationId)
}
