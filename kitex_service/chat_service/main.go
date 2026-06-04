package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/Airiseina/answer/kitex_service/chat_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/chat_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/chat_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/chat_service/internal/service"
	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat/chatservice"
	"github.com/Airiseina/answer/kitex_service/chat_service/rpc"
	"github.com/Airiseina/answer/pkg/infra"
	"github.com/Airiseina/answer/pkg/observability/meter"
	"github.com/Airiseina/answer/pkg/observability/tracer"
	"gorm.io/gorm"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/limit"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	kitexzap "github.com/kitex-contrib/obs-opentelemetry/logging/zap"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	etcd "github.com/kitex-contrib/registry-etcd"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	kitexZapLogger := kitexzap.NewLogger(
		kitexzap.WithCoreWs(zapcore.AddSync(os.Stdout)),
		kitexzap.WithCoreLevel(zap.NewAtomicLevelAt(zap.DebugLevel)),
		kitexzap.WithZapOptions(
			zap.AddCaller(),
			zap.AddCallerSkip(3),
			zap.Fields(zap.String("service", "chat_service")),
		),
	)
	klog.SetLogger(kitexZapLogger)
	config.GetConfig()
	v := config.V
	otelAddr := v.GetString("otel.Addr")
	p := tracer.InitTracer("chat_service", otelAddr)
	defer p.Shutdown(context.Background())
	meter.InitMeter("chat_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
	}
	etcdAddr := v.GetString("etcd.Addr")
	r, err := etcd.NewEtcdRegistry([]string{etcdAddr})
	if err != nil {
		klog.Fatalf("注册中心出错: %v", err)
	}
	rpc.ConnectGroupService()
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:4322")
	if err != nil {
		klog.Fatalf("监听地址出错:%v", err)
	}
	db, err := infra.ConnectPostgres(v)
	if err != nil {
		klog.Fatalf("连接PostgreSQL失败:%v", err)
	}
	err = db.AutoMigrate(&model.Message{}, &model.Conversation{}, &model.ConversationMember{}, &model.MessageEditHistory{}, &model.InboxMessage{})
	if err != nil {
		klog.Fatalf("数据库建表失败:%v", err)
	}
	db.Exec("UPDATE message_table SET conversation_id = 0 WHERE conversation_id IS NULL")
	// 创建 PostgreSQL 按月 Range Partitioning
	// GORM 不原生支持分区表，需要用原生 SQL 在 AutoMigrate 后创建分区表
	// 分区键使用 timestamp 列（毫秒时间戳转 date），按月分区
	if err := createMessagePartitions(db); err != nil {
		klog.Warnf("创建消息表分区失败（非致命，表可能已存在分区）: %v", err)
	}
	rdb, err := infra.ConnectRedis(v)
	if err != nil {
		klog.Fatalf("连接Redis失败:%v", err)
	}
	chatDao := dal.NewChatDao(db, rdb)
	onlineDao := dal.NewOnlineDao(rdb)
	conversationDao := dal.NewConversationDao(db, rdb)
	inboxDao := dal.NewInboxDao(db)
	// 初始化 ClickHouse 冷库连接
	coldEnabled := v.GetBool("cold_storage.enabled")
	hotMonths := v.GetInt("cold_storage.hot_months")
	var coldStorageDao dal.ColdStorageDao
	if coldEnabled {
		chConn, err := infra.ConnectClickHouse(v)
		if err != nil {
			klog.Warnf("连接ClickHouse失败，冷库功能将不可用: %v", err)
			coldEnabled = false
		} else {
			if err := infra.InitClickHouseDB(chConn); err != nil {
				klog.Warnf("初始化ClickHouse数据库失败: %v", err)
			}
			coldStorageDao = dal.NewColdStorageDao(chConn)
		}
	}
	chatService := service.NewChatService(chatDao, onlineDao, conversationDao, inboxDao, coldStorageDao, rdb, coldEnabled, hotMonths)
	svr := chat.NewServer(&ChatServiceImpl{chatService: chatService},
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "chatservice"}),
		server.WithServiceAddr(addr),
		server.WithRegistry(r),
		server.WithLimit(&limit.Option{
			MaxConnections: 5000,
			MaxQPS:         5000,
		}))
	err = svr.Run()
	if err != nil {
		klog.Fatalf("服务启动失败:%v", err)
	}
}

