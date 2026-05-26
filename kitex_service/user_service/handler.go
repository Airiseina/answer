package main

import (
	"context"
	"github.com/Airiseina/answer/kitex_service/user_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user"

	"github.com/Airiseina/answer/pkg/meter"

	"github.com/cloudwego/kitex/pkg/klog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type LoginServiceImpl struct {
	userService   *service.UserService
	friendService *service.FriendService
}

func (s *LoginServiceImpl) Register(ctx context.Context, req *user.RegisterReq) (resp *user.RegisterRes, err error) {
	flag, err := s.userService.Register(req.Account, req.Name, req.Password)
	if err != nil {
		klog.CtxErrorf(ctx, "处理用户[%s]注册请求时发生系统错误: %v", req.Account, err)
		meter.M.UserRegisterTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
		return nil, err
	}
	if !flag {
		meter.M.UserRegisterTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "duplicate")))
		return &user.RegisterRes{
			IsExit: true,
		}, nil
	}
	meter.M.UserRegisterTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "success")))
	return &user.RegisterRes{
		IsExit: false,
	}, nil
}

func (s *LoginServiceImpl) Login(ctx context.Context, req *user.LoginReq) (resp *user.LoginRes, err error) {
	userInfo, err := s.userService.Login(req.Account, req.Password)
	if err != nil {
		klog.CtxErrorf(ctx, "处理用户[%s]登录请求时发生系统错误: %v", req.Account, err)
		meter.M.UserLoginTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
		return nil, err
	}
	if userInfo.Account != req.Account {
		meter.M.UserLoginTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "failed")))
		return &user.LoginRes{}, nil
	}
	meter.M.UserLoginTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "success")))
	return &user.LoginRes{Account: userInfo.Account, Id: userInfo.Id, AvatarUrl: userInfo.AvatarURL}, nil
}

func (s *LoginServiceImpl) CheckUsersExist(ctx context.Context, req *user.CheckUsersExistReq) (resp *user.CheckUsersExistRes, err error) {
	flag, err := s.userService.CheckUsersExist(req.UserIds)
	if err != nil {
		klog.CtxErrorf(ctx, "处理用户创建群聊请求时发生系统错误: %v", err)
		return nil, err
	}
	if !flag {
		resp = &user.CheckUsersExistRes{
			AllExist: false,
		}
		return resp, nil
	}
	resp = &user.CheckUsersExistRes{
		AllExist: true,
	}
	return resp, nil
}

func (s *LoginServiceImpl) AddFriend(ctx context.Context, req *user.AddFriendReq) (resp *user.CommonRes, err error) {
	success, err := s.friendService.AddFriend(req.UserId, req.Receiver, req.Message)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]添加好友[%d]时发生系统错误: %v", req.UserId, req.Receiver, err)
		meter.M.FriendOpTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "add"), attribute.String("status", "error")))
		return nil, err
	}
	if !success {
		meter.M.FriendOpTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "add"), attribute.String("status", "rejected")))
	} else {
		meter.M.FriendOpTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "add"), attribute.String("status", "success")))
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) HandleFriendReq(ctx context.Context, req *user.HandleFriendReqReq) (resp *user.CommonRes, err error) {
	success, err := s.friendService.HandleFriendReq(req.Sender, req.UserId, req.Accept)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]处理好友请求[sender=%d]时发生系统错误: %v", req.UserId, req.Sender, err)
		meter.M.FriendOpTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "handle"), attribute.String("status", "error")))
		return nil, err
	}
	result := "rejected"
	if req.Accept {
		result = "accepted"
	}
	if !success {
		result = "invalid"
	}
	meter.M.FriendOpTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "handle"), attribute.String("status", result)))
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) DeleteFriend(ctx context.Context, req *user.DeleteFriendReq) (resp *user.CommonRes, err error) {
	success, err := s.friendService.DeleteFriend(req.UserId, req.FriendId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]删除好友[%d]时发生系统错误: %v", req.UserId, req.FriendId, err)
		meter.M.FriendOpTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "delete"), attribute.String("status", "error")))
		return nil, err
	}
	if success {
		meter.M.FriendOpTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "delete"), attribute.String("status", "success")))
	} else {
		meter.M.FriendOpTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", "delete"), attribute.String("status", "not_found")))
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) GetFriendList(ctx context.Context, req *user.GetFriendListReq) (resp *user.GetFriendListRes, err error) {
	friends, err := s.friendService.GetFriendList(req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]获取好友列表时发生系统错误: %v", req.UserId, err)
		return nil, err
	}
	var friendList []*user.FriendInfo
	for _, f := range friends {
		friendList = append(friendList, &user.FriendInfo{
			FriendId: f.FriendID,
			Remark:   f.Remark,
			GroupId:  f.GroupID,
			Name:     f.Name,
		})
	}
	return &user.GetFriendListRes{Friends: friendList}, nil
}

