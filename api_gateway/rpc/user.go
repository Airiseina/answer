package rpc

import (
	"context"
	"time"
	"user_service/kitex_gen/user"
	"user_service/kitex_gen/user/loginservice"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/fallback"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

var userCli loginservice.Client

func ConnectUserService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(3)  //最大重试次数
	fp.WithFixedBackOff(100) //
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.1,
		MinSample: 10,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("userservice", cbConfig)
	loginFbPolicy := fallback.NewFallbackPolicy(fallback.UnwrapHelper(
		func(ctx context.Context, req, resp interface{}, err error) (interface{}, error) {
			if err != nil {
				if r, ok := resp.(*user.LoginRes); ok {
					if r == nil {
						r = &user.LoginRes{}
						resp = r
					}
					r.Id = 0
					r.Account = "0"
					return r, nil
				}
			}
			return resp, err
		}))
	c, err := loginservice.NewClient("userservice",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()), //链路追踪
		client.WithFallback(loginFbPolicy),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(5*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		hlog.Fatalf("初始化客户端失败:%v", err)
	}
	userCli = c
}

func Register(ctx context.Context, req *user.RegisterReq) (*user.RegisterRes, error) {
	return userCli.Register(ctx, req)
}

func Login(ctx context.Context, req *user.LoginReq) (*user.LoginRes, error) {
	return userCli.Login(ctx, req)
}
