package handle

import (
	"context"
	"strconv"

	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"
	"github.com/Airiseina/answer/kitex_service/admin_service/kitex_gen/admin"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// AdminProfile 返回当前登录的管理员信息，用于后台前端启动时校验登录态。
func AdminProfile(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	res, err := rpc.AdminGet(ctx, &admin.GetAdminReq{AdminId: identity.Id})
	if err != nil {
		hlog.CtxErrorf(ctx, "查询管理员信息失败, admin_id=%d, err=%v", identity.Id, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if res.Admin == nil {
		response.Error(c, "账号不存在", "请重新登录")
		return
	}
	response.Success(c, map[string]interface{}{
		"admin_id":      strconv.FormatInt(res.Admin.Id, 10),
		"account":       res.Admin.Account,
		"name":          res.Admin.Name,
		"role":          res.Admin.Role,
		"last_login_at": res.Admin.LastLoginAt,
	})
}

type adminChangePasswordParam struct {
	OldPassword    string `json:"old_password"`
	UpdatePassword string `json:"update_password"`
}

func AdminChangePassword(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var param adminChangePasswordParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "修改管理员密码参数解析失败, err=%v", err)
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	if param.OldPassword == "" || param.UpdatePassword == "" {
		response.Error(c, "参数缺失", "原密码与新密码不能为空")
		return
	}
	res, err := rpc.AdminChangePassword(ctx, &admin.ChangePasswordReq{
		AdminId:        identity.Id,
		OldPassword:    param.OldPassword,
		UpdatePassword: param.UpdatePassword,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "修改管理员密码RPC失败, admin_id=%d, err=%v", identity.Id, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "原密码错误")
		return
	}
	response.Success(c, "修改成功")
}

func AdminListAccounts(ctx context.Context, c *app.RequestContext) {
	res, err := rpc.AdminList(ctx, &admin.ListAdminsReq{})
	if err != nil {
		hlog.CtxErrorf(ctx, "查询管理员列表RPC失败, err=%v", err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	type adminItem struct {
		AdminID     string `json:"admin_id"`
		Account     string `json:"account"`
		Name        string `json:"name"`
		Role        string `json:"role"`
		Status      int16  `json:"status"`
		LastLoginAt int64  `json:"last_login_at"`
	}
	list := make([]adminItem, 0, len(res.Admins))
	for _, a := range res.Admins {
		list = append(list, adminItem{
			AdminID:     strconv.FormatInt(a.Id, 10),
			Account:     a.Account,
			Name:        a.Name,
			Role:        a.Role,
			Status:      a.Status,
			LastLoginAt: a.LastLoginAt,
		})
	}
	response.Success(c, list)
}

type adminCreateParam struct {
	Account  string `json:"account"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

func AdminCreateAccount(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var param adminCreateParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "创建管理员参数解析失败, err=%v", err)
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	if param.Account == "" || param.Name == "" || param.Role == "" || param.Password == "" {
		response.Error(c, "参数缺失", "账号、名称、角色、口令均不能为空")
		return
	}
	res, err := rpc.AdminCreate(ctx, &admin.CreateAdminReq{
		OperatorId: identity.Id,
		Account:    param.Account,
		Name:       param.Name,
		Role:       param.Role,
		Password:   param.Password,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "创建管理员RPC失败, account=%s, err=%v", param.Account, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "账号已存在、角色非法或无权限")
		return
	}
	response.Success(c, map[string]interface{}{
		"admin_id": strconv.FormatInt(res.AdminId, 10),
	})
}

type adminStatusParam struct {
	AdminID int64 `json:"admin_id"`
	Status  int16 `json:"status"`
}

func AdminUpdateAccountStatus(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var param adminStatusParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "更新管理员状态参数解析失败, err=%v", err)
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	if param.AdminID == 0 {
		response.Error(c, "参数缺失", "请选择账号")
		return
	}
	res, err := rpc.AdminUpdateStatus(ctx, &admin.UpdateAdminStatusReq{
		OperatorId: identity.Id,
		AdminId:    param.AdminID,
		Status:     param.Status,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "更新管理员状态RPC失败, admin_id=%d, err=%v", param.AdminID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "账号不存在、不能停用自己或无权限")
		return
	}
	response.Success(c, "更新成功")
}
