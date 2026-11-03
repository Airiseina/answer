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
	fp.WithMaxRetryTimes(2)
	fp.WithFixedBackOff(200)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.5,
		MinSample: 50,
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

func PublishMarketBot(ctx context.Context, req *bot.PublishMarketBotReq) (*bot.PublishMarketBotRes, error) {
	return botCli.PublishMarketBot(ctx, req)
}

func UpdateMarketBot(ctx context.Context, req *bot.UpdateMarketBotReq) (*bot.CommonRes, error) {
	return botCli.UpdateMarketBot(ctx, req)
}

func GetMarketBot(ctx context.Context, req *bot.GetMarketBotReq) (*bot.GetMarketBotRes, error) {
	return botCli.GetMarketBot(ctx, req)
}

func ListMarketBots(ctx context.Context, req *bot.ListMarketBotsReq) (*bot.ListMarketBotsRes, error) {
	return botCli.ListMarketBots(ctx, req)
}

func ListMyMarketBots(ctx context.Context, req *bot.ListMyMarketBotsReq) (*bot.ListMyMarketBotsRes, error) {
	return botCli.ListMyMarketBots(ctx, req)
}

func InstallMarketBot(ctx context.Context, req *bot.InstallMarketBotReq) (*bot.InstallMarketBotRes, error) {
	return botCli.InstallMarketBot(ctx, req)
}

func RateMarketBot(ctx context.Context, req *bot.RateMarketBotReq) (*bot.CommonRes, error) {
	return botCli.RateMarketBot(ctx, req)
}

func OfflineMarketBot(ctx context.Context, req *bot.OfflineMarketBotReq) (*bot.CommonRes, error) {
	return botCli.OfflineMarketBot(ctx, req)
}

func ListMarketReviews(ctx context.Context, req *bot.ListMarketReviewsReq) (*bot.ListMarketReviewsRes, error) {
	return botCli.ListMarketReviews(ctx, req)
}

func ReviewMarketBot(ctx context.Context, req *bot.ReviewMarketBotReq) (*bot.CommonRes, error) {
	return botCli.ReviewMarketBot(ctx, req)
}

func SaveAgentGraph(ctx context.Context, req *bot.SaveAgentGraphReq) (*bot.SaveAgentGraphRes, error) {
	return botCli.SaveAgentGraph(ctx, req)
}

func GetAgentGraph(ctx context.Context, req *bot.GetAgentGraphReq) (*bot.GetAgentGraphRes, error) {
	return botCli.GetAgentGraph(ctx, req)
}

func ValidateAgentGraph(ctx context.Context, req *bot.ValidateAgentGraphReq) (*bot.ValidateAgentGraphRes, error) {
	return botCli.ValidateAgentGraph(ctx, req)
}

func PublishAgentGraph(ctx context.Context, req *bot.PublishAgentGraphReq) (*bot.CommonRes, error) {
	return botCli.PublishAgentGraph(ctx, req)
}

func ListAgentGraphHistory(ctx context.Context, req *bot.ListAgentGraphHistoryReq) (*bot.ListAgentGraphHistoryRes, error) {
	return botCli.ListAgentGraphHistory(ctx, req)
}

func UploadUserSkill(ctx context.Context, req *bot.UploadUserSkillReq) (*bot.UploadUserSkillRes, error) {
	return botCli.UploadUserSkill(ctx, req)
}

func ListUserSkills(ctx context.Context, req *bot.ListUserSkillsReq) (*bot.ListUserSkillsRes, error) {
	return botCli.ListUserSkills(ctx, req)
}

func GetUserSkill(ctx context.Context, req *bot.GetUserSkillReq) (*bot.GetUserSkillRes, error) {
	return botCli.GetUserSkill(ctx, req)
}

func DeleteUserSkill(ctx context.Context, req *bot.DeleteUserSkillReq) (*bot.CommonRes, error) {
	return botCli.DeleteUserSkill(ctx, req)
}

func BindUserSkill(ctx context.Context, req *bot.BindUserSkillReq) (*bot.CommonRes, error) {
	return botCli.BindUserSkill(ctx, req)
}

func ListBotSkills(ctx context.Context, req *bot.ListBotSkillsReq) (*bot.ListBotSkillsRes, error) {
	return botCli.ListBotSkills(ctx, req)
}