func (s *LoginServiceImpl) GetFriendRequests(ctx context.Context, req *user.GetFriendRequestsReq) (resp *user.GetFriendRequestsRes, err error) {
	requests, err := s.friendService.GetFriendRequests(req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]获取好友请求列表时发生系统错误: %v", req.UserId, err)
		return nil, err
	}
	var reqList []*user.FriendRequestInfo
	for _, r := range requests {
		reqList = append(reqList, &user.FriendRequestInfo{
			Sender:   r.Sender,
			Receiver: r.Receiver,
			Message:  r.Message,
			Status:   r.Status,
		})
	}
	return &user.GetFriendRequestsRes{Requests: reqList}, nil
}

func (s *LoginServiceImpl) CreateFriendGroup(ctx context.Context, req *user.CreateFriendGroupReq) (resp *user.CreateFriendGroupRes, err error) {
	groupID, err := s.friendService.CreateFriendGroup(req.UserId, req.Name)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]创建好友分组时发生系统错误: %v", req.UserId, err)
		return nil, err
	}
	return &user.CreateFriendGroupRes{GroupId: groupID}, nil
}

func (s *LoginServiceImpl) UpdateFriendGroup(ctx context.Context, req *user.UpdateFriendGroupReq) (resp *user.CommonRes, err error) {
	success, err := s.friendService.UpdateFriendGroup(req.GroupId, req.UserId, req.Name)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]更新好友分组[%d]时发生系统错误: %v", req.UserId, req.GroupId, err)
		return nil, err
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) DeleteFriendGroup(ctx context.Context, req *user.DeleteFriendGroupReq) (resp *user.CommonRes, err error) {
	success, err := s.friendService.DeleteFriendGroup(req.GroupId, req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]删除好友分组[%d]时发生系统错误: %v", req.UserId, req.GroupId, err)
		return nil, err
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) MoveFriendToGroup(ctx context.Context, req *user.MoveFriendToGroupReq) (resp *user.CommonRes, err error) {
	success, err := s.friendService.MoveFriendToGroup(req.UserId, req.FriendId, req.GroupId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]移动好友[%d]到分组[%d]时发生系统错误: %v", req.UserId, req.FriendId, req.GroupId, err)
		return nil, err
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) UpdateFriendRemark(ctx context.Context, req *user.UpdateFriendRemarkReq) (resp *user.CommonRes, err error) {
	success, err := s.friendService.UpdateFriendRemark(req.UserId, req.FriendId, req.Remark)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]修改好友[%d]备注时发生系统错误: %v", req.UserId, req.FriendId, err)
		return nil, err
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) GetFriendGroups(ctx context.Context, req *user.GetFriendGroupsReq) (resp *user.GetFriendGroupsRes, err error) {
	groups, err := s.friendService.GetFriendGroups(req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]获取好友分组列表时发生系统错误: %v", req.UserId, err)
		return nil, err
	}
	var groupList []*user.FriendGroupInfo
	for _, g := range groups {
		groupList = append(groupList, &user.FriendGroupInfo{
			GroupId: g.GroupID,
			Name:    g.Name,
		})
	}
	return &user.GetFriendGroupsRes{Groups: groupList}, nil
}

