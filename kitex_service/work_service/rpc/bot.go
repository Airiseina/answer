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
	fp.WithMaxRetryTimes(2)
	fp.WithFixedBackOff(200)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.5,
		MinSample: 30,
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
	Origin       int16 // 0=用户自建 1=市场安装 2=OpenAPI 接入（OpenAPI 不计入用量统计）
}

// Bot 来源，与 bot_service/internal/model 中的定义保持一致。
const (
	BotOriginUser    int16 = 0
	BotOriginMarket  int16 = 1
	BotOriginOpenAPI int16 = 2
)

// GetBotConfig 读取 Bot 的凭证配置。tenantId 用于强制租户过滤，0 表示系统内部调用（按 botId 全局查）。
func GetBotConfig(ctx context.Context, tenantId, botId int64) (*BotConfig, error) {
	resp, err := botCli.GetBotConfig(ctx, &bot.GetBotConfigReq{BotId: botId, TenantId: &tenantId})
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
	cfg.Origin = resp.GetOrigin()
	return cfg, nil
}

func GetBotMcpServers(ctx context.Context, tenantId, botId int64) (*bot.GetBotMcpServersRes, error) {
	return botCli.GetBotMcpServers(ctx, &bot.GetBotMcpServersReq{BotId: botId, TenantId: &tenantId})
}

// AgentGraphConfig 是 Bot 的协作编排图快照；Found=false 表示该 Bot 尚未发布编排图。
type AgentGraphConfig struct {
	GraphJSON string
	Status    int16
	Version   int32
	Found     bool
}

// GetAgentGraph 读取 Bot 的编排图；未发布或不存在时返回 Found=false，不视为错误。
//
// 编排图是否可执行由调用方按 Status 判定：只有「已发布」才按图协作执行。
func GetAgentGraph(ctx context.Context, tenantId, botId int64) (*AgentGraphConfig, error) {
	resp, err := botCli.GetAgentGraph(ctx, &bot.GetAgentGraphReq{TenantId: tenantId, BotId: botId})
	if err != nil {
		return nil, err
	}
	if resp == nil || !resp.GetSuccess() || resp.GetGraph() == nil {
		return &AgentGraphConfig{}, nil
	}
	g := resp.GetGraph()
	return &AgentGraphConfig{
		GraphJSON: g.GetGraphJson(),
		Status:    g.GetStatus(),
		Version:   g.GetVersion(),
		Found:     true,
	}, nil
}

func GetSystemBot(ctx context.Context) (int64, error) {
	resp, err := botCli.GetSystemBot(ctx)
	if err != nil {
		return 0, err
	}
	if !resp.Success {
		return 0, fmt.Errorf("系统Bot不存在")
	}
	return resp.BotId, nil
}
