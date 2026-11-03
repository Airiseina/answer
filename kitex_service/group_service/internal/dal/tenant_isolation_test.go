package dal

import (
	"strings"
	"testing"

	"github.com/Airiseina/answer/kitex_service/group_service/internal/model"
	"github.com/Airiseina/answer/pkg/tenant"
	"github.com/Airiseina/answer/pkg/tenant/tenanttest"

	"gorm.io/gorm"
)

const (
	tenantA int64 = 20001
	tenantB int64 = 20002
)

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

// TestGroupScopedByTenant：A 租户读不到 B 租户的群与群成员。
func TestGroupScopedByTenant(t *testing.T) {
	db := tenanttest.NewMySQL(t)
	var group model.Group

	sql, vars := tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx, tenantA).Where("id = ?", 9527).First(&group)
	})
	assertTenantScoped(t, sql, vars, tenantA)

	var members []model.GroupMember
	sql, vars = tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx, tenantB).Where("group_id = ?", 9527).Find(&members)
	})
	assertTenantScoped(t, sql, vars, tenantB)
}

// TestGroupWriteScopedByTenant：改/删群信息必须带租户条件，避免跨租户篡改。
func TestGroupWriteScopedByTenant(t *testing.T) {
	db := tenanttest.NewMySQL(t)

	sql, vars := tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx.Model(&model.Group{}), tenantB).Where("id = ?", 9527).
			Updates(map[string]interface{}{"notice": "hijacked"})
	})
	assertTenantScoped(t, sql, vars, tenantB)

	sql, vars = tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx, tenantB).Where("id = ?", 9527).Delete(&model.GroupMember{})
	})
	assertTenantScoped(t, sql, vars, tenantB)
}

// TestGroupZeroTenantSkipsFilter：tenantID<=0 表示系统内部任务，约定不加过滤。
func TestGroupZeroTenantSkipsFilter(t *testing.T) {
	db := tenanttest.NewMySQL(t)
	var group model.Group
	sql, _ := tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx, 0).Where("id = ?", 9527).First(&group)
	})
	if strings.Contains(sql, "tenant_id") {
		t.Fatalf("tenantID<=0 时应不加过滤，实际: %s", sql)
	}
}
