package handle

import (
	"context"
	"strconv"

	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"
	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// tenantItem 是租户信息的对外结构，ID 统一以字符串下发避免前端精度丢失。
type tenantItem struct {
	TenantID    string `json:"tenant_id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	LogoURL     string `json:"logo_url"`
	OwnerID     string `json:"owner_id"`
	IsPersonal  bool   `json:"is_personal"`
	Role        int16  `json:"role"`
	MemberCount int64  `json:"member_count"`
	Status      int16  `json:"status"`
}

func toTenantItem(t *user.TenantInfo) tenantItem {
	return tenantItem{
		TenantID:    strconv.FormatInt(t.Id, 10),
		Name:        t.Name,
		Code:        t.Code,
		LogoURL:     t.LogoUrl,
		OwnerID:     strconv.FormatInt(t.OwnerId, 10),
		IsPersonal:  t.IsPersonal,
		Role:        t.Role,
		MemberCount: t.MemberCount,
		Status:      t.Status,
	}
}

type tenantCreateParam struct {
	Name    string `json:"name"`
	LogoURL string `json:"logo_url"`
}

func TenantCreate(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var param tenantCreateParam
	if err := c.BindJSON(&param); err != nil || param.Name == "" {
		hlog.CtxErrorf(ctx, "创建租户参数错误, err=%v", err)
		response.Error(c, "参数缺失", "租户名称不能为空")
		return
	}
	res, err := rpc.CreateTenant(ctx, &user.CreateTenantReq{
		UserId:  identity.Id,
		Name:    param.Name,
		LogoUrl: param.LogoURL,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "创建租户RPC失败, user_id=%d, err=%v", identity.Id, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success || res.Tenant == nil {
		response.Error(c, "操作失败", "创建租户失败")
		return
	}
	hlog.CtxInfof(ctx, "创建租户成功, user_id=%d, tenant_id=%d", identity.Id, res.Tenant.Id)
	response.Success(c, toTenantItem(res.Tenant))
}

func TenantList(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	res, err := rpc.ListTenants(ctx, &user.ListTenantsReq{UserId: identity.Id})
	if err != nil {
		hlog.CtxErrorf(ctx, "获取租户列表RPC失败, user_id=%d, err=%v", identity.Id, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	list := make([]tenantItem, 0, len(res.Tenants))
	for _, t := range res.Tenants {
		item := toTenantItem(t)
		item.Role = t.Role
		list = append(list, item)
	}
	response.Success(c, map[string]interface{}{
		"current_tenant_id": strconv.FormatInt(identity.TenantId, 10),
		"tenants":           list,
	})
}

func TenantInfo(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	tenantID := identity.TenantId
	if raw := c.Query("tenant_id"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
			tenantID = parsed
		}
	}
	if tenantID == 0 {
		response.Error(c, "参数错误", "请先选择租户")
		return
	}
	res, err := rpc.GetTenant(ctx, &user.GetTenantReq{UserId: identity.Id, TenantId: tenantID})
	if err != nil {
		hlog.CtxErrorf(ctx, "获取租户信息RPC失败, user_id=%d, tenant_id=%d, err=%v", identity.Id, tenantID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if res.Tenant == nil || res.Tenant.Id == 0 {
		response.Error(c, "操作失败", "租户不存在或无权限")
		return
	}
	response.Success(c, toTenantItem(res.Tenant))
}

type tenantSwitchParam struct {
	TenantID int64 `json:"tenant_id"`
}

func TenantSwitch(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var param tenantSwitchParam
	if err := c.BindJSON(&param); err != nil || param.TenantID == 0 {
		hlog.CtxErrorf(ctx, "切换租户参数错误, err=%v", err)
		response.Error(c, "参数缺失", "请选择要切换的租户")
		return
	}
	res, err := rpc.GetTenant(ctx, &user.GetTenantReq{UserId: identity.Id, TenantId: param.TenantID})
	if err != nil {
		hlog.CtxErrorf(ctx, "切换租户RPC失败, user_id=%d, tenant_id=%d, err=%v", identity.Id, param.TenantID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if res.Tenant == nil || res.Tenant.Id == 0 {
		response.Error(c, "操作失败", "你不属于该租户")
		return
	}
	token, expire, err := middleware.Authmiddleware.TokenGenerator(&middleware.Resp{
		Id:       identity.Id,
		Account:  identity.Account,
		TenantId: res.Tenant.Id,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "切换租户签发令牌失败, user_id=%d, tenant_id=%d, err=%v", identity.Id, param.TenantID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	hlog.CtxInfof(ctx, "切换租户成功, user_id=%d, tenant_id=%d", identity.Id, res.Tenant.Id)
	response.Success(c, map[string]interface{}{
		"token":     token,
		"expire_at": expire.Unix(),
		"tenant":    toTenantItem(res.Tenant),
	})
}

type tenantUpdateParam struct {
	Name    string `json:"name"`
	LogoURL string `json:"logo_url"`
}

func TenantUpdate(ctx context.Context, c *app.RequestContext) {
	identity, _ := middleware.CurrentUser(c)
	var param tenantUpdateParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "更新租户参数解析失败, err=%v", err)
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	res, err := rpc.UpdateTenant(ctx, &user.UpdateTenantReq{
		UserId:   identity.Id,
		TenantId: identity.TenantId,
		Name:     param.Name,
		LogoUrl:  param.LogoURL,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "更新租户RPC失败, tenant_id=%d, err=%v", identity.TenantId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "租户不存在或无权限")
		return
	}
	response.Success(c, "更新成功")
}

func TenantListMembers(ctx context.Context, c *app.RequestContext) {
	identity, _ := middleware.CurrentUser(c)
	res, err := rpc.ListTenantMembers(ctx, &user.ListTenantMembersReq{
		OperatorId: identity.Id,
		TenantId:   identity.TenantId,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "获取租户成员RPC失败, tenant_id=%d, err=%v", identity.TenantId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	type memberItem struct {
		UserID    string `json:"user_id"`
		Account   string `json:"account"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Role      int16  `json:"role"`
		DeptID    string `json:"dept_id"`
		Status    int16  `json:"status"`
	}
	list := make([]memberItem, 0, len(res.Members))
	for _, m := range res.Members {
		list = append(list, memberItem{
			UserID:    strconv.FormatInt(m.UserId, 10),
			Account:   m.Account,
			Name:      m.Name,
			AvatarURL: m.AvatarUrl,
			Role:      m.Role,
			DeptID:    strconv.FormatInt(m.DeptId, 10),
			Status:    m.Status,
		})
	}
	response.Success(c, list)
}

