package dal

import (
	"strings"
	"testing"

	"github.com/Airiseina/answer/kitex_service/chat_service/internal/model"
	"github.com/Airiseina/answer/pkg/tenant"
	"github.com/Airiseina/answer/pkg/tenant/tenanttest"

	"gorm.io/gorm"
)

const (
	tenantA int64 = 30001
	tenantB int64 = 30002
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

// TestMessageScopedByTenant：跨租户读不到对方消息（PostgreSQL 热库）。
func TestMessageScopedByTenant(t *testing.T) {
	db := tenanttest.NewPostgres(t)
	var msg model.Message

	sql, vars := tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx, tenantA).Where("conversation_id = ?", 9527).First(&msg)
	})
	assertTenantScoped(t, sql, vars, tenantA)
}

// TestConversationScopedByTenant：会话与会话成员同样强制租户过滤。
func TestConversationScopedByTenant(t *testing.T) {
	db := tenanttest.NewPostgres(t)
	var conv model.Conversation

	sql, vars := tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx, tenantB).Where("id = ?", 9527).First(&conv)
	})
	assertTenantScoped(t, sql, vars, tenantB)

	var members []model.ConversationMember
	sql, vars = tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx, tenantB).Where("conversation_id = ?", 9527).Find(&members)
	})
	assertTenantScoped(t, sql, vars, tenantB)
}

// TestChatZeroTenantSkipsFilter：系统内部任务（tenantID<=0）不加过滤。
func TestChatZeroTenantSkipsFilter(t *testing.T) {
	db := tenanttest.NewPostgres(t)
	var msg model.Message
	sql, _ := tenanttest.Capture(t, db, func(tx *gorm.DB) *gorm.DB {
		return tenant.Scope(tx, 0).Where("conversation_id = ?", 9527).First(&msg)
	})
	if strings.Contains(sql, "tenant_id") {
		t.Fatalf("tenantID<=0 时应不加过滤，实际: %s", sql)
	}
}
