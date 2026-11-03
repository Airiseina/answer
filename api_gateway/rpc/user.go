package rpc

import (
	"context"
	"time"

	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user"
	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user/loginservice"

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
	fp.WithMaxRetryTimes(2)
	fp.WithFixedBackOff(200)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.5,
		MinSample: 50,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("userservice", cbConfig)
	c, err := loginservice.NewClient("userservice",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(3*time.Second),
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

func GetUserIdsByAccounts(ctx context.Context, req *user.GetUserIdsByAccountsReq) (*user.GetUserIdsByAccountsRes, error) {
	return userCli.GetUserIdsByAccounts(ctx, req)
}

func GetUsersInfoByAccounts(ctx context.Context, req *user.GetUsersInfoByAccountsReq) (*user.GetUsersInfoByAccountsRes, error) {
	return userCli.GetUsersInfoByAccounts(ctx, req)
}

func UpdateAvatar(ctx context.Context, req *user.UpdateAvatarReq) (*user.CommonRes, error) {
	return userCli.UpdateAvatar(ctx, req)
}

func CreateTenant(ctx context.Context, req *user.CreateTenantReq) (*user.CreateTenantRes, error) {
	return userCli.CreateTenant(ctx, req)
}

func ListTenants(ctx context.Context, req *user.ListTenantsReq) (*user.ListTenantsRes, error) {
	return userCli.ListTenants(ctx, req)
}

func GetTenant(ctx context.Context, req *user.GetTenantReq) (*user.GetTenantRes, error) {
	return userCli.GetTenant(ctx, req)
}

func UpdateTenant(ctx context.Context, req *user.UpdateTenantReq) (*user.CommonRes, error) {
	return userCli.UpdateTenant(ctx, req)
}

func ListTenantMembers(ctx context.Context, req *user.ListTenantMembersReq) (*user.ListTenantMembersRes, error) {
	return userCli.ListTenantMembers(ctx, req)
}

func AddTenantMember(ctx context.Context, req *user.AddTenantMemberReq) (*user.CommonRes, error) {
	return userCli.AddTenantMember(ctx, req)
}

func SetTenantMemberRole(ctx context.Context, req *user.SetTenantMemberRoleReq) (*user.CommonRes, error) {
	return userCli.SetTenantMemberRole(ctx, req)
}

func RemoveTenantMember(ctx context.Context, req *user.RemoveTenantMemberReq) (*user.CommonRes, error) {
	return userCli.RemoveTenantMember(ctx, req)
}

func ListDepartments(ctx context.Context, req *user.ListDepartmentsReq) (*user.ListDepartmentsRes, error) {
	return userCli.ListDepartments(ctx, req)
}

func CreateDepartment(ctx context.Context, req *user.CreateDepartmentReq) (*user.CreateDepartmentRes, error) {
	return userCli.CreateDepartment(ctx, req)
}

func RenameDepartment(ctx context.Context, req *user.RenameDepartmentReq) (*user.CommonRes, error) {
	return userCli.RenameDepartment(ctx, req)
}

func MoveDepartment(ctx context.Context, req *user.MoveDepartmentReq) (*user.CommonRes, error) {
	return userCli.MoveDepartment(ctx, req)
}

func RemoveDepartment(ctx context.Context, req *user.RemoveDepartmentReq) (*user.CommonRes, error) {
	return userCli.RemoveDepartment(ctx, req)
}

func SetMemberDepartment(ctx context.Context, req *user.SetMemberDepartmentReq) (*user.CommonRes, error) {
	return userCli.SetMemberDepartment(ctx, req)
}
