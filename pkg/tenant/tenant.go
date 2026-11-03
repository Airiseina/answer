// Package tenant 提供多租户请求上下文的透传能力。
//
// 租户 ID 由 api_gateway 从 JWT claims 解析后写入 context，
// 再经 RPC 透传到各 Kitex 服务的 DAO 层，作为强制过滤条件。
package tenant

import "context"

// HeaderTenantID 是跨端透传租户 ID 的 HTTP 头名称。
const HeaderTenantID = "X-Tenant-Id"

// PlatformTenantID 是系统 Bot 与平台级资源归属的固定租户 ID。
//
// 它刻意避开 snowflake 生成的 ID（13 位以上），不会与业务租户冲突。
const PlatformTenantID int64 = 1

type ctxKey struct{}

// WithTenant 把租户 ID 写入 context。
func WithTenant(ctx context.Context, tenantID int64) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKey{}, tenantID)
}

// FromContext 读取租户 ID，缺失时返回 (0, false)。
func FromContext(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	v, ok := ctx.Value(ctxKey{}).(int64)
	if !ok {
		return 0, false
	}
	return v, true
}

// MustFromContext 读取租户 ID，缺失时 panic。
//
// 仅用于调用链上游已保证租户一定存在的内部路径；对外入口请使用 FromContext 做判断。
func MustFromContext(ctx context.Context) int64 {
	id, ok := FromContext(ctx)
	if !ok {
		panic("tenant: context 中缺少租户 ID")
	}
	return id
}

// IsValid 判断租户 ID 是否合法（必须为正数）。
func IsValid(tenantID int64) bool {
	return tenantID > 0
}
