package rpc

import (
	"context"
	"time"

	"github.com/Airiseina/answer/kitex_service/open_service/kitex_gen/open"
	"github.com/Airiseina/answer/kitex_service/open_service/kitex_gen/open/openservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

var openCli openservice.Client

func ConnectOpenService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(2)
	fp.WithFixedBackOff(200)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.5,
		MinSample: 50,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("openservice", cbConfig)
	c, err := openservice.NewClient("openservice",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(5*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		hlog.Fatalf("初始化开放平台客户端失败:%v", err)
	}
	openCli = c
}

// ===== 应用管理 =====

func OpenCreateApp(ctx context.Context, req *open.CreateAppReq) (*open.CreateAppRes, error) {
	return openCli.CreateApp(ctx, req)
}

func OpenListApps(ctx context.Context, req *open.ListAppsReq) (*open.ListAppsRes, error) {
	return openCli.ListApps(ctx, req)
}

func OpenGetApp(ctx context.Context, req *open.GetAppReq) (*open.GetAppRes, error) {
	return openCli.GetApp(ctx, req)
}

func OpenVerifyApp(ctx context.Context, req *open.VerifyAppReq) (*open.VerifyAppRes, error) {
	return openCli.VerifyApp(ctx, req)
}

func OpenAuthenticateApp(ctx context.Context, req *open.AuthenticateAppReq) (*open.AuthenticateAppRes, error) {
	return openCli.AuthenticateApp(ctx, req)
}

func OpenResetAppSecret(ctx context.Context, req *open.ResetAppSecretReq) (*open.ResetAppSecretRes, error) {
	return openCli.ResetAppSecret(ctx, req)
}

func OpenUpdateAppStatus(ctx context.Context, req *open.UpdateAppStatusReq) (*open.CommonRes, error) {
	return openCli.UpdateAppStatus(ctx, req)
}

// ===== Bot 管理 =====

func OpenCreateBot(ctx context.Context, req *open.CreateOpenBotReq) (*open.CreateOpenBotRes, error) {
	return openCli.CreateBot(ctx, req)
}

func OpenListBots(ctx context.Context, req *open.ListOpenBotsReq) (*open.ListOpenBotsRes, error) {
	return openCli.ListBots(ctx, req)
}

func OpenGetBot(ctx context.Context, req *open.GetOpenBotReq) (*open.GetOpenBotRes, error) {
	return openCli.GetBot(ctx, req)
}

func OpenUpdateBot(ctx context.Context, req *open.UpdateOpenBotReq) (*open.CommonRes, error) {
	return openCli.UpdateBot(ctx, req)
}

func OpenDeleteBot(ctx context.Context, req *open.DeleteOpenBotReq) (*open.CommonRes, error) {
	return openCli.DeleteBot(ctx, req)
}

func OpenAddBotToConversation(ctx context.Context, req *open.AddOpenBotConversationReq) (*open.AddOpenBotConversationRes, error) {
	return openCli.AddBotToConversation(ctx, req)
}

// ===== 消息与会话 =====

func OpenSendMessage(ctx context.Context, req *open.OpenSendMessageReq) (*open.OpenSendMessageRes, error) {
	return openCli.SendMessage(ctx, req)
}

func OpenListMessages(ctx context.Context, req *open.ListOpenMessagesReq) (*open.ListOpenMessagesRes, error) {
	return openCli.ListMessages(ctx, req)
}

func OpenListConversations(ctx context.Context, req *open.ListOpenConversationsReq) (*open.ListOpenConversationsRes, error) {
	return openCli.ListConversations(ctx, req)
}

// ===== 事件与回调 =====

func OpenListEvents(ctx context.Context, req *open.ListEventsReq) (*open.ListEventsRes, error) {
	return openCli.ListEvents(ctx, req)
}

func OpenAckEvents(ctx context.Context, req *open.AckEventsReq) (*open.CommonRes, error) {
	return openCli.AckEvents(ctx, req)
}

func OpenRegisterWebhook(ctx context.Context, req *open.RegisterWebhookReq) (*open.RegisterWebhookRes, error) {
	return openCli.RegisterWebhook(ctx, req)
}

func OpenListWebhooks(ctx context.Context, req *open.ListWebhooksReq) (*open.ListWebhooksRes, error) {
	return openCli.ListWebhooks(ctx, req)
}

func OpenGetWebhook(ctx context.Context, req *open.GetWebhookReq) (*open.GetWebhookRes, error) {
	return openCli.GetWebhook(ctx, req)
}

func OpenDeleteWebhook(ctx context.Context, req *open.DeleteWebhookReq) (*open.CommonRes, error) {
	return openCli.DeleteWebhook(ctx, req)
}
