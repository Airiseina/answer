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
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

// chatCli chat_service 的 RPC 客户端
// group_service 通过此客户端调用 chat_service 的会话管理接口
// 实现跨服务数据同步：群组变更 → 会话数据同步
var chatCli chatservice.Client

// ConnectChatService 初始化 chat_service 的 RPC 客户端
// 必须在服务启动时调用，传入 etcd 服务发现解析器
//
// 为什么 group_service 需要调用 chat_service：
//   - 统一会话模型下，群聊会话的成员数据必须与群组成员数据保持一致
//   - group_service 是群组治理的权威源（Source of Truth）
//   - 当群组成员变更时，group_service 负责通知 chat_service 同步更新会话成员
//   - 这确保消息推送能准确路由到所有当前群成员
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
	)
	if err != nil {
		klog.Fatalf("初始化 chat_service 客户端失败: %v", err)
	}
	chatCli = c
}

// CreateConversation 调用 chat_service 创建群聊会话
// 在创建群组后调用，同步创建对应的群聊会话
//
// 参数:
//   - ctx: 上下文，用于链路追踪和超时控制
//   - name: 会话名称，通常与群名一致
//   - memberIDs: 成员用户ID列表，第一个元素为创建者
//   - groupID: 群组ID，写入会话记录用于前端关联
//
// 返回值: 会话ID（后续邀请/踢人时需要使用），错误信息
func CreateConversation(ctx context.Context, name string, memberIDs []int64, groupID int64) (int64, error) {
	resp, err := chatCli.CreateConversation(ctx, &chat.CreateConversationReq{
		Name:      name,
		MemberIds: memberIDs,
		GroupId:   &groupID,
	})
	if err != nil {
		return 0, err
	}
	if !resp.Success {
		return 0, fmt.Errorf("创建会话失败: chat_service 返回 success=false")
	}
	return resp.ConversationId, nil
}

// AddConversationMembers 调用 chat_service 添加会话成员
// 在邀请成员入群后调用，同步更新会话成员列表
//
// 为什么邀请入群后需要同步：
//   - 新成员加入群组后，应该能收到该群的消息推送
//   - 如果不同步 conversation_member，新成员不在会话成员列表中
//   - SendMessage 查询 GetConversationMembers 时会遗漏新成员
func AddConversationMembers(ctx context.Context, conversationID int64, memberIDs []int64) error {
	resp, err := chatCli.AddConversationMembers(ctx, &chat.AddConversationMembersReq{
		ConversationId: conversationID,
		MemberIds:      memberIDs,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("添加会话成员失败: chat_service 返回 success=false")
	}
	return nil
}

// RemoveConversationMembers 调用 chat_service 移除会话成员
// 在踢出成员后调用，同步更新会话成员列表
//
// 为什么踢出后需要同步：
//   - 被踢出的成员不应再收到该群的消息推送
//   - 如果不同步移除 conversation_member，被踢用户仍在会话成员列表中
//   - SendMessage 会继续向已退群的用户推送消息
func RemoveConversationMembers(ctx context.Context, conversationID int64, memberIDs []int64) error {
	resp, err := chatCli.RemoveConversationMembers(ctx, &chat.RemoveConversationMembersReq{
		ConversationId: conversationID,
		MemberIds:      memberIDs,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("移除会话成员失败: chat_service 返回 success=false")
	}
	return nil
}

func DeleteConversation(ctx context.Context, conversationID int64) error {
	resp, err := chatCli.DeleteConversation(ctx, &chat.DeleteConversationReq{
		ConversationId: conversationID,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("删除会话失败: chat_service 返回 success=false")
	}
	return nil
}
