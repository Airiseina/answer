package model

import (
	"encoding/json"
	"testing"
)

// TestTenantBeforeSaveNormalizesEmptySettings：tenant.settings 是 MySQL JSON 列，
// 空字符串会被 MySQL 直接拒绝（Error 3140: The document is empty），
// 因此写入前必须归一化为合法 JSON 文档，否则新建租户（含启动时的平台租户）必然失败。
func TestTenantBeforeSaveNormalizesEmptySettings(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"零值空串", ""},
		{"纯空白", " \n\t "},
	}
	for _, c := range cases {
		tenant := &Tenant{Name: "平台", Code: "platform", Settings: c.in}
		if err := tenant.BeforeSave(nil); err != nil {
			t.Fatalf("%s: 归一化返回错误: %v", c.name, err)
		}
		if tenant.Settings != DefaultTenantSettings {
			t.Fatalf("%s: 期望归一化为 %q，实际 %q", c.name, DefaultTenantSettings, tenant.Settings)
		}
		if !json.Valid([]byte(tenant.Settings)) {
			t.Fatalf("%s: settings 不是合法 JSON: %q", c.name, tenant.Settings)
		}
	}
}

// TestTenantBeforeSaveKeepsExplicitSettings：调用方显式传入的 JSON 不能被钩子覆盖。
func TestTenantBeforeSaveKeepsExplicitSettings(t *testing.T) {
	const explicit = `{"theme":"dark"}`
	tenant := &Tenant{Name: "某组织", Code: "org_demo", Settings: explicit}
	if err := tenant.BeforeSave(nil); err != nil {
		t.Fatalf("归一化返回错误: %v", err)
	}
	if tenant.Settings != explicit {
		t.Fatalf("显式配置被覆盖: %q", tenant.Settings)
	}
}
