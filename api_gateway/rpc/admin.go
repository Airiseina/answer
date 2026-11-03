package rpc

import (
	"context"
	"time"

	"github.com/Airiseina/answer/kitex_service/admin_service/kitex_gen/admin"
	"github.com/Airiseina/answer/kitex_service/admin_service/kitex_gen/admin/adminservice"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

var adminCli adminservice.Client

func ConnectAdminService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(2)
	fp.WithFixedBackOff(200)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.5,
		MinSample: 50,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("adminservice", cbConfig)
	c, err := adminservice.NewClient("adminservice",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(3*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		hlog.Fatalf("初始化管理后台客户端失败:%v", err)
	}
	adminCli = c
}

func AdminLogin(ctx context.Context, req *admin.AdminLoginReq) (*admin.AdminLoginRes, error) {
	return adminCli.Login(ctx, req)
}

func AdminChangePassword(ctx context.Context, req *admin.ChangePasswordReq) (*admin.CommonRes, error) {
	return adminCli.ChangePassword(ctx, req)
}

func AdminGet(ctx context.Context, req *admin.GetAdminReq) (*admin.GetAdminRes, error) {
	return adminCli.GetAdmin(ctx, req)
}

func AdminList(ctx context.Context, req *admin.ListAdminsReq) (*admin.ListAdminsRes, error) {
	return adminCli.ListAdmins(ctx, req)
}

func AdminCreate(ctx context.Context, req *admin.CreateAdminReq) (*admin.CreateAdminRes, error) {
	return adminCli.CreateAdmin(ctx, req)
}

func AdminUpdateStatus(ctx context.Context, req *admin.UpdateAdminStatusReq) (*admin.CommonRes, error) {
	return adminCli.UpdateAdminStatus(ctx, req)
}