type tenantAddMemberParam struct {
	Account string `json:"account"`
	Role    int16  `json:"role"`
	DeptID  int64  `json:"dept_id"`
}

func TenantAddMember(ctx context.Context, c *app.RequestContext) {
	identity, _ := middleware.CurrentUser(c)
	var param tenantAddMemberParam
	if err := c.BindJSON(&param); err != nil || param.Account == "" {
		hlog.CtxErrorf(ctx, "添加租户成员参数错误, err=%v", err)
		response.Error(c, "参数缺失", "请填写成员账号")
		return
	}
	res, err := rpc.AddTenantMember(ctx, &user.AddTenantMemberReq{
		OperatorId: identity.Id,
		TenantId:   identity.TenantId,
		Account:    param.Account,
		Role:       param.Role,
		DeptId:     param.DeptID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "添加租户成员RPC失败, tenant_id=%d, account=%s, err=%v", identity.TenantId, param.Account, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "账号不存在、角色非法或无权限")
		return
	}
	hlog.CtxInfof(ctx, "添加租户成员成功, tenant_id=%d, account=%s", identity.TenantId, param.Account)
	response.Success(c, "添加成功")
}

type tenantSetRoleParam struct {
	UserID int64 `json:"user_id"`
	Role   int16 `json:"role"`
}

