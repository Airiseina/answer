package rpc

import (
	"context"
	"time"

	user "github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user"
	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user/loginservice"

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
		klog.Fatalf("连接user_service失败: %v", err)
	}
	userCli = c
}

func GetUserName(ctx context.Context, userId int64) string {
	resp, err := userCli.GetUserNames(ctx, &user.GetUserNamesReq{
		UserIds: []int64{userId},
	})
	if err != nil {
		klog.CtxWarnf(ctx, "获取用户%d名称失败: %v", userId, err)
		return ""
	}
	if resp == nil || len(resp.Users) == 0 {
		return ""
	}
	return resp.Users[0].Name
}

func GetUserNames(ctx context.Context, userIds []int64) ([]*user.UserNameInfo, error) {
	resp, err := userCli.GetUserNames(ctx, &user.GetUserNamesReq{
		UserIds: userIds,
	})
	if err != nil {
		return nil, err
	}
	return resp.Users, nil
}

// ActiveUser 是枚举用户时的最小信息：用户ID + 个人租户ID。
type ActiveUser struct {
	UserID   int64
	TenantID int64
}

// ListActiveUsers 游标式枚举非 Bot 用户，供日/周总结调度逐个处理。
// afterUserID 为上一页最后一个用户ID，首页传 0。
func ListActiveUsers(ctx context.Context, afterUserID int64, limit int32) ([]ActiveUser, error) {
	resp, err := userCli.ListActiveUsers(ctx, &user.ListActiveUsersReq{
		AfterUserId: afterUserID,
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}
	users := make([]ActiveUser, 0, len(resp.Users))
	for _, u := range resp.Users {
		users = append(users, ActiveUser{UserID: u.UserId, TenantID: u.TenantId})
	}
	return users, nil
}

// GetPersonalTenantID 读取用户的个人租户 ID，供总结报告记录归属。
// 取不到时返回 0，不阻断总结生成。
func GetPersonalTenantID(ctx context.Context, userID int64) int64 {
	resp, err := userCli.ListTenants(ctx, &user.ListTenantsReq{UserId: userID})
	if err != nil {
		klog.CtxWarnf(ctx, "获取用户[%d]租户列表失败: %v", userID, err)
		return 0
	}
	for _, t := range resp.Tenants {
		if t.IsPersonal {
			return t.Id
		}
	}
	return 0
}
