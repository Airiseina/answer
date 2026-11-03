package tenant

import (
	"context"
	"testing"
)

func TestWithTenantAndFromContext(t *testing.T) {
	ctx := WithTenant(context.Background(), 12345)
	id, ok := FromContext(ctx)
	if !ok {
		t.Fatalf("期望从 context 中读到租户 ID")
	}
	if id != 12345 {
		t.Fatalf("租户 ID 不匹配，期望 12345，实际 %d", id)
	}
}

func TestFromContextMissing(t *testing.T) {
	if id, ok := FromContext(context.Background()); ok || id != 0 {
		t.Fatalf("空 context 应返回 (0,false)，实际 (%d,%v)", id, ok)
	}
	if id, ok := FromContext(nil); ok || id != 0 {
		t.Fatalf("nil context 应返回 (0,false)，实际 (%d,%v)", id, ok)
	}
}

func TestMustFromContextPanicsWhenMissing(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("缺少租户时 MustFromContext 应 panic")
		}
	}()
	_ = MustFromContext(context.Background())
}

func TestMustFromContext(t *testing.T) {
	ctx := WithTenant(context.Background(), PlatformTenantID)
	if got := MustFromContext(ctx); got != PlatformTenantID {
		t.Fatalf("期望 %d，实际 %d", PlatformTenantID, got)
	}
}

func TestIsValid(t *testing.T) {
	if IsValid(0) || IsValid(-1) {
		t.Fatalf("非正数租户 ID 应判为非法")
	}
	if !IsValid(PlatformTenantID) {
		t.Fatalf("平台租户 ID 应合法")
	}
}

func TestWithTenantOverwrite(t *testing.T) {
	ctx := WithTenant(context.Background(), 1)
	ctx = WithTenant(ctx, 2)
	if got := MustFromContext(ctx); got != 2 {
		t.Fatalf("重复写入应以最后一次为准，期望 2，实际 %d", got)
	}
}
