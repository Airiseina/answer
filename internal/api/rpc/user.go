package rpc

import (
	"answer/internal/kitex_service/user_service/kitex_gen/user"
	"answer/internal/kitex_service/user_service/kitex_gen/user/loginservice"
	"answer/pkg/logger"
	"context"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/fallback"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	etcd "github.com/kitex-contrib/registry-etcd"
	"go.uber.org/zap"
)

var cli loginservice.Client

func Connect() {
	r, err := etcd.NewEtcdResolver([]string{"127.0.0.1:2379"})
	if err != nil {
		logger.Fatal("连接etcd出错", zap.Error(err))
	}
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
	c, err := loginservice.NewClient("userservice", client.WithResolver(r),
		client.WithFallback(loginFbPolicy),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(5*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		logger.Fatal("初始化客户端失败", zap.Error(err))
	}
	cli = c
}

func Register(ctx context.Context, req *user.RegisterReq) (*user.RegisterRes, error) {
	return cli.Register(ctx, req)
}

func Login(ctx context.Context, req *user.LoginReq) (*user.LoginRes, error) {
	return cli.Login(ctx, req)
}
