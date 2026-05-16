package handle

import (
	"api_gateway/middleware"
	"api_gateway/response"
	"api_gateway/rpc"
	"context"
	"group_service/kitex_gen/group"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type createParam struct {
	Name           string  `json:"name"`
	InitialMembers []int64 `json:"initial_members"`
}

func CreateGroup(ctx context.Context, c *app.RequestContext) {
	var param createParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "创建群组参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
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
		hlog.CtxErrorf(ctx, "创建群组RPC调用失败, creator_id=%d, group_name=%s, err=%v", id, param.Name, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if resp.GroupId == 0 {
		hlog.CtxWarnf(ctx, "创建群组失败, creator_id=%d, group_name=%s, reason=初始成员无效", id, param.Name)
		response.Error(c, "操作失败", "请重新选择拉群人")
		return
	}
	hlog.CtxInfof(ctx, "创建群组成功, creator_id=%d, group_id=%d, group_name=%s, member_count=%d", id, resp.GroupId, param.Name, len(param.InitialMembers))
	response.Success(c, map[string]interface{}{
		"group_id":     resp.GroupId,
		"group_number": strconv.FormatInt(resp.GroupNumber, 10),
	})
}

type inviteParam struct {
	GroupId int64   `json:"group_id"`
	UserIds []int64 `json:"user_ids"`
}

func InviteMembers(ctx context.Context, c *app.RequestContext) {
	var param inviteParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "邀请成员参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
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
		hlog.CtxErrorf(ctx, "邀请成员RPC调用失败, inviter_id=%d, group_id=%d, err=%v", inviterId, param.GroupId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "邀请成员失败, inviter_id=%d, group_id=%d, reason=权限不足或成员已在群聊", inviterId, param.GroupId)
		response.Error(c, "操作失败", "权限不足或成员已在群聊")
		return
	}
	hlog.CtxInfof(ctx, "邀请成员成功, inviter_id=%d, group_id=%d, invite_count=%d", inviterId, param.GroupId, len(param.UserIds))
	response.Success(c, "邀请成功")
}

type kickParam struct {
	GroupId int64   `json:"group_id"`
	UserIds []int64 `json:"user_ids"`
}

func KickMembers(ctx context.Context, c *app.RequestContext) {
	var param kickParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "踢出成员参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
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
		hlog.CtxErrorf(ctx, "踢出成员RPC调用失败, operator_id=%d, group_id=%d, err=%v", operatorId, param.GroupId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "踢出成员失败, operator_id=%d, group_id=%d, reason=权限不足或操作不合法", operatorId, param.GroupId)
		response.Error(c, "操作失败", "权限不足或操作不合法")
		return
	}
	hlog.CtxInfof(ctx, "踢出成员成功, operator_id=%d, group_id=%d, kick_count=%d", operatorId, param.GroupId, len(param.UserIds))
	response.Success(c, "踢出成功")
}

type getInfoParam struct {
	GroupId int64 `query:"group_id" json:"group_id"`
}

func GetGroupInfo(ctx context.Context, c *app.RequestContext) {
	var param getInfoParam
	if err := c.Bind(&param); err != nil || param.GroupId == 0 {
		hlog.CtxErrorf(ctx, "获取群组信息参数错误, err=%v, group_id=%d, client_ip=%s", err, param.GroupId, c.ClientIP())
		response.Error(c, "参数缺失或错误", "无效的群组ID")
		return
	}
	req := &group.GetGroupInfoReq{
		GroupId: param.GroupId,
	}
	resp, err := rpc.GetGroupInfo(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "获取群组信息RPC调用失败, group_id=%d, err=%v", param.GroupId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	result := make(map[string]interface{}) //没有群主名字，也没有成员名，还没有群号
	if resp.Group.GroupId != 0 {
		result["group_id"] = resp.Group.GroupId
		result["name"] = resp.Group.Name
		result["owner_id"] = resp.Group.OwnerId
		result["owner_name"] = resp.Group.OwnerName
		result["notice"] = resp.Group.Notice
		result["create_time"] = resp.Group.CreateTime
		result["group_number"] = strconv.FormatInt(resp.Group.GroupNumber, 10)
	} else {
		hlog.CtxErrorf(ctx, "输入错误的群号，group_id=%d", param.GroupId)
		response.Error(c, "操作失败", "请进行合法操作")
		return
	}
	var members []map[string]interface{}
	for _, m := range resp.Members {
		members = append(members, map[string]interface{}{
			"user_id":   m.UserId,
			"name":      m.Name,
			"role":      m.Role,
			"is_muted":  m.IsMuted,
			"join_time": m.JoinTime,
		})
	}
	result["members"] = members
	hlog.CtxInfof(ctx, "获取群组信息成功, group_id=%d", param.GroupId)
	response.Success(c, result)
}

type changeOwnerParam struct {
	GroupId int64 `json:"group_id"`
	NewId   int64 `json:"new_id"`
}

func ChangeOwner(ctx context.Context, c *app.RequestContext) {
	var param changeOwnerParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "转让群主参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
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
		hlog.CtxErrorf(ctx, "转让群主RPC调用失败, old_id=%d, group_id=%d, new_id=%d, err=%v", oldId, param.GroupId, param.NewId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "转让群主失败, old_id=%d, group_id=%d, new_id=%d, reason=权限不足或新群主不在群聊内", oldId, param.GroupId, param.NewId)
		response.Error(c, "操作失败", "权限不足或新群主不在群聊内")
		return
	}
	hlog.CtxInfof(ctx, "转让群主成功, old_id=%d, group_id=%d, new_id=%d", oldId, param.GroupId, param.NewId)
	response.Success(c, "转让成功")
}