func (s *LoginServiceImpl) SearchUserByAccount(ctx context.Context, req *user.SearchUserByAccountReq) (resp *user.SearchUserByAccountRes, err error) {
	searchUserDTO, err := s.userService.SearchByAccount(req.Account)
	if err != nil {
		klog.CtxErrorf(ctx, "搜索账号[%s]时发生系统错误: %v", req.Account, err)
		return nil, err
	}
	if searchUserDTO.Id == 0 {
		return &user.SearchUserByAccountRes{}, nil
	}
	return &user.SearchUserByAccountRes{
		UserInfo: &user.SearchUserResult_{
			Id:        searchUserDTO.Id,
			Account:   searchUserDTO.Account,
			Name:      searchUserDTO.Name,
			AvatarUrl: searchUserDTO.AvatarURL,
		},
	}, nil
}

func (s *LoginServiceImpl) GetUserNames(ctx context.Context, req *user.GetUserNamesReq) (resp *user.GetUserNamesRes, err error) {
	users, err := s.userService.GetUserNames(req.UserIds)
	if err != nil {
		klog.CtxErrorf(ctx, "获取用户名称时发生系统错误: %v", err)
		return nil, err
	}
	var list []*user.UserNameInfo
	for _, u := range users {
		list = append(list, &user.UserNameInfo{
			Id:        u.Id,
			Name:      u.Name,
			Account:   u.Account,
			AvatarUrl: u.AvatarURL,
		})
	}
	return &user.GetUserNamesRes{Users: list}, nil
}

// GetUserIdsByAccounts implements the LoginServiceImpl interface.
func (s *LoginServiceImpl) GetUserIdsByAccounts(ctx context.Context, req *user.GetUserIdsByAccountsReq) (resp *user.GetUserIdsByAccountsRes, err error) {
	dtos, err := s.userService.GetUserIdsByAccounts(req.Accounts)
	if err != nil {
		klog.CtxErrorf(ctx, "批量查询用户ID时发生系统错误: %v", err)
		return nil, err
	}
	var list []*user.UserAccountPair
	for _, dto := range dtos {
		list = append(list, &user.UserAccountPair{
			Id:      dto.Id,
			Account: dto.Account,
		})
	}
	return &user.GetUserIdsByAccountsRes{Users: list}, nil
}

func (s *LoginServiceImpl) GetUsersInfoByAccounts(ctx context.Context, req *user.GetUsersInfoByAccountsReq) (resp *user.GetUsersInfoByAccountsRes, err error) {
	dtos, err := s.userService.GetUsersInfoByAccounts(req.Accounts)
	if err != nil {
		klog.CtxErrorf(ctx, "批量查询用户信息时发生系统错误: %v", err)
		return nil, err
	}
	var list []*user.UserInfoItem
	for _, dto := range dtos {
		list = append(list, &user.UserInfoItem{
			Account:   dto.Account,
			Name:      dto.Name,
			AvatarUrl: dto.AvatarURL,
		})
	}
	return &user.GetUsersInfoByAccountsRes{Users: list}, nil
}

func (s *LoginServiceImpl) UpdateAvatar(ctx context.Context, req *user.UpdateAvatarReq) (resp *user.CommonRes, err error) {
	success, err := s.userService.UpdateAvatar(req.UserId, req.AvatarUrl)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]更新头像时发生系统错误: %v", req.UserId, err)
		return nil, err
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) CreateBotUser(ctx context.Context, req *user.CreateBotUserReq) (resp *user.CreateBotUserRes, err error) {
	userID, err := s.userService.CreateBotUser(req.Name, req.AvatarUrl)
	if err != nil {
		klog.CtxErrorf(ctx, "创建Bot用户时发生系统错误: %v", err)
		return &user.CreateBotUserRes{Success: false}, err
	}
	return &user.CreateBotUserRes{Success: true, UserId: userID}, nil
}
