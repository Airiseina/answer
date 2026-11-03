package dal

import (
	"strings"
	"testing"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"
	"github.com/Airiseina/answer/pkg/tenant"
	"github.com/Airiseina/answer/pkg/tenant/tenanttest"

	"gorm.io/gorm"
)

const (
	tenantA int64 = 10001
	tenantB int64 = 10002
)

func newDryRunDao(t *testing.T) *botDao {
	t.Helper()
	return &botDao{db: tenanttest.NewMySQL(t)}
}

// assertTenantScoped 断言 SQL 里出现 tenant_id 条件，且绑定参数包含期望租户。
func assertTenantScoped(t *testing.T, sql string, vars []interface{}, want int64) {
	t.Helper()
	if !strings.Contains(sql, "tenant_id") {
		t.Fatalf("查询缺少租户过滤条件: %s", sql)
	}
	for _, v := range vars {
		if id, ok := v.(int64); ok && id == want {
			return
		}
	}
	t.Fatalf("绑定参数中未包含租户 %d，实际 %v（SQL: %s）", want, vars, sql)
}

// TestGetBotScopesByTenant：A 租户不能读到 B 租户的 Bot —— 查询必须带 tenant_id。
func TestGetBotScopesByTenant(t *testing.T) {
	dao := newDryRunDao(t)
	var bot model.Bot

	sql, vars := tenanttest.Capture(t, dao.db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx, tenantA).Where("id = ?", 9527).First(&bot)
	})
	assertTenantScoped(t, sql, vars, tenantA)
}

// TestUpdateAndDeleteScopedByTenant：写操作同样必须带租户条件，避免跨租户篡改。
func TestUpdateAndDeleteScopedByTenant(t *testing.T) {
	dao := newDryRunDao(t)

	sql, vars := tenanttest.Capture(t, dao.db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx.Model(&model.Bot{}), tenantB).Where("id = ?", 9527).
			Updates(map[string]interface{}{"model": "gpt-4o"})
	})
	assertTenantScoped(t, sql, vars, tenantB)

	sql, vars = tenanttest.Capture(t, dao.db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx, tenantB).Where("id = ?", 9527).Delete(&model.Bot{})
	})
	assertTenantScoped(t, sql, vars, tenantB)
}

// TestUserBotsListScopedByTenant：列表读也必须限定在调用租户内。
func TestUserBotsListScopedByTenant(t *testing.T) {
	dao := newDryRunDao(t)
	var bots []model.Bot

	sql, vars := tenanttest.Capture(t, dao.db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx, tenantA).
			Where("creator_id = ? OR is_system = ?", 7, true).Find(&bots)
	})
	assertTenantScoped(t, sql, vars, tenantA)
}

// TestZeroTenantSkipsFilter：tenantID <= 0 表示系统内部任务，约定不加过滤，
// 因此这里断言 SQL 中不出现 tenant_id 条件，把这个「宽松约定」固化成显式契约。
func TestZeroTenantSkipsFilter(t *testing.T) {
	dao := newDryRunDao(t)
	var bot model.Bot
	sql, _ := tenanttest.Capture(t, dao.db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx, 0).Where("id = ?", 9527).First(&bot)
	})
	if strings.Contains(sql, "tenant_id") {
		t.Fatalf("tenantID<=0 时应不加过滤，实际: %s", sql)
	}
}
