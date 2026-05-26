package rpc

import (
	"context"
	"time"

	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user"
	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user/loginservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/fallback"
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
	checkFbPolicy := fallback.NewFallbackPolicy(fallback.UnwrapHelper(
		func(ctx context.Context, req, resp interface{}, err error) (interface{}, error) {
			if err != nil {
				if r, ok := resp.(*user.CheckUsersExistRes); ok {
					if r == nil {
						r = &user.CheckUsersExistRes{}
						resp = r
					}
					r.AllExist = false
					return r, nil
				}
			}
			return resp, err
		}))

	c, err := loginservice.NewClient("userservice",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()), // 核心：开启 OpenTelemetry TraceID 透传！
		client.WithFallback(checkFbPolicy),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(5*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		klog.Fatalf("初始化客户端失败:%v", err)
	}
	userCli = c
}

func CheckUsersExist(ctx context.Context, userIds []int64) (bool, error) {
	req := &user.CheckUsersExistReq{
		UserIds: userIds,
	}
	resp, err := userCli.CheckUsersExist(ctx, req)
	if err != nil {
		return false, err
	}
	return resp.AllExist, nil
}

func GetUserNames(ctx context.Context, userIds []int64) (map[int64]string, error) {
	req := &user.GetUserNamesReq{
		UserIds: userIds,
	}
	resp, err := userCli.GetUserNames(ctx, req)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]string)
	if resp != nil {
		for _, u := range resp.Users {
			result[u.Id] = u.Name
		}
	}
	return result, nil
}
