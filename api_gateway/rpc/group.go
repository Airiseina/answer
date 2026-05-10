package rpc

import (
	"context"
	"group_service/kitex_gen/group"
	"group_service/kitex_gen/group/groupservice"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/fallback"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

var groupCli groupservice.Client

func ConnectGroupService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(3)  //最大重试次数
	fp.WithFixedBackOff(100) //
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.1,
		MinSample: 10,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("groupservice", cbConfig)
	groupFbPolicy := fallback.NewFallbackPolicy(fallback.UnwrapHelper(
		func(ctx context.Context, req, resp interface{}, err error) (interface{}, error) {
			if err != nil {
				if r, ok := resp.(*group.CreateGroupRes); ok {
					if r == nil {
						r = &group.CreateGroupRes{}
						resp = r
					}
					r.GroupId = 0
					return r, nil
				}
			}
			return resp, err
		}))
	c, err := groupservice.NewClient("groupservice",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()), //链路追踪
		client.WithFallback(groupFbPolicy),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(5*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		hlog.Fatalf("初始化客户端失败:%v", err)
	}
	groupCli = c
}

func CreateGroup(ctx context.Context, req *group.CreateGroupReq) (*group.CreateGroupRes, error) {
	return groupCli.CreateGroup(ctx, req)
}

func InviteMembers(ctx context.Context, req *group.InviteMembersReq) (*group.CommonRes, error) {
	return groupCli.InviteMembers(ctx, req)
}

func KickMembers(ctx context.Context, req *group.KickMembersReq) (*group.CommonRes, error) {
	return groupCli.KickMembers(ctx, req)
}

func GetGroupInfo(ctx context.Context, req *group.GetGroupInfoReq) (*group.GetGroupInfoRes, error) {
	return groupCli.GetGroupInfo(ctx, req)
}

func ChangeOwner(ctx context.Context, req *group.ChangeOwnerReq) (resp *group.CommonRes, err error) {
	return groupCli.ChangeOwner(ctx, req)
}

func ChangeNotice(ctx context.Context, req *group.ChangeNoticeReq) (resp *group.CommonRes, err error) {
	return groupCli.ChangeNotice(ctx, req)
}
func Muted(ctx context.Context, req *group.MutedReq) (resp *group.CommonRes, err error) {
	return groupCli.Muted(ctx, req)
}
func SetAdmin(ctx context.Context, req *group.SetAdminReq) (resp *group.CommonRes, err error) {
	return groupCli.SetAdmin(ctx, req)
}
