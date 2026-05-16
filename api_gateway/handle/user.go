package handle

import (
	"api_gateway/middleware"
	"api_gateway/response"
	"api_gateway/rpc"
	"context"
	"user_service/kitex_gen/user"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type registerParam struct {
	Account  string `json:"account"`
	Name     string `json:"username"`
	Password string `json:"password"`
}

func Register(ctx context.Context, c *app.RequestContext) {
	var param registerParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "注册参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	res, err := rpc.Register(ctx, &user.RegisterReq{
		Account:  param.Account,
		Name:     param.Name,
		Password: param.Password,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "注册RPC调用失败, account=%s, err=%v", param.Account, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if res.IsExit {
		hlog.CtxWarnf(ctx, "注册重复, account=%s", param.Account)
		response.Error(c, "操作失败", "用户已存在或完善你的信息")
		return
	}
	hlog.CtxInfof(ctx, "注册成功, account=%s, name=%s", param.Account, param.Name)
	response.Success(c, "注册成功")
}

type addFriendParam struct {
	Receiver int64  `json:"receiver"`
	Message  string `json:"message"`
}

func AddFriend(ctx context.Context, c *app.RequestContext) {
	var param addFriendParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "添加好友参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id
	res, err := rpc.AddFriend(ctx, &user.AddFriendReq{
		UserId:   userID,
		Receiver: param.Receiver,
		Message:  param.Message,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "添加好友RPC调用失败, user_id=%d, receiver=%d, err=%v", userID, param.Receiver, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		hlog.CtxWarnf(ctx, "添加好友失败, user_id=%d, receiver=%d, reason=用户不存在或已是好友或已发送请求", userID, param.Receiver)
		response.Error(c, "操作失败", "用户不存在或已是好友或已发送请求")
		return
	}
	hlog.CtxInfof(ctx, "添加好友请求发送成功, user_id=%d, receiver=%d", userID, param.Receiver)
	response.Success(c, "好友请求已发送")
}

type handleFriendReqParam struct {
	Sender int64 `json:"sender"`
	Accept bool  `json:"accept"`
}

func HandleFriendReq(ctx context.Context, c *app.RequestContext) {
	var param handleFriendReqParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "处理好友请求参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id
	res, err := rpc.HandleFriendReq(ctx, &user.HandleFriendReqReq{
		Sender: param.Sender,
		UserId: userID,
		Accept: param.Accept,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "处理好友请求RPC调用失败, user_id=%d, sender=%d, err=%v", userID, param.Sender, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		hlog.CtxWarnf(ctx, "处理好友请求失败, user_id=%d, sender=%d, reason=请求不存在或已处理", userID, param.Sender)
		response.Error(c, "操作失败", "请求不存在或已处理")
		return
	}
	action := "已拒绝"
	if param.Accept {
		action = "已通过"
	}
	hlog.CtxInfof(ctx, "处理好友请求成功, user_id=%d, sender=%d, action=%s", userID, param.Sender, action)
	response.Success(c, "好友请求"+action)
}

type deleteFriendParam struct {
	FriendID int64 `json:"friend_id"`
}

func DeleteFriend(ctx context.Context, c *app.RequestContext) {
	var param deleteFriendParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "删除好友参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id
	res, err := rpc.DeleteFriend(ctx, &user.DeleteFriendReq{
		UserId:   userID,
		FriendId: param.FriendID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "删除好友RPC调用失败, user_id=%d, friend_id=%d, err=%v", userID, param.FriendID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		hlog.CtxWarnf(ctx, "删除好友失败, user_id=%d, friend_id=%d, reason=好友关系不存在", userID, param.FriendID)
		response.Error(c, "操作失败", "好友关系不存在")
		return
	}
	hlog.CtxInfof(ctx, "删除好友成功, user_id=%d, friend_id=%d", userID, param.FriendID)
	response.Success(c, "删除成功")
}

func GetFriendList(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id
	res, err := rpc.GetFriendList(ctx, &user.GetFriendListReq{
		UserId: userID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "获取好友列表RPC调用失败, user_id=%d, err=%v", userID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	hlog.CtxInfof(ctx, "获取好友列表成功, user_id=%d, count=%d", userID, len(res.Friends))
	response.Success(c, res.Friends)
}

func GetFriendRequests(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id
	res, err := rpc.GetFriendRequests(ctx, &user.GetFriendRequestsReq{
		UserId: userID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "获取好友请求列表RPC调用失败, user_id=%d, err=%v", userID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	hlog.CtxInfof(ctx, "获取好友请求列表成功, user_id=%d, count=%d", userID, len(res.Requests))
	response.Success(c, res.Requests)
}

type createFriendGroupParam struct {
	Name string `json:"name"`
}

func CreateFriendGroup(ctx context.Context, c *app.RequestContext) {
	var param createFriendGroupParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "创建好友分组参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	if param.Name == "" {
		hlog.CtxWarnf(ctx, "创建好友分组名称为空, client_ip=%s", c.ClientIP())
		response.Error(c, "参数缺失", "分组名称不能为空")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id
	res, err := rpc.CreateFriendGroup(ctx, &user.CreateFriendGroupReq{
		UserId: userID,
		Name:   param.Name,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "创建好友分组RPC调用失败, user_id=%d, name=%s, err=%v", userID, param.Name, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if res.GroupId == 0 {
		hlog.CtxWarnf(ctx, "创建好友分组失败, user_id=%d, name=%s", userID, param.Name)
		response.Error(c, "操作失败", "创建分组失败")
		return
	}
	hlog.CtxInfof(ctx, "创建好友分组成功, user_id=%d, group_id=%d, name=%s", userID, res.GroupId, param.Name)
	response.Success(c, res)
}

type updateFriendGroupParam struct {
	GroupID int64  `json:"group_id"`
	Name    string `json:"name"`
}

func UpdateFriendGroup(ctx context.Context, c *app.RequestContext) {
	var param updateFriendGroupParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "修改好友分组参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	if param.Name == "" {
		hlog.CtxWarnf(ctx, "修改好友分组名称为空, client_ip=%s", c.ClientIP())
		response.Error(c, "参数缺失", "分组名称不能为空")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id
	res, err := rpc.UpdateFriendGroup(ctx, &user.UpdateFriendGroupReq{
		GroupId: param.GroupID,
		UserId:  userID,
		Name:    param.Name,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "修改好友分组RPC调用失败, user_id=%d, group_id=%d, err=%v", userID, param.GroupID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		hlog.CtxWarnf(ctx, "修改好友分组失败, user_id=%d, group_id=%d, reason=分组不存在或无权限", userID, param.GroupID)
		response.Error(c, "操作失败", "分组不存在或无权限")
		return
	}
	hlog.CtxInfof(ctx, "修改好友分组成功, user_id=%d, group_id=%d, name=%s", userID, param.GroupID, param.Name)
	response.Success(c, "修改成功")
}

type deleteFriendGroupParam struct {
	GroupID int64 `json:"group_id"`
}

func DeleteFriendGroup(ctx context.Context, c *app.RequestContext) {
	var param deleteFriendGroupParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "删除好友分组参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id
	res, err := rpc.DeleteFriendGroup(ctx, &user.DeleteFriendGroupReq{
		GroupId: param.GroupID,
		UserId:  userID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "删除好友分组RPC调用失败, user_id=%d, group_id=%d, err=%v", userID, param.GroupID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		hlog.CtxWarnf(ctx, "删除好友分组失败, user_id=%d, group_id=%d, reason=分组不存在或无权限", userID, param.GroupID)
		response.Error(c, "操作失败", "分组不存在或无权限")
		return
	}
	hlog.CtxInfof(ctx, "删除好友分组成功, user_id=%d, group_id=%d", userID, param.GroupID)
	response.Success(c, "删除成功")
}

type moveFriendToGroupParam struct {
	FriendID int64 `json:"friend_id"`
	GroupID  int64 `json:"group_id"`
}

func MoveFriendToGroup(ctx context.Context, c *app.RequestContext) {
	var param moveFriendToGroupParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "移动好友到分组参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id
	res, err := rpc.MoveFriendToGroup(ctx, &user.MoveFriendToGroupReq{
		UserId:   userID,
		FriendId: param.FriendID,
		GroupId:  param.GroupID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "移动好友到分组RPC调用失败, user_id=%d, friend_id=%d, group_id=%d, err=%v", userID, param.FriendID, param.GroupID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		hlog.CtxWarnf(ctx, "移动好友到分组失败, user_id=%d, friend_id=%d, group_id=%d, reason=好友关系或分组不存在", userID, param.FriendID, param.GroupID)
		response.Error(c, "操作失败", "好友关系或分组不存在")
		return
	}
	hlog.CtxInfof(ctx, "移动好友到分组成功, user_id=%d, friend_id=%d, group_id=%d", userID, param.FriendID, param.GroupID)
	response.Success(c, "移动成功")
}

type updateFriendRemarkParam struct {
	FriendID int64  `json:"friend_id"`
	Remark   string `json:"remark"`
}

func UpdateFriendRemark(ctx context.Context, c *app.RequestContext) {
	var param updateFriendRemarkParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "修改好友备注参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id
	res, err := rpc.UpdateFriendRemark(ctx, &user.UpdateFriendRemarkReq{
		UserId:   userID,
		FriendId: param.FriendID,
		Remark:   param.Remark,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "修改好友备注RPC调用失败, user_id=%d, friend_id=%d, err=%v", userID, param.FriendID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		hlog.CtxWarnf(ctx, "修改好友备注失败, user_id=%d, friend_id=%d, reason=好友关系不存在", userID, param.FriendID)
		response.Error(c, "操作失败", "好友关系不存在")
		return
	}
	hlog.CtxInfof(ctx, "修改好友备注成功, user_id=%d, friend_id=%d", userID, param.FriendID)
	response.Success(c, "修改成功")
}

func GetFriendGroups(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id
	res, err := rpc.GetFriendGroups(ctx, &user.GetFriendGroupsReq{
		UserId: userID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "获取好友分组列表RPC调用失败, user_id=%d, err=%v", userID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	hlog.CtxInfof(ctx, "获取好友分组列表成功, user_id=%d, count=%d", userID, len(res.Groups))
	response.Success(c, res.Groups)
}

type searchUserByAccountParam struct {
	Account string `query:"account" json:"account"`
}

func SearchUserByAccount(ctx context.Context, c *app.RequestContext) {
	var param searchUserByAccountParam
	if err := c.Bind(&param); err != nil || param.Account == "" {
		hlog.CtxErrorf(ctx, "搜索用户参数错误, err=%v, account=%s, client_ip=%s", err, param.Account, c.ClientIP())
		response.Error(c, "参数缺失或错误", "请输入账号")
		return
	}
	res, err := rpc.SearchUserByAccount(ctx, &user.SearchUserByAccountReq{
		Account: param.Account,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "搜索用户RPC调用失败, account=%s, err=%v", param.Account, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if res.UserInfo == nil {
		response.Error(c, "用户不存在", "请检查账号")
		return
	}
	hlog.CtxInfof(ctx, "搜索用户成功, account=%s, user_id=%d", param.Account, res.UserInfo.Id)
	response.Success(c, res.UserInfo)
}
