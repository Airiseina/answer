package handle

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"
	"github.com/Airiseina/answer/kitex_service/group_service/kitex_gen/group"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type createParam struct {
	Name            string   `json:"name"`
	InitialAccounts []string `json:"initial_accounts"`
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

	userIdMap := buildUserIdMap(ctx, param.InitialAccounts)
	initialMembers := make([]int64, 0, len(param.InitialAccounts))
	for _, acc := range param.InitialAccounts {
		if uid, ok := userIdMap[acc]; ok && uid != 0 {
			initialMembers = append(initialMembers, uid)
		}
	}

	req := &group.CreateGroupReq{
		CreatorId:      id,
		Name:           param.Name,
		InitialMembers: initialMembers,
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
	hlog.CtxInfof(ctx, "创建群组成功, creator_id=%d, group_id=%d, group_name=%s", id, resp.GroupId, param.Name)
	response.Success(c, map[string]interface{}{
		"group_number":    strconv.FormatInt(resp.GroupNumber, 10),
		"conversation_id": strconv.FormatInt(resp.ConversationId, 10),
	})
}

type inviteParam struct {
	GroupNumber string   `json:"group_number"`
	Accounts    []string `json:"accounts"`
}

func InviteMembers(ctx context.Context, c *app.RequestContext) {
	var param inviteParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "邀请成员参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	groupNumber, err := strconv.ParseInt(param.GroupNumber, 10, 64)
	if err != nil || groupNumber == 0 {
		response.Error(c, "参数错误", "group_number格式不正确")
		return
	}

	searchResp, err := rpc.SearchGroupByNumber(ctx, &group.SearchGroupByNumberReq{GroupNumber: groupNumber})
	if err != nil {
		hlog.CtxErrorf(ctx, "邀请成员-群号查找失败, group_number=%d, err=%v", groupNumber, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if searchResp.GroupInfo.GroupId == 0 {
		response.Error(c, "群不存在", "请检查群号是否正确")
		return
	}
	groupID := searchResp.GroupInfo.GroupId

	userIdMap := buildUserIdMap(ctx, param.Accounts)
	userIDs := make([]int64, 0, len(param.Accounts))
	for _, acc := range param.Accounts {
		if uid, ok := userIdMap[acc]; ok && uid != 0 {
			userIDs = append(userIDs, uid)
		}
	}

	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	inviterId := userInfo.Id
	req := &group.InviteMembersReq{
		InviterId: inviterId,
		GroupId:   groupID,
		UserIds:   userIDs,
	}
	resp, err := rpc.InviteMembers(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "邀请成员RPC调用失败, inviter_id=%d, group_id=%d, err=%v", inviterId, groupID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "邀请成员失败, inviter_id=%d, group_id=%d", inviterId, groupID)
		response.Error(c, "操作失败", "权限不足或成员已在群聊")
		return
	}
	hlog.CtxInfof(ctx, "邀请成员成功, inviter_id=%d, group_id=%d, invite_count=%d", inviterId, groupID, len(userIDs))
	response.Success(c, "邀请成功")
}

type kickParam struct {
	GroupNumber string   `json:"group_number"`
	Accounts    []string `json:"accounts"`
}

func KickMembers(ctx context.Context, c *app.RequestContext) {
	var param kickParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "踢出成员参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	groupNumber, err := strconv.ParseInt(param.GroupNumber, 10, 64)
	if err != nil || groupNumber == 0 {
		response.Error(c, "参数错误", "group_number格式不正确")
		return
	}

	searchResp, err := rpc.SearchGroupByNumber(ctx, &group.SearchGroupByNumberReq{GroupNumber: groupNumber})
	if err != nil {
		hlog.CtxErrorf(ctx, "踢出成员-群号查找失败, group_number=%d, err=%v", groupNumber, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if searchResp.GroupInfo.GroupId == 0 {
		response.Error(c, "群不存在", "请检查群号是否正确")
		return
	}
	groupID := searchResp.GroupInfo.GroupId

	userIdMap := buildUserIdMap(ctx, param.Accounts)
	userIDs := make([]int64, 0, len(param.Accounts))
	for _, acc := range param.Accounts {
		if uid, ok := userIdMap[acc]; ok && uid != 0 {
			userIDs = append(userIDs, uid)
		}
	}

	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	operatorId := userInfo.Id
	req := &group.KickMembersReq{
		OperatorId: operatorId,
		GroupId:    groupID,
		UserIds:    userIDs,
	}
	resp, err := rpc.KickMembers(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "踢出成员RPC调用失败, operator_id=%d, group_id=%d, err=%v", operatorId, groupID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "踢出成员失败, operator_id=%d, group_id=%d", operatorId, groupID)
		response.Error(c, "操作失败", "权限不足或操作不合法")
		return
	}
	hlog.CtxInfof(ctx, "踢出成员成功, operator_id=%d, group_id=%d, kick_count=%d", operatorId, groupID, len(userIDs))
	response.Success(c, "踢出成功")
}

type getInfoParam struct {
	GroupNumber string `query:"group_number" json:"group_number"`
}

func GetGroupInfo(ctx context.Context, c *app.RequestContext) {
	var param getInfoParam
	if err := c.Bind(&param); err != nil || param.GroupNumber == "" {
		hlog.CtxErrorf(ctx, "获取群组信息参数错误, err=%v, group_number=%s, client_ip=%s", err, param.GroupNumber, c.ClientIP())
		response.Error(c, "参数缺失或错误", "无效的群号")
		return
	}
	groupNumber, err := strconv.ParseInt(param.GroupNumber, 10, 64)
	if err != nil {
		response.Error(c, "参数错误", "群号格式不正确")
		return
	}

	searchResp, err := rpc.SearchGroupByNumber(ctx, &group.SearchGroupByNumberReq{GroupNumber: groupNumber})
	if err != nil {
		hlog.CtxErrorf(ctx, "获取群组信息-群号查找失败, group_number=%d, err=%v", groupNumber, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if searchResp.GroupInfo.GroupId == 0 {
		response.Error(c, "群不存在", "请检查群号是否正确")
		return
	}
	groupID := searchResp.GroupInfo.GroupId

	req := &group.GetGroupInfoReq{
		GroupId: groupID,
	}
	resp, err := rpc.GetGroupInfo(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "获取群组信息RPC调用失败, group_id=%d, err=%v", groupID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	result := make(map[string]interface{})
	if resp.Group.GroupId != 0 {
		result["name"] = resp.Group.Name
		result["create_time"] = resp.Group.CreateTime
		result["group_number"] = strconv.FormatInt(resp.Group.GroupNumber, 10)

		ownerIDs := []int64{resp.Group.OwnerId}
		ownerAccountMap := buildAccountMap(ctx, ownerIDs)
		result["owner_account"] = ownerAccountMap[resp.Group.OwnerId]
		result["owner_name"] = resp.Group.OwnerName

		type noticeItem struct {
			ID         int64  `json:"id"`
			Content    string `json:"content"`
			OperatorID int64  `json:"operator_id"`
			CreateTime int64  `json:"create_time"`
		}
		var notices []noticeItem
		if jsonErr := json.Unmarshal([]byte(resp.Group.Notice), &notices); jsonErr == nil && len(notices) > 0 {
			result["notice"] = notices[0].Content
			var noticeOperatorIDs []int64
			for _, n := range notices {
				noticeOperatorIDs = append(noticeOperatorIDs, n.OperatorID)
			}
			noticeAccountMap := buildAccountMap(ctx, noticeOperatorIDs)
			var noticeResults []map[string]interface{}
			for _, n := range notices {
				noticeResults = append(noticeResults, map[string]interface{}{
					"id":            n.ID,
					"content":       n.Content,
					"operator_id":   n.OperatorID,
					"operator_name": noticeAccountMap[n.OperatorID],
					"create_time":   n.CreateTime,
				})
			}
			result["notices"] = noticeResults
		} else {
			result["notice"] = resp.Group.Notice
			result["notices"] = []interface{}{}
		}
	} else {
		hlog.CtxErrorf(ctx, "输入错误的群号，group_id=%d", groupID)
		response.Error(c, "操作失败", "请进行合法操作")
		return
	}

	var memberUserIDs []int64
	for _, m := range resp.Members {
		memberUserIDs = append(memberUserIDs, m.UserId)
	}
	memberAccountMap := buildAccountMap(ctx, memberUserIDs)

	var members []map[string]interface{}
	for _, m := range resp.Members {
		members = append(members, map[string]interface{}{
			"account":   memberAccountMap[m.UserId],
			"name":      m.Name,
			"role":      m.Role,
			"is_muted":  m.IsMuted,
			"join_time": m.JoinTime,
		})
	}
	result["members"] = members
	hlog.CtxInfof(ctx, "获取群组信息成功, group_id=%d", groupID)
	response.Success(c, result)
}

type changeOwnerParam struct {
	GroupNumber string `json:"group_number"`
	NewAccount  string `json:"new_account"`
}

func ChangeOwner(ctx context.Context, c *app.RequestContext) {
	var param changeOwnerParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "转让群主参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	groupNumber, err := strconv.ParseInt(param.GroupNumber, 10, 64)
	if err != nil || groupNumber == 0 {
		response.Error(c, "参数错误", "group_number格式不正确")
		return
	}

	searchResp, err := rpc.SearchGroupByNumber(ctx, &group.SearchGroupByNumberReq{GroupNumber: groupNumber})
	if err != nil {
		hlog.CtxErrorf(ctx, "转让群主-群号查找失败, group_number=%d, err=%v", groupNumber, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if searchResp.GroupInfo.GroupId == 0 {
		response.Error(c, "群不存在", "请检查群号是否正确")
		return
	}
	groupID := searchResp.GroupInfo.GroupId

	newIdMap := buildUserIdMap(ctx, []string{param.NewAccount})
	newId, ok := newIdMap[param.NewAccount]
	if !ok || newId == 0 {
		response.Error(c, "参数错误", "新群主账号不存在")
		return
	}

	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	oldId := userInfo.Id
	req := &group.ChangeOwnerReq{
		OldId:   oldId,
		GroupId: groupID,
		NewId_:  newId,
	}
	resp, err := rpc.ChangeOwner(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "转让群主RPC调用失败, old_id=%d, group_id=%d, new_id=%d, err=%v", oldId, groupID, newId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "转让群主失败, old_id=%d, group_id=%d, new_id=%d", oldId, groupID, newId)
		response.Error(c, "操作失败", "权限不足或新群主不在群聊内")
		return
	}
	hlog.CtxInfof(ctx, "转让群主成功, old_id=%d, group_id=%d, new_id=%d", oldId, groupID, newId)
	response.Success(c, "转让成功")
}

type changeNoticeParam struct {
	GroupNumber string `json:"group_number"`
	Notice      string `json:"notice"`
}

func ChangeNotice(ctx context.Context, c *app.RequestContext) {
	var param changeNoticeParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "修改群公告参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	groupNumber, err := strconv.ParseInt(param.GroupNumber, 10, 64)
	if err != nil || groupNumber == 0 {
		response.Error(c, "参数错误", "group_number格式不正确")
		return
	}

	searchResp, err := rpc.SearchGroupByNumber(ctx, &group.SearchGroupByNumberReq{GroupNumber: groupNumber})
	if err != nil {
		hlog.CtxErrorf(ctx, "修改群公告-群号查找失败, group_number=%d, err=%v", groupNumber, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if searchResp.GroupInfo.GroupId == 0 {
		response.Error(c, "群不存在", "请检查群号是否正确")
		return
	}
	groupID := searchResp.GroupInfo.GroupId

	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	operatorId := userInfo.Id
	req := &group.ChangeNoticeReq{
		OperatorId: operatorId,
		GroupId:    groupID,
		Notice:     param.Notice,
	}
	resp, err := rpc.ChangeNotice(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "修改群公告RPC调用失败, operator_id=%d, group_id=%d, err=%v", operatorId, groupID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "修改群公告失败, operator_id=%d, group_id=%d", operatorId, groupID)
		response.Error(c, "操作失败", "权限不足")
		return
	}
	hlog.CtxInfof(ctx, "修改群公告成功, operator_id=%d, group_id=%d", operatorId, groupID)
	response.Success(c, "群公告修改成功")
}

type mutedParam struct {
	GroupNumber  string `json:"group_number"`
	MutedAccount string `json:"muted_account"`
	IsMuted      bool   `json:"is_muted"`
}

func Muted(ctx context.Context, c *app.RequestContext) {
	var param mutedParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "禁言操作参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	groupNumber, err := strconv.ParseInt(param.GroupNumber, 10, 64)
	if err != nil || groupNumber == 0 {
		response.Error(c, "参数错误", "group_number格式不正确")
		return
	}

	searchResp, err := rpc.SearchGroupByNumber(ctx, &group.SearchGroupByNumberReq{GroupNumber: groupNumber})
	if err != nil {
		hlog.CtxErrorf(ctx, "禁言操作-群号查找失败, group_number=%d, err=%v", groupNumber, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if searchResp.GroupInfo.GroupId == 0 {
		response.Error(c, "群不存在", "请检查群号是否正确")
		return
	}
	groupID := searchResp.GroupInfo.GroupId

	mutedIdMap := buildUserIdMap(ctx, []string{param.MutedAccount})
	mutedId, ok := mutedIdMap[param.MutedAccount]
	if !ok || mutedId == 0 {
		response.Error(c, "参数错误", "禁言账号不存在")
		return
	}

	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	operatorId := userInfo.Id
	req := &group.MutedReq{
		OperatorId: operatorId,
		GroupId:    groupID,
		MutedId:    mutedId,
		IsMuted:    param.IsMuted,
	}
	resp, err := rpc.Muted(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "禁言操作RPC调用失败, operator_id=%d, group_id=%d, muted_id=%d, err=%v", operatorId, groupID, mutedId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "禁言操作失败, operator_id=%d, group_id=%d, muted_id=%d", operatorId, groupID, mutedId)
		response.Error(c, "操作失败", "权限不足或操作不合法")
		return
	}
	hlog.CtxInfof(ctx, "禁言操作成功, operator_id=%d, group_id=%d, muted_id=%d, is_muted=%v", operatorId, groupID, mutedId, param.IsMuted)
	response.Success(c, "禁言状态修改成功")
}

type setAdminParam struct {
	GroupNumber   string `json:"group_number"`
	TargetAccount string `json:"target_account"`
	Role          int64  `json:"role"`
}

func SetAdmin(ctx context.Context, c *app.RequestContext) {
	var param setAdminParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "设置管理员参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	if param.Role != 0 && param.Role != 1 {
		response.Error(c, "参数错误", "role只能为0(普通成员)或1(管理员)")
		return
	}
	groupNumber, err := strconv.ParseInt(param.GroupNumber, 10, 64)
	if err != nil || groupNumber == 0 {
		response.Error(c, "参数错误", "group_number格式不正确")
		return
	}

	searchResp, err := rpc.SearchGroupByNumber(ctx, &group.SearchGroupByNumberReq{GroupNumber: groupNumber})
	if err != nil {
		hlog.CtxErrorf(ctx, "设置管理员-群号查找失败, group_number=%d, err=%v", groupNumber, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if searchResp.GroupInfo.GroupId == 0 {
		response.Error(c, "群不存在", "请检查群号是否正确")
		return
	}
	groupID := searchResp.GroupInfo.GroupId

	targetIdMap := buildUserIdMap(ctx, []string{param.TargetAccount})
	targetId, ok := targetIdMap[param.TargetAccount]
	if !ok || targetId == 0 {
		response.Error(c, "参数错误", "目标账号不存在")
		return
	}

	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	operatorId := userInfo.Id
	req := &group.SetAdminReq{
		OperatorId: operatorId,
		GroupId:    groupID,
		TargetId:   targetId,
		Role:       param.Role,
	}
	resp, err := rpc.SetAdmin(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "设置管理员RPC调用失败, operator_id=%d, group_id=%d, target_id=%d, err=%v", operatorId, groupID, targetId, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "设置管理员失败, operator_id=%d, group_id=%d, target_id=%d", operatorId, groupID, targetId)
		response.Error(c, "操作失败", "权限不足或操作不合法")
		return
	}
	hlog.CtxInfof(ctx, "设置管理员成功, operator_id=%d, group_id=%d, target_id=%d, role=%d", operatorId, groupID, targetId, param.Role)
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
	hlog.CtxInfof(ctx, "搜索群号成功, group_number=%s", param.GroupNumber)
	response.Success(c, map[string]interface{}{
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
	if groupNumber == 0 {
		response.Error(c, "参数错误", "群号不能为0")
		return
	}

	searchResp, searchErr := rpc.SearchGroupByNumber(ctx, &group.SearchGroupByNumberReq{GroupNumber: groupNumber})
	if searchErr != nil {
		hlog.CtxErrorf(ctx, "申请入群-群号查找失败, group_number=%d, err=%v", groupNumber, searchErr)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if searchResp.GroupInfo.GroupId == 0 {
		response.Error(c, "群不存在", "请检查群号是否正确")
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
		errMsg := err.Error()
		if strings.Contains(errMsg, "群不存在") {
			response.Error(c, "群不存在", "请检查群号是否正确")
			return
		}
		if strings.Contains(errMsg, "已是群成员") {
			response.Error(c, "操作失败", "你已经是群成员")
			return
		}
		if strings.Contains(errMsg, "已存在待处理") {
			response.Error(c, "操作失败", "已存在待处理的申请")
			return
		}
		hlog.CtxErrorf(ctx, "申请入群RPC调用失败, user_id=%d, group_number=%d, err=%v", userId, groupNumber, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "申请入群失败, user_id=%d, group_number=%d", userId, groupNumber)
		response.Error(c, "操作失败", "申请入群失败")
		return
	}
	hlog.CtxInfof(ctx, "申请入群成功, user_id=%d, group_number=%d", userId, groupNumber)
	response.Success(c, "申请已发送")
}

type handleJoinReqParam struct {
	GroupNumber string `json:"group_number"`
	Account     string `json:"account"`
	Accept      bool   `json:"accept"`
}

func HandleJoinReq(ctx context.Context, c *app.RequestContext) {
	var param handleJoinReqParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "处理入群申请参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	groupNumber, err := strconv.ParseInt(param.GroupNumber, 10, 64)
	if err != nil || groupNumber == 0 {
		response.Error(c, "参数错误", "group_number格式不正确")
		return
	}

	searchResp, err := rpc.SearchGroupByNumber(ctx, &group.SearchGroupByNumberReq{GroupNumber: groupNumber})
	if err != nil {
		hlog.CtxErrorf(ctx, "处理入群申请-群号查找失败, group_number=%d, err=%v", groupNumber, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if searchResp.GroupInfo.GroupId == 0 {
		response.Error(c, "群不存在", "请检查群号是否正确")
		return
	}
	groupID := searchResp.GroupInfo.GroupId

	userIdMap := buildUserIdMap(ctx, []string{param.Account})
	userID, ok := userIdMap[param.Account]
	if !ok || userID == 0 {
		response.Error(c, "参数错误", "账号不存在")
		return
	}

	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	operatorId := userInfo.Id
	req := &group.HandleJoinReqReq{
		OperatorId: operatorId,
		GroupId:    groupID,
		UserId:     userID,
		Accept:     param.Accept,
	}
	resp, err := rpc.HandleJoinReq(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "处理入群申请RPC调用失败, operator_id=%d, group_id=%d, user_id=%d, err=%v", operatorId, groupID, userID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		hlog.CtxWarnf(ctx, "处理入群申请失败, operator_id=%d, group_id=%d, user_id=%d", operatorId, groupID, userID)
		response.Error(c, "操作失败", "权限不足或申请已被处理")
		return
	}
	hlog.CtxInfof(ctx, "处理入群申请成功, operator_id=%d, group_id=%d, user_id=%d, accept=%v", operatorId, groupID, userID, param.Accept)
	response.Success(c, "处理成功")
}

type getJoinRequestsParam struct {
	GroupNumber string `query:"group_number" json:"group_number"`
}

func GetJoinRequests(ctx context.Context, c *app.RequestContext) {
	var param getJoinRequestsParam
	if err := c.Bind(&param); err != nil || param.GroupNumber == "" {
		hlog.CtxErrorf(ctx, "获取入群申请参数错误, err=%v, group_number=%s, client_ip=%s", err, param.GroupNumber, c.ClientIP())
		response.Error(c, "参数缺失或错误", "无效的群号")
		return
	}
	groupNumber, err := strconv.ParseInt(param.GroupNumber, 10, 64)
	if err != nil {
		response.Error(c, "参数错误", "群号格式不正确")
		return
	}

	searchResp, err := rpc.SearchGroupByNumber(ctx, &group.SearchGroupByNumberReq{GroupNumber: groupNumber})
	if err != nil {
		hlog.CtxErrorf(ctx, "获取入群申请-群号查找失败, group_number=%d, err=%v", groupNumber, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if searchResp.GroupInfo.GroupId == 0 {
		response.Error(c, "群不存在", "请检查群号是否正确")
		return
	}
	groupID := searchResp.GroupInfo.GroupId

	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	operatorId := userInfo.Id

	groupInfoResp, err := rpc.GetGroupInfo(ctx, &group.GetGroupInfoReq{GroupId: groupID})
	if err != nil {
		hlog.CtxErrorf(ctx, "获取入群申请-查询群信息失败, group_id=%d, err=%v", groupID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	isOwner := groupInfoResp.Group.OwnerId == operatorId
	isAdmin := false
	if !isOwner {
		for _, m := range groupInfoResp.Members {
			if m.UserId == operatorId && m.Role >= 1 {
				isAdmin = true
				break
			}
		}
	}
	if !isOwner && !isAdmin {
		response.Error(c, "权限不足", "仅群主或管理员可查看入群申请")
		return
	}

	req := &group.GetJoinRequestsReq{
		GroupId: groupID,
	}
	resp, err := rpc.GetJoinRequests(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "获取入群申请RPC调用失败, group_id=%d, err=%v", groupID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}

	var requestUserIDs []int64
	for _, r := range resp.Requests {
		requestUserIDs = append(requestUserIDs, r.UserId)
	}
	accountMap := buildAccountMap(ctx, requestUserIDs)

	type joinRequestItem struct {
		Account string `json:"account"`
		Name    string `json:"name"`
		Message string `json:"message"`
		Status  int64  `json:"status"`
	}
	var items []joinRequestItem
	for _, r := range resp.Requests {
		items = append(items, joinRequestItem{
			Account: accountMap[r.UserId],
			Name:    r.Name,
			Message: r.Message,
			Status:  r.Status,
		})
	}
	if items == nil {
		items = []joinRequestItem{}
	}

	hlog.CtxInfof(ctx, "获取入群申请成功, group_id=%d, count=%d", groupID, len(resp.Requests))
	response.Success(c, items)
}