type changeNoticeParam struct {
	GroupId int64  `json:"group_id"`
	Notice  string `json:"notice"`
}

func ChangeNotice(ctx context.Context, c *app.RequestContext) {
	var param changeNoticeParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "修改群公告参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
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
		hlog.CtxErrorf(ctx, "修改群公告RPC调用失败, operator_id=%d, group_id=%d, err=%v", operatorId, param.GroupId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "修改群公告失败, operator_id=%d, group_id=%d, reason=权限不足", operatorId, param.GroupId)
		response.Error(c, "操作失败", "权限不足")
		return
	}
	hlog.CtxInfof(ctx, "修改群公告成功, operator_id=%d, group_id=%d", operatorId, param.GroupId)
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
		hlog.CtxErrorf(ctx, "禁言操作参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
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
		hlog.CtxErrorf(ctx, "禁言操作RPC调用失败, operator_id=%d, group_id=%d, muted_id=%d, err=%v", operatorId, param.GroupId, param.MutedId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "禁言操作失败, operator_id=%d, group_id=%d, muted_id=%d, is_muted=%v, reason=权限不足或操作不合法", operatorId, param.GroupId, param.MutedId, param.IsMuted)
		response.Error(c, "操作失败", "权限不足或操作不合法")
		return
	}
	hlog.CtxInfof(ctx, "禁言操作成功, operator_id=%d, group_id=%d, muted_id=%d, is_muted=%v", operatorId, param.GroupId, param.MutedId, param.IsMuted)
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
		hlog.CtxErrorf(ctx, "设置管理员参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
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
		hlog.CtxErrorf(ctx, "设置管理员RPC调用失败, operator_id=%d, group_id=%d, target_id=%d, err=%v", operatorId, param.GroupId, param.TargetId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "设置管理员失败, operator_id=%d, group_id=%d, target_id=%d, role=%d, reason=权限不足或操作不合法", operatorId, param.GroupId, param.TargetId, param.Role)
		response.Error(c, "操作失败", "权限不足或操作不合法")
		return
	}
	hlog.CtxInfof(ctx, "设置管理员成功, operator_id=%d, group_id=%d, target_id=%d, role=%d", operatorId, param.GroupId, param.TargetId, param.Role)
	response.Success(c, "管理员设置修改成功")
}

