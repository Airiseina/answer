package dal

import (
	"strings"
	"testing"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/model"
	"github.com/Airiseina/answer/pkg/tenant"
	"github.com/Airiseina/answer/pkg/tenant/tenanttest"

	"gorm.io/gorm"
)

const (
	tenantA int64 = 40001
	tenantB int64 = 40002
)

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

// TestKnowledgeBaseScopedByTenant：A 租户读不到 B 租户的知识库。
func TestKnowledgeBaseScopedByTenant(t *testing.T) {
	db := tenanttest.NewMySQL(t)
	var kb model.KnowledgeBase

	sql, vars := tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx.Where("id = ?", 9527), tenantA).First(&kb)
	})
	assertTenantScoped(t, sql, vars, tenantA)
}

// TestKnowledgeWriteScopedByTenant：文档改/删必须限定在调用租户内。
func TestKnowledgeWriteScopedByTenant(t *testing.T) {
	db := tenanttest.NewMySQL(t)

	sql, vars := tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx.Model(&model.KbDocument{}).Where("id = ?", 9527), tenantB).
			Updates(map[string]interface{}{"status": 1})
	})
	assertTenantScoped(t, sql, vars, tenantB)

	sql, vars = tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx.Where("kb_id = ?", 9527), tenantB).Delete(&model.KbDocument{})
	})
	assertTenantScoped(t, sql, vars, tenantB)
}

// TestKnowledgeZeroTenantSkipsFilter：系统内部任务（tenantID<=0）不加过滤。
func TestKnowledgeZeroTenantSkipsFilter(t *testing.T) {
	db := tenanttest.NewMySQL(t)
	var kb model.KnowledgeBase
	sql, _ := tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx.Where("id = ?", 9527), 0).First(&kb)
	})
	if strings.Contains(sql, "tenant_id") {
		t.Fatalf("tenantID<=0 时应不加过滤，实际: %s", sql)
	}
}
