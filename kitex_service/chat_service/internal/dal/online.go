package dal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type OnlineInfo struct {
	UserID      int64
	Online      bool
	GatewayAddr string
}

// onlineKey 生成用户在线状态 Key。
//
// 这里刻意不带租户前缀：在线状态描述的是「这个连接上的用户」，
// 而 user_id 由 snowflake 全局唯一生成，跨租户不会碰撞；
// 网关在任意租户上下文下都需要按 userID 反查在线状态，带租户反而会导致查不到。
func (dao *onlineDao) onlineKey(userID int64) string {
	return fmt.Sprintf("online:%d", userID)
}

func (dao *onlineDao) SetOnline(ctx context.Context, userID int64, gatewayAddr string) error {
	err := dao.rdb.Set(ctx, dao.onlineKey(userID), gatewayAddr, 60*time.Second).Err()
	if err != nil {
		return fmt.Errorf("设置在线状态失败: %w", err)
	}
	return nil
}

func (dao *onlineDao) RenewOnline(ctx context.Context, userID int64) error {
	err := dao.rdb.Expire(ctx, dao.onlineKey(userID), 60*time.Second).Err()
	if err != nil {
		return fmt.Errorf("续期在线状态失败: %w", err)
	}
	return nil
}

func (dao *onlineDao) SetOffline(ctx context.Context, userID int64) error {
	err := dao.rdb.Del(ctx, dao.onlineKey(userID)).Err()
	if err != nil {
		return fmt.Errorf("设置离线状态失败: %w", err)
	}
	return nil
}

func (dao *onlineDao) GetOnlineStatus(ctx context.Context, userIDs []int64) ([]OnlineInfo, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	keys := make([]string, len(userIDs))
	for i, uid := range userIDs {
		keys[i] = dao.onlineKey(uid)
	}
	vals, err := dao.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("批量查询在线状态失败: %w", err)
	}
	result := make([]OnlineInfo, 0, len(userIDs))
	for i, val := range vals {
		info := OnlineInfo{UserID: userIDs[i], Online: false}
		if val != nil {
			if addr, ok := val.(string); ok && addr != "" {
				info.Online = true
				info.GatewayAddr = addr
			}
		}
		result = append(result, info)
	}
	return result, nil
}

func (dao *onlineDao) IsOnline(ctx context.Context, userID int64) (bool, string, error) {
	val, err := dao.rdb.Get(ctx, dao.onlineKey(userID)).Result()
	if errors.Is(err, redis.Nil) {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("查询在线状态失败: %w", err)
	}
	return true, val, nil
}
