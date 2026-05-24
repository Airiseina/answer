package rpc

import (
	"context"
	"time"

	"group_service/kitex_gen/group"
	"group_service/kitex_gen/group/groupservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	etcd "github.com/kitex-contrib/registry-etcd"
	"github.com/spf13/viper"
)

var groupCli groupservice.Client

func ConnectGroupService() {
	etcdAddr := viper.GetString("etcd.Addr")
	r, err := etcd.NewEtcdResolver([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("连接etcd出错:%v", err)
	}
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(3)
	fp.WithFixedBackOff(100)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.1,
		MinSample: 10,
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

func CheckMuted(ctx context.Context, groupId int64, userId int64) (bool, error) {
	resp, err := groupCli.CheckMuted(ctx, &group.CheckMutedReq{
		GroupId: groupId,
		UserId:  userId,
	})
	if err != nil {
		return false, err
	}
	return resp.IsMuted, nil
}
