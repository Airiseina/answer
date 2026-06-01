package rpc

import (
	"context"
	"time"

	bot "github.com/Airiseina/answer/kitex_service/bot_service/kitex_gen/bot"
	"github.com/Airiseina/answer/kitex_service/bot_service/kitex_gen/bot/botservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

var botCli botservice.Client

func ConnectBotService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(3)
	fp.WithFixedBackOff(100)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.1,
		MinSample: 10,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("botservice", cbConfig)
	c, err := botservice.NewClient("botservice",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(5*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		klog.Fatalf("连接bot_service失败: %v", err)
	}
	botCli = c
}

func CreateBot(ctx context.Context, req *bot.CreateBotReq) (*bot.CreateBotRes, error) {
	return botCli.CreateBot(ctx, req)
}

func GetUserBots(ctx context.Context, req *bot.GetUserBotsReq) (*bot.GetUserBotsRes, error) {
	return botCli.GetUserBots(ctx, req)
}

func UpdateBot(ctx context.Context, req *bot.UpdateBotReq) (*bot.CommonRes, error) {
	return botCli.UpdateBot(ctx, req)
}

func DeleteBot(ctx context.Context, req *bot.DeleteBotReq) (*bot.CommonRes, error) {
	return botCli.DeleteBot(ctx, req)
}

func AddBotToConversation(ctx context.Context, req *bot.AddBotToConversationReq) (*bot.AddBotToConversationRes, error) {
	return botCli.AddBotToConversation(ctx, req)
}

func GetSystemBot(ctx context.Context) (*bot.GetSystemBotRes, error) {
	return botCli.GetSystemBot(ctx)
}

func GetBot(ctx context.Context, req *bot.GetBotReq) (*bot.GetBotRes, error) {
	return botCli.GetBot(ctx, req)
}

func CreateMcpServer(ctx context.Context, req *bot.CreateMcpServerReq) (*bot.CreateMcpServerRes, error) {
	return botCli.CreateMcpServer(ctx, req)
}

func GetBotMcpServers(ctx context.Context, req *bot.GetBotMcpServersReq) (*bot.GetBotMcpServersRes, error) {
	return botCli.GetBotMcpServers(ctx, req)
}

func UpdateMcpServer(ctx context.Context, req *bot.UpdateMcpServerReq) (*bot.CommonRes, error) {
	return botCli.UpdateMcpServer(ctx, req)
}

func DeleteMcpServer(ctx context.Context, req *bot.DeleteMcpServerReq) (*bot.CommonRes, error) {
	return botCli.DeleteMcpServer(ctx, req)
}
