package rpc

import (
	"context"
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
		klog.Fatal("连接chatrpc失败")
	}
	chatCli = c
}

func SendMessage(ctx context.Context, req *chat.SendMessageReq) (*chat.SendMessageRes, error) {
	return chatCli.SendMessage(ctx, req)
}

func SetOnline(ctx context.Context, req *chat.SetOnlineReq) (*chat.CommonRes, error) {
	return chatCli.SetOnline(ctx, req)
}

func SetOffline(ctx context.Context, req *chat.SetOfflineReq) (*chat.CommonRes, error) {
	return chatCli.SetOffline(ctx, req)
}

func RenewOnline(ctx context.Context, req *chat.RenewOnlineReq) (*chat.CommonRes, error) {
	return chatCli.RenewOnline(ctx, req)
}

func GetOnlineStatus(ctx context.Context, req *chat.GetOnlineStatusReq) (*chat.GetOnlineStatusRes, error) {
	return chatCli.GetOnlineStatus(ctx, req)
}

func MarkRead(ctx context.Context, req *chat.MarkReadReq) (*chat.MarkReadRes, error) {
	return chatCli.MarkRead(ctx, req)
}

// GetConversationMembers 调用 chat_service 查询会话成员列表
// 用于 typing 等场景下获取推送目标用户
func GetConversationMembers(ctx context.Context, req *chat.GetConversationMembersReq) (*chat.GetConversationMembersRes, error) {
	return chatCli.GetConversationMembers(ctx, req)
}

// RecallMessage 调用 chat_service 撤回消息
func RecallMessage(ctx context.Context, req *chat.RecallMessageReq) (*chat.RecallMessageRes, error) {
	return chatCli.RecallMessage(ctx, req)
}

// EditMessage 调用 chat_service 编辑消息
func EditMessage(ctx context.Context, req *chat.EditMessageReq) (*chat.EditMessageRes, error) {
	return chatCli.EditMessage(ctx, req)
}

// SyncMessages 调用 chat_service 同步消息
// 客户端断线重连后，携带每个会话的本地最大 seq，服务端返回增量消息
func SyncMessages(ctx context.Context, req *chat.SyncMessagesReq) (*chat.SyncMessagesRes, error) {
	return chatCli.SyncMessages(ctx, req)
}
