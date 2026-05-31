package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

func ConnectRedis(v *viper.Viper) (*redis.Client, error) {
	addr := v.GetString("redis.addr")
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	if rdb == nil {
		return nil, fmt.Errorf("Redis连接失败: client为nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis连接验证失败: %w", err)
	}
	return rdb, nil
}
