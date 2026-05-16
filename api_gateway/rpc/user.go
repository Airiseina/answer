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

func AddFriend(ctx context.Context, req *user.AddFriendReq) (*user.CommonRes, error) {
	return userCli.AddFriend(ctx, req)
}

func HandleFriendReq(ctx context.Context, req *user.HandleFriendReqReq) (*user.CommonRes, error) {
	return userCli.HandleFriendReq(ctx, req)
}

func DeleteFriend(ctx context.Context, req *user.DeleteFriendReq) (*user.CommonRes, error) {
	return userCli.DeleteFriend(ctx, req)
}

func GetFriendList(ctx context.Context, req *user.GetFriendListReq) (*user.GetFriendListRes, error) {
	return userCli.GetFriendList(ctx, req)
}

func GetFriendRequests(ctx context.Context, req *user.GetFriendRequestsReq) (*user.GetFriendRequestsRes, error) {
	return userCli.GetFriendRequests(ctx, req)
}

func CreateFriendGroup(ctx context.Context, req *user.CreateFriendGroupReq) (*user.CreateFriendGroupRes, error) {
	return userCli.CreateFriendGroup(ctx, req)
}

func UpdateFriendGroup(ctx context.Context, req *user.UpdateFriendGroupReq) (*user.CommonRes, error) {
	return userCli.UpdateFriendGroup(ctx, req)
}

func DeleteFriendGroup(ctx context.Context, req *user.DeleteFriendGroupReq) (*user.CommonRes, error) {
	return userCli.DeleteFriendGroup(ctx, req)
}

func MoveFriendToGroup(ctx context.Context, req *user.MoveFriendToGroupReq) (*user.CommonRes, error) {
	return userCli.MoveFriendToGroup(ctx, req)
}

func UpdateFriendRemark(ctx context.Context, req *user.UpdateFriendRemarkReq) (*user.CommonRes, error) {
	return userCli.UpdateFriendRemark(ctx, req)
}

func GetFriendGroups(ctx context.Context, req *user.GetFriendGroupsReq) (*user.GetFriendGroupsRes, error) {
	return userCli.GetFriendGroups(ctx, req)
}

func SearchUserByAccount(ctx context.Context, req *user.SearchUserByAccountReq) (*user.SearchUserByAccountRes, error) {
	return userCli.SearchUserByAccount(ctx, req)
}

func GetUserNames(ctx context.Context, req *user.GetUserNamesReq) (*user.GetUserNamesRes, error) {
	return userCli.GetUserNames(ctx, req)
}