func GetUserGroups(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userId := userInfo.Id
	req := &group.GetUserGroupsReq{
		UserId: userId,
	}
	resp, err := rpc.GetUserGroups(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "获取用户群聊列表RPC调用失败, user_id=%d, err=%v", userId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	hlog.CtxInfof(ctx, "获取用户群聊列表成功, user_id=%d, count=%d", userId, len(resp.Groups))
	var list []map[string]interface{}
	for _, g := range resp.Groups {
		list = append(list, map[string]interface{}{
			"group_id":     g.GroupId,
			"name":         g.Name,
			"group_number": strconv.FormatInt(g.GroupNumber, 10),
		})
	}
	response.Success(c, list)
}

type searchGroupParam struct {
	GroupNumber string `query:"group_number" json:"group_number"`
}

func SearchGroupByNumber(ctx context.Context, c *app.RequestContext) {
	var param searchGroupParam
	if err := c.Bind(&param); err != nil || param.GroupNumber == "" {
		hlog.CtxErrorf(ctx, "搜索群号参数错误, err=%v, group_number=%s, client_ip=%s", err, param.GroupNumber, c.ClientIP())
		response.Error(c, "参数缺失或错误", "无效的群号")
		return
	}
	groupNumber, err := strconv.ParseInt(param.GroupNumber, 10, 64)
	if err != nil {
		hlog.CtxErrorf(ctx, "搜索群号格式错误, group_number=%s, err=%v", param.GroupNumber, err)
		response.Error(c, "参数格式错误", "群号必须为数字")
		return
	}
	req := &group.SearchGroupByNumberReq{
		GroupNumber: groupNumber,
	}
	resp, err := rpc.SearchGroupByNumber(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "搜索群号RPC调用失败, group_number=%s, err=%v", param.GroupNumber, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if resp.GroupInfo.GroupId == 0 {
		response.Error(c, "群号不存在", "请检查群号")
		return
	}
	hlog.CtxInfof(ctx, "搜索群号成功, group_number=%s, group_id=%d", param.GroupNumber, resp.GroupInfo.GroupId)
	response.Success(c, map[string]interface{}{
		"group_id":     resp.GroupInfo.GroupId,
		"name":         resp.GroupInfo.Name,
		"owner_name":   resp.GroupInfo.OwnerName,
		"group_number": strconv.FormatInt(resp.GroupInfo.GroupNumber, 10),
	})
}

type joinGroupParam struct {
	GroupNumber string `json:"group_number"`
	Message     string `json:"message"`
}

func JoinGroup(ctx context.Context, c *app.RequestContext) {
	var param joinGroupParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "申请入群参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	groupNumber, err := strconv.ParseInt(param.GroupNumber, 10, 64)
	if err != nil {
		hlog.CtxErrorf(ctx, "申请入群群号格式错误, group_number=%s, err=%v", param.GroupNumber, err)
		response.Error(c, "参数格式错误", "群号必须为数字")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userId := userInfo.Id
	req := &group.JoinGroupReq{
		UserId:      userId,
		GroupNumber: groupNumber,
		Message:     param.Message,
	}
	resp, err := rpc.JoinGroup(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "申请入群RPC调用失败, user_id=%d, group_number=%d, err=%v", userId, param.GroupNumber, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "申请入群失败, user_id=%d, group_number=%d", userId, param.GroupNumber)
		response.Error(c, "操作失败", "已存在待处理的申请或已是群成员")
		return
	}
	hlog.CtxInfof(ctx, "申请入群成功, user_id=%d, group_number=%d", userId, param.GroupNumber)
	response.Success(c, "申请已发送")
}

type handleJoinReqParam struct {
	GroupId int64 `json:"group_id"`
	UserId  int64 `json:"user_id"`
	Accept  bool  `json:"accept"`
}

func HandleJoinReq(ctx context.Context, c *app.RequestContext) {
	var param handleJoinReqParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "处理入群申请参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	operatorId := userInfo.Id
	req := &group.HandleJoinReqReq{
		OperatorId: operatorId,
		GroupId:    param.GroupId,
		UserId:     param.UserId,
		Accept:     param.Accept,
	}
	resp, err := rpc.HandleJoinReq(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "处理入群申请RPC调用失败, operator_id=%d, group_id=%d, user_id=%d, err=%v", operatorId, param.GroupId, param.UserId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "处理入群申请失败, operator_id=%d, group_id=%d, user_id=%d", operatorId, param.GroupId, param.UserId)
		response.Error(c, "操作失败", "权限不足或申请已被处理")
		return
	}
	hlog.CtxInfof(ctx, "处理入群申请成功, operator_id=%d, group_id=%d, user_id=%d, accept=%v", operatorId, param.GroupId, param.UserId, param.Accept)
	response.Success(c, "处理成功")
}

type getJoinRequestsParam struct {
	GroupId int64 `query:"group_id" json:"group_id"`
}

func GetJoinRequests(ctx context.Context, c *app.RequestContext) {
	var param getJoinRequestsParam
	if err := c.Bind(&param); err != nil || param.GroupId == 0 {
		hlog.CtxErrorf(ctx, "获取入群申请参数错误, err=%v, group_id=%d, client_ip=%s", err, param.GroupId, c.ClientIP())
		response.Error(c, "参数缺失或错误", "无效的群组ID")
		return
	}
	req := &group.GetJoinRequestsReq{
		GroupId: param.GroupId,
	}
	resp, err := rpc.GetJoinRequests(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "获取入群申请RPC调用失败, group_id=%d, err=%v", param.GroupId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	hlog.CtxInfof(ctx, "获取入群申请成功, group_id=%d, count=%d", param.GroupId, len(resp.Requests))
	response.Success(c, resp.Requests)
}
