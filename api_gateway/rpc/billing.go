package rpc

import (
	"context"
	"time"

	"github.com/Airiseina/answer/kitex_service/billing_service/kitex_gen/billing"
	"github.com/Airiseina/answer/kitex_service/billing_service/kitex_gen/billing/billingservice"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
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
		MinSample: 50,
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
		hlog.Fatalf("初始化用量统计客户端失败:%v", err)
	}
	billingCli = c
}

// ===== 用户侧：本人用量 =====

func GetUsageDetail(ctx context.Context, req *billing.GetUsageDetailReq) (*billing.GetUsageDetailRes, error) {
	return billingCli.GetUsageDetail(ctx, req)
}

func GetUsageSummary(ctx context.Context, req *billing.GetUsageSummaryReq) (*billing.GetUsageSummaryRes, error) {
	return billingCli.GetUsageSummary(ctx, req)
}

// ===== 管理后台：参考单价 =====

func ListModelPrices(ctx context.Context, req *billing.ListModelPricesReq) (*billing.ListModelPricesRes, error) {
	return billingCli.ListModelPrices(ctx, req)
}

func UpsertModelPrice(ctx context.Context, req *billing.UpsertModelPriceReq) (*billing.CommonRes, error) {
	return billingCli.UpsertModelPrice(ctx, req)
}

func DeleteModelPrice(ctx context.Context, req *billing.DeleteModelPriceReq) (*billing.CommonRes, error) {
	return billingCli.DeleteModelPrice(ctx, req)
}