func TenantSetMemberRole(ctx context.Context, c *app.RequestContext) {
	identity, _ := middleware.CurrentUser(c)
	var param tenantSetRoleParam
	if err := c.BindJSON(&param); err != nil || param.UserID == 0 {
		hlog.CtxErrorf(ctx, "设置租户成员角色参数错误, err=%v", err)
		response.Error(c, "参数缺失", "请选择成员")
		return
	}
	res, err := rpc.SetTenantMemberRole(ctx, &user.SetTenantMemberRoleReq{
		OperatorId: identity.Id,
		TenantId:   identity.TenantId,
		UserId:     param.UserID,
		Role:       param.Role,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "设置租户成员角色RPC失败, tenant_id=%d, user_id=%d, err=%v", identity.TenantId, param.UserID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "成员不存在、角色非法或无权限")
		return
	}
	response.Success(c, "设置成功")
}

type tenantRemoveMemberParam struct {
	UserID int64 `json:"user_id"`
}

func TenantRemoveMember(ctx context.Context, c *app.RequestContext) {
	identity, _ := middleware.CurrentUser(c)
	var param tenantRemoveMemberParam
	if err := c.BindJSON(&param); err != nil || param.UserID == 0 {
		hlog.CtxErrorf(ctx, "移除租户成员参数错误, err=%v", err)
		response.Error(c, "参数缺失", "请选择成员")
		return
	}
	res, err := rpc.RemoveTenantMember(ctx, &user.RemoveTenantMemberReq{
		OperatorId: identity.Id,
		TenantId:   identity.TenantId,
		UserId:     param.UserID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "移除租户成员RPC失败, tenant_id=%d, user_id=%d, err=%v", identity.TenantId, param.UserID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "成员不存在或无权移除该成员")
		return
	}
	response.Success(c, "移除成功")
}

