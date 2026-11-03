package rpc

import (
	"context"
	"fmt"
	"time"

	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat"
	"github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat/chatservice"
	"github.com/Airiseina/answer/pkg/tenant"

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

// tenantFromCtx 从上下文读取租户 ID；缺失时返回 nil，由服务端决定是否放行内部调用。
func tenantFromCtx(ctx context.Context) *int64 {
	if id, ok := tenant.FromContext(ctx); ok && id > 0 {
		return &id
	}
	return nil
}

func GetConversationMembers(ctx context.Context, conversationId int64) ([]int64, error) {
	resp, err := chatCli.GetConversationMembers(ctx, &chat.GetConversationMembersReq{
		ConversationId: conversationId,
		TenantId:       tenantFromCtx(ctx),
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
		TenantId:       tenantFromCtx(ctx),
	})
	if err != nil {
		return nil, err
	}
	return resp.Messages, nil
}

func GetConversationType(ctx context.Context, conversationId, userId int64) (int16, error) {
	resp, err := chatCli.GetConversations(ctx, &chat.GetConversationsReq{
		UserId:   userId,
		TenantId: tenantFromCtx(ctx),
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

// ConversationBrief 是总结归类时间线所需的最小会话信息。
type ConversationBrief struct {
	ID        int64
	Type      int16
	Name      string
	MemberIDs []int64
}

// GetConversationBriefs 拉取用户的会话摘要（跨租户），供总结把时间线按会话归类。
// 私聊会话没有名称，由调用方结合成员列表推导对方显示名。
func GetConversationBriefs(ctx context.Context, userID int64) (map[int64]ConversationBrief, error) {
	resp, err := chatCli.GetConversations(ctx, &chat.GetConversationsReq{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("查询会话列表失败: %w", err)
	}
	briefs := make(map[int64]ConversationBrief, len(resp.Conversations))
	for _, conv := range resp.Conversations {
		briefs[conv.ConversationId] = ConversationBrief{
			ID:        conv.ConversationId,
			Type:      conv.Type,
			Name:      conv.Name,
			MemberIDs: conv.MemberIds,
		}
	}
	return briefs, nil
}

// GetOrCreatePrivateConversation 获取或创建两个用户之间的单聊会话。
// 日/周总结需要用它拿到「用户 ⇄ 系统Bot」的私聊会话，再把总结发进去。
func GetOrCreatePrivateConversation(ctx context.Context, userIDA, userIDB int64) (int64, error) {
	resp, err := chatCli.GetOrCreatePrivateConversation(ctx, &chat.GetOrCreatePrivateConversationReq{
		UserIdA:  userIDA,
		UserIdB:  userIDB,
		TenantId: tenantFromCtx(ctx),
	})
	if err != nil {
		return 0, fmt.Errorf("获取或创建单聊会话失败: %w", err)
	}
	if !resp.Success {
		return 0, fmt.Errorf("获取或创建单聊会话失败: chat_service返回success=false")
	}
	return resp.ConversationId, nil
}

// GetMessagesByTimeRange 跨会话拉取用户在时间窗内可见的消息（供日/周总结使用）。
// tenantID 传 0 表示不按租户过滤，从而聚合用户在所有租户下的会话。
// 返回消息时间线、是否因条数上限被截断。
func GetMessagesByTimeRange(ctx context.Context, tenantID, userID, startTime, endTime int64, limit int16) ([]*chat.Message, bool, error) {
	req := &chat.GetMessagesByTimeRangeReq{
		UserId:    userID,
		StartTime: startTime,
		EndTime:   endTime,
	}
	if limit > 0 {
		req.Limit = &limit
	}
	if tenantID > 0 {
		req.TenantId = &tenantID
	}
	resp, err := chatCli.GetMessagesByTimeRange(ctx, req)
	if err != nil {
		return nil, false, err
	}
	truncated := resp.Truncated != nil && resp.GetTruncated()
	return resp.Messages, truncated, nil
}
