package dal

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"
	"github.com/Airiseina/answer/pkg/tenant/tenanttest"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// marketSQLRecorder 捕获 DryRun 会话编译出的语句。
//
// 市场 DAO 的关键约定（全局可见 vs 发布者隔离 vs 安装记录隔离）无法从返回值观察，
// 只能通过断言语句里的过滤条件来守住，避免后续改动把隔离条件悄悄删掉。
type marketSQLRecorder struct {
	logger.Interface
	stmts []string
}

func (r *marketSQLRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	if sql != "" {
		r.stmts = append(r.stmts, sql)
	}
}

func (r *marketSQLRecorder) last() string {
	if len(r.stmts) == 0 {
		return ""
	}
	return r.stmts[len(r.stmts)-1]
}

func newMarketDB(t *testing.T) (*gorm.DB, *marketSQLRecorder) {
	t.Helper()
	rec := &marketSQLRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	return tenanttest.NewMySQLWithLogger(t, rec), rec
}

// wantSQL 断言语句里出现全部期望片段；比较前先去掉反引号，避免受方言转义影响。
func wantSQL(t *testing.T, sql string, parts ...string) {
	t.Helper()
	normalized := strings.ReplaceAll(sql, "`", "")
	for _, part := range parts {
		if !strings.Contains(normalized, part) {
			t.Fatalf("语句缺少片段 %q，实际: %s", part, normalized)
		}
	}
}

// TestMarketListPublishedIsGlobalAndOnlyPublished：市场浏览是全局可见的，
// 但只允许看到已发布条目，因此语句必须带 status = 2，且不出现租户过滤。
func TestMarketListPublishedIsGlobalAndOnlyPublished(t *testing.T) {
	db, rec := newMarketDB(t)
	dao := &marketBotDao{db: db}

	if _, _, err := dao.ListPublished("周报", "", 0, 20); err != nil {
		t.Fatalf("ListPublished 出错: %v", err)
	}
	wantSQL(t, rec.last(), "status = 2", "install_count DESC")
	if strings.Contains(strings.ReplaceAll(rec.last(), "`", ""), "tenant_id") {
		t.Fatalf("市场浏览应对全平台可见，不应附加租户过滤: %s", rec.last())
	}
}

// TestMarketPublisherQueriesScopedByTenant：发布者管理自己的条目必须限定租户，
// 否则 A 租户可以读到/改到 B 租户的发布内容。
func TestMarketPublisherQueriesScopedByTenant(t *testing.T) {
	db, rec := newMarketDB(t)
	dao := &marketBotDao{db: db}

	if _, err := dao.GetMarketBotForPublisher(tenantA, 9527); err != nil {
		t.Fatalf("GetMarketBotForPublisher 出错: %v", err)
	}
	wantSQL(t, rec.last(), "tenant_id = 10001", "id = 9527")

	if _, _, err := dao.ListByPublisher(tenantB, 7, model.MarketStatusPending, 0, 20); err != nil {
		t.Fatalf("ListByPublisher 出错: %v", err)
	}
	wantSQL(t, rec.last(), "tenant_id = 10002", "publisher_id = 7", "status = 1")
}

// TestMarketListByPublisherAllStatusWhenSentinel：status 传负数表示不过滤，
// 用于「我的发布」一次性看到草稿/待审/驳回/下架全部状态。
func TestMarketListByPublisherAllStatusWhenSentinel(t *testing.T) {
	db, rec := newMarketDB(t)
	dao := &marketBotDao{db: db}

	if _, _, err := dao.ListByPublisher(tenantA, 7, -1, 0, 20); err != nil {
		t.Fatalf("ListByPublisher 出错: %v", err)
	}
	sql := strings.ReplaceAll(rec.last(), "`", "")
	if strings.Contains(sql, "status =") {
		t.Fatalf("status 为负值时应返回全部状态，不应追加状态条件: %s", sql)
	}
	wantSQL(t, sql, "tenant_id = 10001", "publisher_id = 7")
}

// TestMarketReviewListIsGlobal：人工审核是平台侧动作，需要跨租户看到所有待审条目，
// 因此这里刻意不加租户条件（与发布者隔离形成对照）。
func TestMarketReviewListIsGlobal(t *testing.T) {
	db, rec := newMarketDB(t)
	dao := &marketBotDao{db: db}

	if _, _, err := dao.ListByStatus(model.MarketStatusPending, 0, 20); err != nil {
		t.Fatalf("ListByStatus 出错: %v", err)
	}
	wantSQL(t, rec.last(), "status = 1")
	if strings.Contains(strings.ReplaceAll(rec.last(), "`", ""), "tenant_id") {
		t.Fatalf("审核列表应跨租户可见，不应附加租户过滤: %s", rec.last())
	}
}

