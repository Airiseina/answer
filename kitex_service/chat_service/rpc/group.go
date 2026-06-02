package rpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Airiseina/answer/kitex_service/chat_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/group_service/kitex_gen/group"
	"github.com/Airiseina/answer/kitex_service/group_service/kitex_gen/group/groupservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	etcd "github.com/kitex-contrib/registry-etcd"
)

var groupCli groupservice.Client

func ConnectGroupService() {
	etcdAddr := config.V.GetString("etcd.Addr")
	r, err := etcd.NewEtcdResolver([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("连接etcd出错:%v", err)
	}
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

func GetMemberRole(ctx context.Context, groupId int64, userId int64) (int64, error) {
	resp, err := groupCli.GetGroupInfo(ctx, &group.GetGroupInfoReq{
		GroupId: groupId,
	})
	if err != nil {
		return -1, err
	}
	if resp.Group == nil || resp.Group.GroupId == 0 {
		return -1, fmt.Errorf("群不存在")
	}
	for _, m := range resp.Members {
		if m.UserId == userId {
			return m.Role, nil
		}
	}
	return -1, fmt.Errorf("用户不在群中")
}
