package rpc

import (
	"context"
	"fmt"
	"time"

	chat "chat_service/kitex_gen/chat"
	"chat_service/kitex_gen/chat/chatservice"

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
	fp.WithMaxRetryTimes(3)
	fp.WithFixedBackOff(100)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.1,
		MinSample: 10,
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

func AddConversationMembers(ctx context.Context, conversationID int64, memberIDs []int64) error {
	resp, err := chatCli.AddConversationMembers(ctx, &chat.AddConversationMembersReq{
		ConversationId: conversationID,
		MemberIds:      memberIDs,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("添加会话成员失败: chat_service返回success=false")
	}
	return nil
}

func GetOrCreatePrivateConversation(ctx context.Context, userIDA, userIDB int64) (int64, error) {
	resp, err := chatCli.GetOrCreatePrivateConversation(ctx, &chat.GetOrCreatePrivateConversationReq{
		UserIdA: userIDA,
		UserIdB: userIDB,
	})
	if err != nil {
		return 0, fmt.Errorf("获取或创建单聊会话失败: %w", err)
	}
	if !resp.Success {
		return 0, fmt.Errorf("获取或创建单聊会话失败: chat_service返回success=false")
	}
	return resp.ConversationId, nil
}

func GetConversationMembers(ctx context.Context, conversationID int64) ([]int64, error) {
	resp, err := chatCli.GetConversationMembers(ctx, &chat.GetConversationMembersReq{
		ConversationId: conversationID,
	})
	if err != nil {
		return nil, err
	}
	return resp.MemberIds, nil
}
