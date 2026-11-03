package middleware

import (
	"context"

	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"
	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

const tenantRoleAdmin = int16(1)

// TenantRoleKey 保存当前登录态在所在租户内的角色，供业务 handler 复用。
const TenantRoleKey = "tenant_role"

// RequireTenantAdmin 限定接口仅允许「当前租户」的 owner/admin 访问。
func RequireTenantAdmin() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		identity, ok := CurrentUser(c)
		if !ok {
			response.Error(c, "登录过期", "请重新登录")
			c.Abort()
			return
		}
		if identity.TenantId == 0 {
			response.Error(c, "无权限", "请先选择要操作的租户")
			c.Abort()
			return
		}
		res, err := rpc.GetTenant(ctx, &user.GetTenantReq{
			UserId:   identity.Id,
			TenantId: identity.TenantId,
		})
		if err != nil {
			hlog.CtxErrorf(ctx, "校验租户管理员失败, user_id=%d, tenant_id=%d, err=%v", identity.Id, identity.TenantId, err)
			response.Error(c, "系统繁忙", "请稍后重试")
			c.Abort()
			return
		}
		if res.Tenant == nil || res.Tenant.Role < tenantRoleAdmin {
			response.Error(c, "无权限", "仅租户管理员可执行该操作")
			c.Abort()
			return
		}
		c.Set(TenantRoleKey, res.Tenant.Role)
		c.Next(ctx)
	}
}
