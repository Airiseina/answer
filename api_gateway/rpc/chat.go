package rpc

import (
	"context"
	"time"

	chat "chat_service/kitex_gen/chat"
	"chat_service/kitex_gen/chat/chatservice"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

// chatCli chat_service 的 Kitex RPC 客户端实例
// 在 ConnectChatService 中初始化，后续通过封装函数调用
var chatCli chatservice.Client

// ConnectChatService 初始化 chat_service 的 RPC 客户端
// 参数 r: 服务发现解析器，通常基于注册中心（如 Nacos/ETCD）实现
// 客户端配置：
//   - 服务发现: 通过 resolver 动态解析服务地址
//   - 链路追踪: 集成 OpenTelemetry
//   - 失败重试: 最多重试 3 次，固定间隔 100ms
//   - RPC 超时: 5 秒
//   - 熔断: 错误率超过 10% 且采样数 >= 10 时触发熔断
//   - 负载均衡: 加权轮询
func ConnectChatService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(3)
	fp.WithFixedBackOff(100)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.1, // 错误率阈值 10%
		MinSample: 10,  // 最小采样数
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
		hlog.Fatalf("初始化ChatService客户端失败:%v", err)
	}
	chatCli = c
}

// GetHistory 调用 chat_service 拉取历史消息
func GetHistory(ctx context.Context, req *chat.GetHistoryReq) (*chat.GetHistoryRes, error) {
	return chatCli.GetHistory(ctx, req)
}

// GetConversations 调用 chat_service 查询用户会话列表
func GetConversations(ctx context.Context, req *chat.GetConversationsReq) (*chat.GetConversationsRes, error) {
	return chatCli.GetConversations(ctx, req)
}
