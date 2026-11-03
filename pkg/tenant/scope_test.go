package tenant

import (
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// fakeDialector 让 GORM 在无数据库依赖的情况下构建 SQL（配合 DryRun 使用）。
type fakeDialector struct{}

func (fakeDialector) Name() string { return "fake" }

func (fakeDialector) Initialize(db *gorm.DB) error {
	callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{})
	return nil
}

func (fakeDialector) Migrator(*gorm.DB) gorm.Migrator { return nil }

func (fakeDialector) DataTypeOf(*schema.Field) string { return "text" }

func (fakeDialector) DefaultValueOf(*schema.Field) clause.Expression {
	return clause.Expr{SQL: "NULL"}
}

func (fakeDialector) BindVarTo(writer clause.Writer, _ *gorm.Statement, _ interface{}) {
	_ = writer.WriteByte('?')
}

func (fakeDialector) QuoteTo(writer clause.Writer, str string) {
	_, _ = writer.WriteString(str)
}

func (fakeDialector) Explain(sql string, _ ...interface{}) string { return sql }

type botRow struct {
	ID       int64
	TenantID int64
}

func (botRow) TableName() string { return "bot_table" }

type conversationMemberRow struct {
	ConversationID int64
	UserID         int64
	TenantID       int64
}

func (conversationMemberRow) TableName() string { return "conversation_member" }

func newDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(fakeDialector{}, &gorm.Config{})
	if err != nil {
		t.Fatalf("构造 DryRun DB 失败: %v", err)
	}
	return db.Session(&gorm.Session{DryRun: true})
}

func TestScopeAppendsTenantCondition(t *testing.T) {
	db := newDryRunDB(t)

	var rows []botRow
	tx := Scope(db.Model(&botRow{}), 42).Find(&rows)
	if tx.Error != nil {
		t.Fatalf("构建查询失败: %v", tx.Error)
	}
	stmt := tx.Statement
	query := stmt.SQL.String()
	if !strings.Contains(query, "tenant_id") {
		t.Fatalf("期望查询带 tenant_id 过滤，实际: %s", query)
	}
	if !strings.Contains(query, "?") || len(stmt.Vars) == 0 || stmt.Vars[0] != int64(42) {
		t.Fatalf("期望绑定租户 ID 42，实际: %v", stmt.Vars)
	}
}

func TestScopeKeepsQueryWhenTenantMissing(t *testing.T) {
	db := newDryRunDB(t)

	var rows []botRow
	tx := Scope(db.Model(&botRow{}), 0).Find(&rows)
	if tx.Error != nil {
		t.Fatalf("构建查询失败: %v", tx.Error)
	}
	stmt := tx.Statement
	if strings.Contains(stmt.SQL.String(), "tenant_id") {
		t.Fatalf("租户为空时不应附加过滤，实际: %s", stmt.SQL.String())
	}
}

func TestColumnScopeUsesGivenColumn(t *testing.T) {
	db := newDryRunDB(t)

	var rows []conversationMemberRow
	tx := ColumnScope(db.Model(&conversationMemberRow{}), "m.tenant_id", 7).Find(&rows)
	if tx.Error != nil {
		t.Fatalf("构建查询失败: %v", tx.Error)
	}
	stmt := tx.Statement
	if !strings.Contains(stmt.SQL.String(), "m.tenant_id") {
		t.Fatalf("期望使用指定列名，实际: %s", stmt.SQL.String())
	}
}
