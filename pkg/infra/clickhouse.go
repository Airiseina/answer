package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/spf13/viper"
)

// ConnectClickHouse 连接 ClickHouse 冷库
// 使用 clickhouse-go/v2 原生客户端，比 database/sql 包装器性能更好
// ClickHouse 作为冷库存储历史消息，支持海量数据的快速分析查询
func ConnectClickHouse(v *viper.Viper) (clickhouse.Conn, error) {
	addr := v.GetString("clickhouse.addr")
	username := v.GetString("clickhouse.username")
	password := v.GetString("clickhouse.password")
	database := v.GetString("clickhouse.database")

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
		DialTimeout:      5 * time.Second,
		MaxOpenConns:     10,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Hour,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
		BlockBufferSize:  10,
	})
	if err != nil {
		return nil, fmt.Errorf("ClickHouse连接失败: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ClickHouse连接验证失败: %w", err)
	}
	return conn, nil
}

// InitClickHouseDB 初始化 ClickHouse 数据库和表
// 使用 ReplicatedMergeTree 引擎（单节点也兼容），按月分区，按 conversation_id + timestamp 排序
func InitClickHouseDB(conn clickhouse.Conn) error {
	ctx := context.Background()
	// 创建数据库（如果不存在）
	if err := conn.Exec(ctx, "CREATE DATABASE IF NOT EXISTS answer_cold"); err != nil {
		return fmt.Errorf("创建ClickHouse数据库失败: %w", err)
	}
	// 创建冷库消息表
	// 使用 MergeTree 引擎，按 toYYYYMM(timestamp) 按月分区
	// ORDER BY (conversation_id, timestamp) 使得按会话+时间查询非常高效
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS answer_cold.cold_message (
		msg_id Int64,
		client_seq Int64,
		sender_id Int64,
		conversation_id Int64,
		seq Int64,
		content String,
		status Int16,
		is_edited UInt8,
		timestamp Int64,
		quote_msg_id Int64,
		archived_at Int64
	) ENGINE = MergeTree()
	PARTITION BY toYYYYMM(toDateTime(timestamp / 1000))
	ORDER BY (conversation_id, timestamp)
	TTL toDateTime(archived_at / 1000) + INTERVAL 5 YEAR
	`
	if err := conn.Exec(ctx, createTableSQL); err != nil {
		return fmt.Errorf("创建ClickHouse冷库表失败: %w", err)
	}
	return nil
}
