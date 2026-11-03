package rpc

import (
	"context"
	"time"

	billing "github.com/Airiseina/answer/kitex_service/billing_service/kitex_gen/billing"
	"github.com/Airiseina/answer/kitex_service/billing_service/kitex_gen/billing/billingservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

var billingCli billingservice.Client

func ConnectBillingService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(2)
	fp.WithFixedBackOff(200)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.5,
		MinSample: 30,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("billingservice", cbConfig)
	c, err := billingservice.NewClient("billingservice",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(5*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		klog.Fatalf("连接billing_service失败: %v", err)
	}
	billingCli = c
}

// RecordUsageInput 是上报一条 token 用量所需的字段。
type RecordUsageInput struct {
	UserID           int64
	TenantID         int64
	BotID            int64
	Origin           int16
	ConversationID   int64
	MsgID            int64
	Model            string
	PromptTokens     int64
	CompletionTokens int64
}

// RecordUsage 上报一次 Bot 回复的 token 用量，由 billing_service 按 msg_id 幂等去重。
func RecordUsage(ctx context.Context, in RecordUsageInput) error {
	resp, err := billingCli.RecordUsage(ctx, &billing.RecordUsageReq{
		UserId:           in.UserID,
		TenantId:         in.TenantID,
		BotId:            in.BotID,
		Origin:           in.Origin,
		ConversationId:   in.ConversationID,
		MsgId:            in.MsgID,
		Model:            in.Model,
		PromptTokens:     in.PromptTokens,
		CompletionTokens: in.CompletionTokens,
	})
	if err != nil {
		return err
	}
	if resp != nil && !resp.Success {
		return nil
	}
	return nil
}
