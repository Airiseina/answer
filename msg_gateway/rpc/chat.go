package rpc

import (
	"context"
	"time"

	chat "chat_service/kitex_gen/chat"
	"chat_service/kitex_gen/chat/chatservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
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
		panic(err)
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

func GetOnlineStatus(ctx context.Context, req *chat.GetOnlineStatusReq) (*chat.GetOnlineStatusRes, error) {
	return chatCli.GetOnlineStatus(ctx, req)
}