func TenantListDepartments(ctx context.Context, c *app.RequestContext) {
	identity, _ := middleware.CurrentUser(c)
	res, err := rpc.ListDepartments(ctx, &user.ListDepartmentsReq{
		OperatorId: identity.Id,
		TenantId:   identity.TenantId,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "获取部门列表RPC失败, tenant_id=%d, err=%v", identity.TenantId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	type deptItem struct {
		DeptID   string `json:"dept_id"`
		ParentID string `json:"parent_id"`
		Name     string `json:"name"`
		Order    int32  `json:"order"`
	}
	list := make([]deptItem, 0, len(res.Departments))
	for _, d := range res.Departments {
		list = append(list, deptItem{
			DeptID:   strconv.FormatInt(d.Id, 10),
			ParentID: strconv.FormatInt(d.ParentId, 10),
			Name:     d.Name,
			Order:    d.Order,
		})
	}
	response.Success(c, list)
}

type departmentCreateParam struct {
	ParentID int64  `json:"parent_id"`
	Name     string `json:"name"`
}

func TenantCreateDepartment(ctx context.Context, c *app.RequestContext) {
	identity, _ := middleware.CurrentUser(c)
	var param departmentCreateParam
	if err := c.BindJSON(&param); err != nil || param.Name == "" {
		hlog.CtxErrorf(ctx, "创建部门参数错误, err=%v", err)
		response.Error(c, "参数缺失", "部门名称不能为空")
		return
	}
	res, err := rpc.CreateDepartment(ctx, &user.CreateDepartmentReq{
		OperatorId: identity.Id,
		TenantId:   identity.TenantId,
		ParentId:   param.ParentID,
		Name:       param.Name,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "创建部门RPC失败, tenant_id=%d, err=%v", identity.TenantId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "父部门不存在或无权限")
		return
	}
	response.Success(c, map[string]interface{}{
		"dept_id": strconv.FormatInt(res.DeptId, 10),
	})
}

type departmentRenameParam struct {
	DeptID int64  `json:"dept_id"`
	Name   string `json:"name"`
}

func TenantRenameDepartment(ctx context.Context, c *app.RequestContext) {
	identity, _ := middleware.CurrentUser(c)
	var param departmentRenameParam
	if err := c.BindJSON(&param); err != nil || param.DeptID == 0 || param.Name == "" {
		hlog.CtxErrorf(ctx, "重命名部门参数错误, err=%v", err)
		response.Error(c, "参数缺失", "部门与名称不能为空")
		return
	}
	res, err := rpc.RenameDepartment(ctx, &user.RenameDepartmentReq{
		OperatorId: identity.Id,
		TenantId:   identity.TenantId,
		DeptId:     param.DeptID,
		Name:       param.Name,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "重命名部门RPC失败, dept_id=%d, err=%v", param.DeptID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "部门不存在或无权限")
		return
	}
	response.Success(c, "修改成功")
}

type departmentMoveParam struct {
	DeptID   int64 `json:"dept_id"`
	ParentID int64 `json:"parent_id"`
}

func TenantMoveDepartment(ctx context.Context, c *app.RequestContext) {
	identity, _ := middleware.CurrentUser(c)
	var param departmentMoveParam
	if err := c.BindJSON(&param); err != nil || param.DeptID == 0 {
		hlog.CtxErrorf(ctx, "移动部门参数错误, err=%v", err)
		response.Error(c, "参数缺失", "请选择部门")
		return
	}
	res, err := rpc.MoveDepartment(ctx, &user.MoveDepartmentReq{
		OperatorId: identity.Id,
		TenantId:   identity.TenantId,
		DeptId:     param.DeptID,
		ParentId:   param.ParentID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "移动部门RPC失败, dept_id=%d, err=%v", param.DeptID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "部门不存在、会形成环或无权限")
		return
	}
	response.Success(c, "移动成功")
}

type departmentRemoveParam struct {
	DeptID int64 `json:"dept_id"`
}

func TenantRemoveDepartment(ctx context.Context, c *app.RequestContext) {
	identity, _ := middleware.CurrentUser(c)
	var param departmentRemoveParam
	if err := c.BindJSON(&param); err != nil || param.DeptID == 0 {
		hlog.CtxErrorf(ctx, "删除部门参数错误, err=%v", err)
		response.Error(c, "参数缺失", "请选择部门")
		return
	}
	res, err := rpc.RemoveDepartment(ctx, &user.RemoveDepartmentReq{
		OperatorId: identity.Id,
		TenantId:   identity.TenantId,
		DeptId:     param.DeptID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "删除部门RPC失败, dept_id=%d, err=%v", param.DeptID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "部门不存在或无权限")
		return
	}
	response.Success(c, "删除成功")
}

type setMemberDeptParam struct {
	UserID int64 `json:"user_id"`
	DeptID int64 `json:"dept_id"`
}

func TenantSetMemberDepartment(ctx context.Context, c *app.RequestContext) {
	identity, _ := middleware.CurrentUser(c)
	var param setMemberDeptParam
	if err := c.BindJSON(&param); err != nil || param.UserID == 0 {
		hlog.CtxErrorf(ctx, "调整成员部门参数错误, err=%v", err)
		response.Error(c, "参数缺失", "请选择成员")
		return
	}
	res, err := rpc.SetMemberDepartment(ctx, &user.SetMemberDepartmentReq{
		OperatorId: identity.Id,
		TenantId:   identity.TenantId,
		UserId:     param.UserID,
		DeptId:     param.DeptID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "调整成员部门RPC失败, user_id=%d, dept_id=%d, err=%v", param.UserID, param.DeptID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "成员或部门不存在、无权限")
		return
	}
	response.Success(c, "设置成功")
}
