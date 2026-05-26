package rpc

import (
	"context"
	"fmt"
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
		klog.Fatalf("连接user_service失败: %v", err)
	}
	userCli = c
}

func CreateBotUser(ctx context.Context, name, avatarURL string) (int64, error) {
	resp, err := userCli.CreateBotUser(ctx, &user.CreateBotUserReq{
		Name:      name,
		AvatarUrl: avatarURL,
	})
	if err != nil {
		return 0, err
	}
	if !resp.Success {
		return 0, fmt.Errorf("创建Bot用户记录返回失败")
	}
	return resp.UserId, nil
}

func UpdateBotUserName(ctx context.Context, userID int64, name string) error {
	resp, err := userCli.UpdateBotUserName(ctx, &user.UpdateBotUserNameReq{
		UserId: userID,
		Name:   name,
	})
	if err != nil {
		return fmt.Errorf("更新Bot用户名失败: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("更新Bot用户名返回失败")
	}
	return nil
}

func UpdateBotUserAvatar(ctx context.Context, userID int64, avatarURL string) error {
	resp, err := userCli.UpdateAvatar(ctx, &user.UpdateAvatarReq{
		UserId:    userID,
		AvatarUrl: avatarURL,
	})
	if err != nil {
		return fmt.Errorf("更新Bot用户头像失败: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("更新Bot用户头像返回失败")
	}
	return nil
}

type UserNameDTO struct {
	Id        int64
	Name      string
	AvatarURL string
}

func GetUserNames(ctx context.Context, userIds []int64) ([]UserNameDTO, error) {
	resp, err := userCli.GetUserNames(ctx, &user.GetUserNamesReq{UserIds: userIds})
	if err != nil {
		return nil, fmt.Errorf("查询用户名称失败: %w", err)
	}
	var result []UserNameDTO
	for _, u := range resp.Users {
		result = append(result, UserNameDTO{
			Id:        u.Id,
			Name:      u.Name,
			AvatarURL: u.AvatarUrl,
		})
	}
	return result, nil
}
