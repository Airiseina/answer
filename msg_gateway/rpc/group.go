package rpc

import (
	"time"

	"github.com/Airiseina/answer/kitex_service/group_service/kitex_gen/group/groupservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

var groupCli groupservice.Client

func ConnectGroupService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(2)
	fp.WithFixedBackOff(200)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.5,
		MinSample: 50,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("groupservice", cbConfig)
	c, err := groupservice.NewClient("groupservice",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(5*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		klog.Fatalf("连接group_service失败: %v", err)
	}
	groupCli = c
}
