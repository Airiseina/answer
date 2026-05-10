package handle

import (
	"api_gateway/middleware"
	"api_gateway/response"
	"api_gateway/rpc"
	"context"
	"group_service/kitex_gen/group"

	"github.com/cloudwego/hertz/pkg/app"
)

type createParam struct {
	Name           string  `json:"name"`
	InitialMembers []int64 `json:"initial_members"`
}

func CreateGroup(ctx context.Context, c *app.RequestContext) {
	var param createParam
	if err := c.BindJSON(&param); err != nil {
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	id := userInfo.Id
	req := &group.CreateGroupReq{
		CreatorId:      id,
		Name:           param.Name,
		InitialMembers: param.InitialMembers,
	}
	resp, err := rpc.CreateGroup(ctx, req)
	if err != nil {
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if resp.GroupId == 0 {
		response.Error(c, "操作失败", "请重新选择拉群人")
		return
	}
	response.Success(c, resp)
}

type inviteParam struct {
	GroupId int64   `json:"group_id"`
	UserIds []int64 `json:"user_ids"`
}

func InviteMembers(ctx context.Context, c *app.RequestContext) {
	var param inviteParam
	if err := c.BindJSON(&param); err != nil {
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	inviterId := userInfo.Id
	req := &group.InviteMembersReq{
		InviterId: inviterId,
		GroupId:   param.GroupId,
		UserIds:   param.UserIds,
	}
	resp, err := rpc.InviteMembers(ctx, req)
	if err != nil {
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "操作失败", "权限不足或成员已在群聊")
		return
	}
	response.Success(c, "邀请成功")
}

type kickParam struct {
	GroupId int64   `json:"group_id"`
	UserIds []int64 `json:"user_ids"`
}

func KickMembers(ctx context.Context, c *app.RequestContext) {
	var param kickParam
	if err := c.BindJSON(&param); err != nil {
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	operatorId := userInfo.Id
	req := &group.KickMembersReq{
		OperatorId: operatorId,
		GroupId:    param.GroupId,
		UserIds:    param.UserIds,
	}
	resp, err := rpc.KickMembers(ctx, req)
	if err != nil {
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "操作失败", "权限不足或操作不合法")
		return
	}
	response.Success(c, "踢出成功")
}

type getInfoParam struct {
	GroupId int64 `query:"group_id" json:"group_id"`
}

func GetGroupInfo(ctx context.Context, c *app.RequestContext) {
	var param getInfoParam
	if err := c.Bind(&param); err != nil || param.GroupId == 0 {
		response.Error(c, "参数缺失或错误", "无效的群组ID")
		return
	}
	req := &group.GetGroupInfoReq{
		GroupId: param.GroupId,
	}
	resp, err := rpc.GetGroupInfo(ctx, req)
	if err != nil {
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	response.Success(c, resp)
}

type changeOwnerParam struct {
	GroupId int64 `json:"group_id"`
	NewId   int64 `json:"new_id"`
}

func ChangeOwner(ctx context.Context, c *app.RequestContext) {
	var param changeOwnerParam
	if err := c.BindJSON(&param); err != nil {
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	oldId := userInfo.Id
	req := &group.ChangeOwnerReq{
		OldId:   oldId,
		GroupId: param.GroupId,
		NewId_:  param.NewId,
	}
	resp, err := rpc.ChangeOwner(ctx, req)
	if err != nil {
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "操作失败", "权限不足或新群主不在群聊内")
		return
	}
	response.Success(c, "转让成功")
}

type changeNoticeParam struct {
	GroupId int64  `json:"group_id"`
	Notice  string `json:"notice"`
}

func ChangeNotice(ctx context.Context, c *app.RequestContext) {
	var param changeNoticeParam
	if err := c.BindJSON(&param); err != nil {
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	operatorId := userInfo.Id
	req := &group.ChangeNoticeReq{
		OperatorId: operatorId,
		GroupId:    param.GroupId,
		Notice:     param.Notice,
	}
	resp, err := rpc.ChangeNotice(ctx, req)
	if err != nil {
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "操作失败", "权限不足")
		return
	}
	response.Success(c, "群公告修改成功")
}

type mutedParam struct {
	GroupId int64 `json:"group_id"`
	MutedId int64 `json:"muted_id"`
	IsMuted bool  `json:"is_muted"`
}

func Muted(ctx context.Context, c *app.RequestContext) {
	var param mutedParam
	if err := c.BindJSON(&param); err != nil {
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	operatorId := userInfo.Id
	req := &group.MutedReq{
		OperatorId: operatorId,
		GroupId:    param.GroupId,
		MutedId:    param.MutedId,
		IsMuted:    param.IsMuted,
	}
	resp, err := rpc.Muted(ctx, req)
	if err != nil {
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "操作失败", "权限不足或操作不合法")
		return
	}
	response.Success(c, "禁言状态修改成功")
}

type setAdminParam struct {
	GroupId  int64 `json:"group_id"`
	TargetId int64 `json:"target_id"`
	Role     int64 `json:"role"`
}

func SetAdmin(ctx context.Context, c *app.RequestContext) {
	var param setAdminParam
	if err := c.BindJSON(&param); err != nil {
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	operatorId := userInfo.Id
	req := &group.SetAdminReq{
		OperatorId: operatorId,
		GroupId:    param.GroupId,
		TargetId:   param.TargetId,
		Role:       param.Role,
	}
	resp, err := rpc.SetAdmin(ctx, req)
	if err != nil {
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "操作失败", "权限不足或操作不合法")
		return
	}
	response.Success(c, "管理员设置修改成功")
}
