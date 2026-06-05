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
	defer func() { _ = p.Shutdown(context.Background()) }()
	meter.InitMeter("chat_service")
	if os.Getenv("KITEX_IP_TO_REGISTRY") == "" {
		_ = os.Setenv("KITEX_IP_TO_REGISTRY", "127.0.0.1")
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
	// 检查 message_table 是否已存在
	var messageTableExists bool
	db.Raw("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = 'message_table')").Scan(&messageTableExists)

	if messageTableExists {
		// 已存在普通表，走 AutoMigrate 更新结构
		err = db.AutoMigrate(&model.Message{}, &model.Conversation{}, &model.ConversationMember{}, &model.MessageEditHistory{}, &model.InboxMessage{})
		if err != nil {
			klog.Fatalf("数据库建表失败:%v", err)
		}
		db.Exec("UPDATE message_table SET conversation_id = 0 WHERE conversation_id IS NULL")
	} else {
		// 全新数据库：先 AutoMigrate 其他表，再直接创建 message_table 分区表
		err = db.AutoMigrate(&model.Conversation{}, &model.ConversationMember{}, &model.MessageEditHistory{}, &model.InboxMessage{})
		if err != nil {
			klog.Fatalf("数据库建表失败:%v", err)
		}
	}
	// 创建 PostgreSQL 按月 Range Partitioning
	// 全新数据库：直接将 message_table 建为分区表
	// 已有数据：创建 message_table_partitioned，等待迁移脚本
	if err := createMessagePartitions(db); err != nil {
		if !messageTableExists {
			klog.Fatalf("创建消息分区表失败（全新数据库，message_table 不存在）: %v", err)
		}
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

	// 冷库归档定时任务：每天凌晨 3 点执行
	// 使用轻量级自调度器，无需引入 robfig/cron 等外部依赖
	if chatService.IsColdEnabled() {
		go scheduleArchiveColdMessages(chatService)
	}

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
// 策略：
//  1. 如果 message_table 已是分区表（relkind = 'p'），直接补建子分区
//  2. 如果 message_table 不存在（全新数据库），直接创建名为 message_table 的分区表 + 子分区
//  3. 如果 message_table 是普通表（已有数据），创建 message_table_partitioned，等待迁移脚本
//
// 注意：分区键使用 timestamp（BIGINT 毫秒时间戳）而非 to_timestamp()，
// 因为 PostgreSQL 要求分区键表达式必须是 IMMUTABLE 的，而 to_timestamp() 是 STABLE 的
func createMessagePartitions(db *gorm.DB) error {
	// 检查 message_table 是否已经是分区表
	var isPartitioned bool
	db.Raw("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = 'message_table' AND relkind = 'p')").Scan(&isPartitioned)
	if isPartitioned {
		// 已经是分区表，补建子分区
		return createMonthlyPartitions(db, "message_table")
	}

	// 检查 message_table 是否存在（普通表）
	var tableExists bool
	db.Raw("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = 'message_table')").Scan(&tableExists)

	if !tableExists {
		// 全新数据库：直接创建名为 message_table 的分区表
		// 使用 PARTITION BY RANGE (timestamp)，按毫秒时间戳整数范围分区
		createParentSQL := `
		CREATE TABLE message_table (
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
		) PARTITION BY RANGE (timestamp)`
		if err := db.Exec(createParentSQL).Error; err != nil {
			return fmt.Errorf("创建分区表 message_table 失败: %w", err)
		}
		if err := createMonthlyPartitions(db, "message_table"); err != nil {
			return err
		}
		// 创建索引
		db.Exec("CREATE INDEX IF NOT EXISTS idx_conversation_timestamp ON message_table (conversation_id, timestamp DESC)")
		db.Exec("CREATE INDEX IF NOT EXISTS idx_conversation_seq ON message_table (conversation_id, seq)")
		db.Exec("CREATE INDEX IF NOT EXISTS idx_sender ON message_table (sender_id)")
		klog.Info("全新数据库：message_table 已创建为分区表")
		return nil
	}

	// message_table 是普通表（已有数据），创建 message_table_partitioned 等待迁移
	var parentExists bool
	db.Raw("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = 'message_table_partitioned' AND relkind = 'p')").Scan(&parentExists)
	if !parentExists {
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
		) PARTITION BY RANGE (timestamp)`
		if err := db.Exec(createParentSQL).Error; err != nil {
			return fmt.Errorf("创建分区父表失败: %w", err)
		}
	}
	if err := createMonthlyPartitions(db, "message_table_partitioned"); err != nil {
		return err
	}
	klog.Info("分区表 message_table_partitioned 及子分区已创建，请通过迁移脚本将数据从 message_table 迁移到分区表")
	return nil
}

// createMonthlyPartitions 为指定的分区父表创建前后 12 个月的子分区
// 分区边界使用毫秒时间戳整数（与 timestamp 列类型一致）
func createMonthlyPartitions(db *gorm.DB, parentTable string) error {
	now := time.Now()
	for i := -12; i <= 12; i++ {
		partitionTime := now.AddDate(0, i, 0)
		year := partitionTime.Year()
		month := int(partitionTime.Month())

		partitionName := fmt.Sprintf("message_table_y%dm%02d", year, month)
		startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		endDate := startDate.AddDate(0, 1, 0)

		// 使用毫秒时间戳作为分区边界，与 PARTITION BY RANGE (timestamp) 匹配
		startTs := startDate.Unix() * 1000
		endTs := endDate.Unix() * 1000

		sql := fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM (%d) TO (%d)",
			partitionName,
			parentTable,
			startTs,
			endTs,
		)
		if err := db.Exec(sql).Error; err != nil {
			klog.Warnf("创建分区 %s 失败（可能已存在）: %v", partitionName, err)
		}
	}
	return nil
}

// scheduleArchiveColdMessages 冷库归档定时调度器
// 每天凌晨 3 点执行一次 ArchiveColdMessages，将热库中超过 hotMonths 的消息迁移到 ClickHouse 冷库
// 实现方式：使用 time.Ticker 每小时检查一次，当当前小时为 3 且今天尚未执行时触发
func scheduleArchiveColdMessages(chatService *service.ChatService) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// 记录上次执行的日期，避免同一天重复执行
	var lastRunDate string

	for range ticker.C {
		now := time.Now()
		todayDate := now.Format("2006-01-02")

		// 仅在凌晨 3 点且今天尚未执行时触发
		if now.Hour() == 3 && todayDate != lastRunDate {
			klog.Infof("冷库归档任务开始，日期=%s", todayDate)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			if err := chatService.ArchiveColdMessages(ctx); err != nil {
				klog.Errorf("冷库归档任务失败，日期=%s, err=%v", todayDate, err)
			} else {
				klog.Infof("冷库归档任务完成，日期=%s", todayDate)
			}
			cancel()
			lastRunDate = todayDate
		}
	}
}
