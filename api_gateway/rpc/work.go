package rpc

import (
	"context"
	"time"

	work "github.com/Airiseina/answer/kitex_service/work_service/kitex_gen/work"
	"github.com/Airiseina/answer/kitex_service/work_service/kitex_gen/work/workservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

var workCli workservice.Client

func ConnectWorkService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(1)
	fp.WithFixedBackOff(500)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.5,
		MinSample: 20,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("workservice", cbConfig)
	c, err := workservice.NewClient("workservice",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(30*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		klog.Fatalf("连接work_service失败: %v", err)
	}
	workCli = c
}

func SummarizeConversation(ctx context.Context, req *work.SummarizeConversationReq) (*work.SummarizeConversationRes, error) {
	return workCli.SummarizeConversation(ctx, req)
}

func SuggestReplies(ctx context.Context, req *work.SuggestRepliesReq) (*work.SuggestRepliesRes, error) {
	return workCli.SuggestReplies(ctx, req)
}

func TranslateMessage(ctx context.Context, req *work.TranslateMessageReq) (*work.TranslateMessageRes, error) {
	return workCli.TranslateMessage(ctx, req)
}