// TestMarketInstallScopedByTenant：安装判定（是否已装过）必须限定「租户 + 用户」，
// 否则同一用户在另一个租户下会被误判为已安装而拿不到自己的 Bot 实例。
func TestMarketInstallScopedByTenant(t *testing.T) {
	db, rec := newMarketDB(t)
	dao := &marketInstallDao{db: db}

	if _, err := dao.GetUserInstall(tenantA, 7, 9527); err != nil {
		t.Fatalf("GetUserInstall 出错: %v", err)
	}
	wantSQL(t, rec.last(), "tenant_id = 10001", "user_id = 7", "market_bot_id = 9527")
}

// TestMarketBotModelCarriesNoKnowledgeOrMCP：市场只分发 Bot 定义，
// 知识库与 MCP 不随安装迁移，这里用反射把「不分发」固化成结构体契约。
func TestMarketBotModelCarriesNoKnowledgeOrMCP(t *testing.T) {
	forbidden := []string{"kb", "knowledge", "mcp"}
	typ := reflect.TypeOf(model.MarketBot{})
	for i := 0; i < typ.NumField(); i++ {
		name := strings.ToLower(typ.Field(i).Name)
		tag := strings.ToLower(typ.Field(i).Tag.Get("json"))
		for _, word := range forbidden {
			if strings.Contains(name, word) || strings.Contains(tag, word) {
				t.Fatalf("市场分发条目不允许携带 %s 相关字段，发现: %s", word, typ.Field(i).Name)
			}
		}
	}
}

// TestUpsertRatingMaintainsSummary：评分写入必须同时维护市场条目的评分汇总列。
//
// DryRun 只编译语句、不真正执行，因此 `First(&existing)` 不会返回
// gorm.ErrRecordNotFound，代码会落入「改分」分支：existing 为零值，
// delta = 新分 - 0。这里就据此断言两条语句都被正确构造：
// 一是更新评分行的分值，二是按增量累加市场条目的 rating_sum。
func TestUpsertRatingMaintainsSummary(t *testing.T) {
	db, rec := newMarketDB(t)
	// Transaction 需要真实连接才能开启事务，这里换成一个不落库的桩，
	// 让 DryRun 也能把事务内的语句编译出来。
	db.Statement.ConnPool = dryTxPool{}
	dao := &marketRatingDao{db: db}

	err := dao.UpsertRating(model.MarketBotRating{
		ID:          1,
		TenantID:    tenantA,
		MarketBotID: 9527,
		UserID:      7,
		Score:       5,
	})
	if err != nil {
		t.Fatalf("UpsertRating 出错: %v", err)
	}
	joined := strings.ReplaceAll(strings.Join(rec.stmts, "\n"), "`", "")
	wantSQL(t, joined, "UPDATE market_bot_rating", "score")
	wantSQL(t, joined, "UPDATE market_bot", "rating_sum", "+ 5")
}

// dryTxPool 让 DryRun 会话也能走 Transaction 分支：它只负责返回一个可用的
// 事务句柄，真正的语句依然不会执行（DryRun 已在查询回调里拦下）。
type dryTxPool struct{}

func (dryTxPool) PrepareContext(context.Context, string) (*sql.Stmt, error) { return nil, nil }
func (dryTxPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, nil
}
func (dryTxPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, nil
}
func (dryTxPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row { return nil }

// BeginTx 返回指针类型：GORM 的 Commit/Rollback 会对事务句柄做 reflect.ValueOf(x).IsNil()，
// 值类型（结构体）会在该调用上 panic。
func (dryTxPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &dryTx{}, nil
}

type dryTx struct{}

func (dryTx) PrepareContext(context.Context, string) (*sql.Stmt, error) { return nil, nil }
func (dryTx) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, nil
}
func (dryTx) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, nil
}
func (dryTx) QueryRowContext(context.Context, string, ...interface{}) *sql.Row { return nil }
func (dryTx) Commit() error                                                    { return nil }
func (dryTx) Rollback() error                                                  { return nil }
