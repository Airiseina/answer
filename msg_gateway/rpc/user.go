package rpc

import (
	"context"
	"time"

	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user"
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
		klog.Errorf("获取用户%d名称失败: %v", userId, err)
		return ""
	}
	if resp == nil || len(resp.Users) == 0 {
		return ""
	}
	return resp.Users[0].Name
}

func GetUserAccount(ctx context.Context, userId int64) string {
	resp, err := userCli.GetUserNames(ctx, &user.GetUserNamesReq{
		UserIds: []int64{userId},
	})
	if err != nil {
		klog.Errorf("获取用户%d账号失败: %v", userId, err)
		return ""
	}
	if resp == nil || len(resp.Users) == 0 {
		return ""
	}
	return resp.Users[0].Account
}

func GetUserAccountMap(ctx context.Context, userIds []int64) map[int64]string {
	if len(userIds) == 0 {
		return make(map[int64]string)
	}
	uniqueIDs := make(map[int64]struct{})
	for _, id := range userIds {
		uniqueIDs[id] = struct{}{}
	}
	idList := make([]int64, 0, len(uniqueIDs))
	for id := range uniqueIDs {
		idList = append(idList, id)
	}
	resp, err := userCli.GetUserNames(ctx, &user.GetUserNamesReq{
		UserIds: idList,
	})
	if err != nil {
		klog.Errorf("批量获取用户account失败: %v", err)
		return make(map[int64]string)
	}
	m := make(map[int64]string, len(resp.Users))
	for _, u := range resp.Users {
		m[u.Id] = u.Account
	}
	return m
}

func GetUserIdMap(ctx context.Context, accounts []string) map[string]int64 {
	if len(accounts) == 0 {
		return make(map[string]int64)
	}
	uniqueAccounts := make(map[string]struct{})
	for _, a := range accounts {
		uniqueAccounts[a] = struct{}{}
	}
	accountList := make([]string, 0, len(uniqueAccounts))
	for a := range uniqueAccounts {
		accountList = append(accountList, a)
	}
	resp, err := userCli.GetUserIdsByAccounts(ctx, &user.GetUserIdsByAccountsReq{
		Accounts: accountList,
	})
	if err != nil {
		klog.Errorf("批量获取用户ID失败: %v", err)
		return make(map[string]int64)
	}
	m := make(map[string]int64, len(resp.Users))
	for _, u := range resp.Users {
		m[u.Account] = u.Id
	}
	return m
}
