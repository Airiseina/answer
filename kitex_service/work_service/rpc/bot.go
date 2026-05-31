package rpc

import (
	"context"
	"fmt"
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

type BotConfig struct {
	ApiKey       string
	Model        string
	SystemPrompt string
	UserID       int64
	BaseUrl      string
	Name         string
}

func GetBotConfig(ctx context.Context, botId int64) (*BotConfig, error) {
	resp, err := botCli.GetBotConfig(ctx, &bot.GetBotConfigReq{BotId: botId})
	if err != nil {
		return nil, err
	}
	cfg := &BotConfig{}
	if resp.ApiKey != nil {
		cfg.ApiKey = resp.GetApiKey()
	}
	if resp.Model != nil {
		cfg.Model = resp.GetModel()
	}
	if resp.SystemPrompt != nil {
		cfg.SystemPrompt = resp.GetSystemPrompt()
	}
	if resp.UserId != nil {
		cfg.UserID = resp.GetUserId()
	}
	if cfg.ApiKey == "" {
		return nil, fmt.Errorf("Bot[%d]未配置API Key", botId)
	}
	if resp.BaseUrl != nil {
		cfg.BaseUrl = resp.GetBaseUrl()
	}
	return cfg, nil
}

func GetBotMcpServers(ctx context.Context, botId int64) (*bot.GetBotMcpServersRes, error) {
	return botCli.GetBotMcpServers(ctx, &bot.GetBotMcpServersReq{BotId: botId})
}
