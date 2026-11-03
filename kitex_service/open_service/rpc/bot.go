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

// BotConfig 是开放平台侧需要的 Bot 视图。
type BotConfig struct {
	BotID        int64
	CreatorID    int64
	Name         string
	SystemPrompt string
	Model        string
	BaseURL      string
	Origin       int16
	AppID        string
	IsSystem     bool
	CreatedAt    int64
}

// BotRuntimeConfig 是 Bot 的运行期配置，含 Bot 自身的用户 ID（发消息时作为 sender_id）。
type BotRuntimeConfig struct {
	UserID       int64
	APIKey       string
	Model        string
	SystemPrompt string
	BaseURL      string
	Origin       int16
}

func CreateBot(ctx context.Context, tenantID, creatorID int64, name, systemPrompt, apiKey, modelName, baseURL string, origin int16, appID string) (int64, error) {
	resp, err := botCli.CreateBot(ctx, &bot.CreateBotReq{
		CreatorId:    creatorID,
		Name:         name,
		SystemPrompt: systemPrompt,
		ApiKey:       apiKey,
		Model:        modelName,
		BaseUrl:      &baseURL,
		TenantId:     &tenantID,
		Origin:       &origin,
		AppId:        &appID,
	})
	if err != nil {
		return 0, fmt.Errorf("创建Bot失败: %w", err)
	}
	if !resp.Success {
		return 0, fmt.Errorf("创建Bot返回失败")
	}
	return resp.BotId, nil
}

func GetBot(ctx context.Context, tenantID, botID int64) (BotConfig, error) {
	resp, err := botCli.GetBot(ctx, &bot.GetBotReq{BotId: botID, TenantId: &tenantID})
	if err != nil {
		return BotConfig{}, fmt.Errorf("查询Bot失败: %w", err)
	}
	if !resp.Success || resp.BotInfo == nil {
		return BotConfig{}, nil
	}
	return toBotConfig(resp.BotInfo), nil
}

func GetBotConfig(ctx context.Context, tenantID, botID int64) (BotRuntimeConfig, error) {
	resp, err := botCli.GetBotConfig(ctx, &bot.GetBotConfigReq{BotId: botID, TenantId: &tenantID})
	if err != nil {
		return BotRuntimeConfig{}, fmt.Errorf("查询Bot配置失败: %w", err)
	}
	if !resp.Success {
		return BotRuntimeConfig{}, nil
	}
	return BotRuntimeConfig{
		UserID:       resp.GetUserId(),
		APIKey:       resp.GetApiKey(),
		Model:        resp.GetModel(),
		SystemPrompt: resp.GetSystemPrompt(),
		BaseURL:      resp.GetBaseUrl(),
		Origin:       resp.GetOrigin(),
	}, nil
}

func ListUserBots(ctx context.Context, tenantID, creatorID int64) ([]BotConfig, error) {
	resp, err := botCli.GetUserBots(ctx, &bot.GetUserBotsReq{CreatorId: creatorID, TenantId: &tenantID})
	if err != nil {
		return nil, fmt.Errorf("查询Bot列表失败: %w", err)
	}
	list := make([]BotConfig, 0, len(resp.Bots))
	for _, b := range resp.Bots {
		list = append(list, toBotConfig(b))
	}
	return list, nil
}

func UpdateBot(ctx context.Context, tenantID, botID, operatorID int64, updates map[string]interface{}) (bool, error) {
	req := &bot.UpdateBotReq{
		BotId:      botID,
		OperatorId: operatorID,
		TenantId:   &tenantID,
	}
	if v, ok := updates["name"].(string); ok {
		req.Name = &v
	}
	if v, ok := updates["system_prompt"].(string); ok {
		req.SystemPrompt = &v
	}
	if v, ok := updates["api_key"].(string); ok {
		req.ApiKey = &v
	}
	if v, ok := updates["model"].(string); ok {
		req.Model = &v
	}
	if v, ok := updates["base_url"].(string); ok {
		req.BaseUrl = &v
	}
	resp, err := botCli.UpdateBot(ctx, req)
	if err != nil {
		return false, fmt.Errorf("更新Bot失败: %w", err)
	}
	return resp.Success, nil
}

func DeleteBot(ctx context.Context, tenantID, botID, operatorID int64) (bool, error) {
	resp, err := botCli.DeleteBot(ctx, &bot.DeleteBotReq{
		BotId:      botID,
		OperatorId: operatorID,
		TenantId:   &tenantID,
	})
	if err != nil {
		return false, fmt.Errorf("删除Bot失败: %w", err)
	}
	return resp.Success, nil
}

func AddBotToConversation(ctx context.Context, tenantID, operatorID, botID, conversationID int64, convType int16) (int64, error) {
	resp, err := botCli.AddBotToConversation(ctx, &bot.AddBotToConversationReq{
		OperatorId:       operatorID,
		BotId:            botID,
		ConversationId:   conversationID,
		ConversationType: convType,
		TenantId:         &tenantID,
	})
	if err != nil {
		return 0, fmt.Errorf("将Bot加入会话失败: %w", err)
	}
	if !resp.Success {
		return 0, fmt.Errorf("将Bot加入会话返回失败")
	}
	return resp.GetConversationId(), nil
}

// IsBot 依据 Bot 用户 ID 反查 Bot，用于事件消费侧把回复映射回 Bot 定义。
func IsBot(ctx context.Context, userID int64) (bool, int64, error) {
	resp, err := botCli.IsBot(ctx, &bot.IsBotReq{UserId: userID})
	if err != nil {
		return false, 0, fmt.Errorf("查询Bot失败: %w", err)
	}
	return resp.IsBot, resp.GetBotId(), nil
}

func toBotConfig(b *bot.BotInfo) BotConfig {
	return BotConfig{
		BotID:        b.BotId,
		CreatorID:    b.CreatorId,
		Name:         b.Name,
		SystemPrompt: b.SystemPrompt,
		Model:        b.Model,
		BaseURL:      b.GetBaseUrl(),
		Origin:       b.GetOrigin(),
		AppID:        b.GetAppId(),
		IsSystem:     b.IsSystem,
		CreatedAt:    b.CreatedAt,
	}
}
