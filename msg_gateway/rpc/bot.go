package rpc

import (
	"context"
	"sync"
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

func IsBot(ctx context.Context, userId int64) (bool, int64, error) {
	resp, err := botCli.IsBot(ctx, &bot.IsBotReq{UserId: userId})
	if err != nil {
		return false, 0, err
	}
	botId := int64(0)
	if resp.IsSetBotId() {
		botId = resp.GetBotId()
	}
	return resp.IsBot, botId, nil
}

type botCacheEntry struct {
	isBot    bool
	botId    int64
	expireAt time.Time
}

var (
	botCache    sync.Map
	botCacheTTL = 5 * time.Minute
)

func IsBotCached(ctx context.Context, userId int64) (bool, int64, error) {
	if entry, ok := botCache.Load(userId); ok {
		e := entry.(*botCacheEntry)
		if time.Now().Before(e.expireAt) {
			return e.isBot, e.botId, nil
		}
		botCache.Delete(userId)
	}
	isBot, botId, err := IsBot(ctx, userId)
	if err != nil {
		return false, 0, err
	}
	botCache.Store(userId, &botCacheEntry{
		isBot:    isBot,
		botId:    botId,
		expireAt: time.Now().Add(botCacheTTL),
	})
	return isBot, botId, nil
}

func InvalidateBotCache(userId int64) {
	botCache.Delete(userId)
}
