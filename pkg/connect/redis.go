package connect

import (
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

func ConnectRedis() (*redis.Client, error) {
	addr := viper.GetString("redis.addr")
	password := viper.GetString("redis.password")
	db := viper.GetInt("redis.db")
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if rdb == nil {
		return nil, fmt.Errorf("Redis连接失败: client为nil")
	}
	return rdb, nil
}
