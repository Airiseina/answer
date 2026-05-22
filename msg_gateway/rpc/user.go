package rpc

import (
	"context"
	"time"

	"user_service/kitex_gen/user"
	"user_service/kitex_gen/user/loginservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

var userCli loginservice.Client

func ConnectUserService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(3)
	fp.WithFixedBackOff(100)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.1,
		MinSample: 10,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("userservice", cbConfig)
	c, err := loginservice.NewClient("userservice",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(5*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		klog.Fatalf("连接user_service失败: %v", err)
	}
	userCli = c
}

func GetUserName(ctx context.Context, userId int64) string {
	resp, err := userCli.GetUserNames(ctx, &user.GetUserNamesReq{
		UserIds: []int64{userId},
	})
	if err != nil {
		klog.Errorf("获取用户%d名称失败: %v", userId, err)
		return ""
	}
	if resp == nil || len(resp.Users) == 0 {
		return ""
	}
	return resp.Users[0].Name
}
