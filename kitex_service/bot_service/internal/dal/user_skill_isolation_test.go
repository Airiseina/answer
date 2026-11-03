package dal

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"
	"github.com/Airiseina/answer/pkg/tenant/tenanttest"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// skillSQLRecorder 捕获 DryRun 会话编译出的语句。
//
// 个人 Skill 的隔离约定（严格按租户 + 上传者归属）无法从返回值观察，
// 只能通过断言语句里的过滤条件守住，避免后续改动把隔离条件悄悄删掉。
type skillSQLRecorder struct {
	logger.Interface
	stmts []string
}

func (r *skillSQLRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	if sql != "" {
		r.stmts = append(r.stmts, sql)
	}
}

func (r *skillSQLRecorder) last() string {
	if len(r.stmts) == 0 {
		return ""
	}
	return r.stmts[len(r.stmts)-1]
}

func newSkillDB(t *testing.T) (*gorm.DB, *skillSQLRecorder) {
	t.Helper()
	rec := &skillSQLRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	return tenanttest.NewMySQLWithLogger(t, rec), rec
}

func wantSkillSQL(t *testing.T, sql string, parts ...string) {
	t.Helper()
	normalized := strings.ReplaceAll(sql, "`", "")
	for _, part := range parts {
		if !strings.Contains(normalized, part) {
			t.Fatalf("语句缺少片段 %q，实际: %s", part, normalized)
		}
	}
}

// TestUserSkillGetScopesTenant：按 ID 读取 Skill 必须同时带租户条件，防止跨租户越权读取。
func TestUserSkillGetScopesTenant(t *testing.T) {
	db, rec := newSkillDB(t)
	dao := NewUserSkillDao(db)
	if _, err := dao.GetUserSkill(9, 42); err != nil {
		t.Fatalf("DryRun 查询不应失败: %v", err)
	}
	wantSkillSQL(t, rec.last(), "user_skill", "id = 42", "tenant_id = 9")
}

// TestUserSkillListScopesOwner：列表必须按「租户 + 上传者」过滤，同名不同用户互不可见。
func TestUserSkillListScopesOwner(t *testing.T) {
	db, rec := newSkillDB(t)
	dao := NewUserSkillDao(db)
	if _, err := dao.ListUserSkills(9, 7); err != nil {
		t.Fatalf("DryRun 查询不应失败: %v", err)
	}
	wantSkillSQL(t, rec.last(), "tenant_id = 9", "owner_id = 7")
}

// TestBotSkillDeleteScopesTenant：解绑必须带租户与 Bot、Skill 三元条件。
func TestBotSkillDeleteScopesTenant(t *testing.T) {
	db, rec := newSkillDB(t)
	dao := NewUserSkillDao(db)
	if err := dao.DeleteBotSkill(9, 42, 100); err != nil {
		t.Fatalf("DryRun 删除不应失败: %v", err)
	}
	wantSkillSQL(t, rec.last(), "tenant_id = 9", "bot_id = 42", "skill_id = 100")
}

// TestSkillDeleteBySkillFiltersTenant：删除 Skill 时清理绑定必须限定租户，
// 否则会误删其他租户中同 ID 的绑定记录。
func TestSkillDeleteBySkillFiltersTenant(t *testing.T) {
	db, rec := newSkillDB(t)
	dao := NewUserSkillDao(db)
	if err := dao.DeleteBotSkillsBySkill(9, 100); err != nil {
		t.Fatalf("DryRun 删除不应失败: %v", err)
	}
	wantSkillSQL(t, rec.last(), "bot_skill", "tenant_id = 9", "skill_id = 100")
}

// TestUserSkillModelTableNames：表名固定为复数外的约定名，避免 GORM 复数化差异。
func TestUserSkillModelTableNames(t *testing.T) {
	cases := map[string]string{
		model.UserSkill{}.TableName():     "user_skill",
		model.UserSkillFile{}.TableName(): "user_skill_file",
		model.BotSkill{}.TableName():      "bot_skill",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("表名不符合约定: got=%q want=%q", got, want)
		}
	}
}