// createMessagePartitions 创建 PostgreSQL 按月 Range Partitioning
// GORM 不原生支持分区表，需要用原生 SQL 创建
//
// PostgreSQL 不允许将已有普通表直接转为分区表，因此采用以下策略：
//  1. 检查 message_table 是否已是分区表（relkind = 'p'），是则跳过
//  2. 检查分区父表 message_table_partitioned 是否存在，不存在则创建
//  3. 为分区父表创建按月子分区
//
// 注意：从普通表迁移到分区表需要通过数据迁移脚本完成，步骤如下：
//
//	a. CREATE TABLE message_table_partitioned (...) PARTITION BY RANGE (to_timestamp(timestamp/1000));
//	b. 创建各月子分区
//	c. INSERT INTO message_table_partitioned SELECT * FROM message_table;
//	d. DROP TABLE message_table;
//	e. ALTER TABLE message_table_partitioned RENAME TO message_table;
//
// 此迁移应在维护窗口执行，此处仅创建分区表结构
func createMessagePartitions(db *gorm.DB) error {
	// 检查 message_table 是否已经是分区表
	var isPartitioned bool
	db.Raw("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = 'message_table' AND relkind = 'p')").Scan(&isPartitioned)
	if isPartitioned {
		return nil // 已经是分区表，无需重复创建
	}

	// 检查分区父表是否已存在
	var parentExists bool
	db.Raw("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = 'message_table_partitioned' AND relkind = 'p')").Scan(&parentExists)
	if !parentExists {
		// 创建分区父表，结构与 message_table 一致
		// PostgreSQL 分区表要求 PRIMARY KEY 必须包含分区键列
		// 分区键为 to_timestamp(timestamp/1000)，因此主键必须包含 timestamp
		// 使用复合主键 (msg_id, timestamp) 替代原来的单列主键 (msg_id)
		createParentSQL := `
		CREATE TABLE IF NOT EXISTS message_table_partitioned (
			msg_id BIGINT NOT NULL,
			client_seq BIGINT NOT NULL DEFAULT 0,
			sender_id BIGINT NOT NULL,
			conversation_id BIGINT NOT NULL DEFAULT 0,
			seq BIGINT NOT NULL DEFAULT 0,
			content JSONB NOT NULL,
			status SMALLINT NOT NULL DEFAULT 0,
			is_edited BOOLEAN NOT NULL DEFAULT FALSE,
			timestamp BIGINT NOT NULL,
			quote_msg_id BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (msg_id, timestamp)
		) PARTITION BY RANGE (to_timestamp(timestamp / 1000))`
		if err := db.Exec(createParentSQL).Error; err != nil {
			return fmt.Errorf("创建分区父表失败: %w", err)
		}
	}

	// 预创建前后 12 个月的分区
	now := time.Now()
	for i := -12; i <= 12; i++ {
		partitionTime := now.AddDate(0, i, 0)
		year := partitionTime.Year()
		month := int(partitionTime.Month())

		partitionName := fmt.Sprintf("message_table_y%dm%02d", year, month)
		startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		endDate := startDate.AddDate(0, 1, 0)

		sql := fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s PARTITION OF message_table_partitioned FOR VALUES FROM ('%s') TO ('%s')",
			partitionName,
			startDate.Format("2006-01-02"),
			endDate.Format("2006-01-02"),
		)
		if err := db.Exec(sql).Error; err != nil {
			klog.Warnf("创建分区 %s 失败（可能已存在）: %v", partitionName, err)
		}
	}

	klog.Info("分区表 message_table_partitioned 及子分区已创建，请通过迁移脚本将数据从 message_table 迁移到分区表")
	return nil
}
