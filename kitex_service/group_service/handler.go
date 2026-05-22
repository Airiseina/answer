package main

import (
	"context"
	"group_service/internal/service"
	"group_service/kitex_gen/group"
	"group_service/rpc"

	"github.com/cloudwego/kitex/pkg/klog"
)

type GroupServiceImpl struct {
	groupService *service.GroupService
}

func (s *GroupServiceImpl) CreateGroup(ctx context.Context, req *group.CreateGroupReq) (resp *group.CreateGroupRes, err error) {
	members := req.InitialMembers
	creatorInMembers := false
	for _, m := range members {
		if m == req.CreatorId {
			creatorInMembers = true
			break
		}
	}
	if !creatorInMembers {
		members = append([]int64{req.CreatorId}, members...)
	} else {
		newMembers := []int64{req.CreatorId}
		for _, m := range members {
			if m != req.CreatorId {
				newMembers = append(newMembers, m)
			}
		}
		members = newMembers
	}
	f, err := rpc.CheckUsersExist(ctx, members)
	if err != nil {
		return nil, err
	}
	if !f {
		return &group.CreateGroupRes{GroupId: 0}, nil
	}
	nameMap, err := rpc.GetUserNames(ctx, members)
	if err != nil {
		klog.CtxErrorf(ctx, "创建群聊时获取用户名失败:%v", err)
		nameMap = make(map[int64]string)
	}
	groupId, groupNumber, conversationID, err := s.groupService.CreateGroup(ctx, req.CreatorId, req.Name, members, nameMap)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]创建群聊时发生系统错误:%v", req.CreatorId, err)
		return nil, err
	}
	resp = &group.CreateGroupRes{
		GroupId:        groupId,
		GroupNumber:    groupNumber,
		ConversationId: conversationID,
	}
	return resp, nil
}

func (s *GroupServiceImpl) InviteMembers(ctx context.Context, req *group.InviteMembersReq) (resp *group.CommonRes, err error) {
	nameMap, rpcErr := rpc.GetUserNames(ctx, req.UserIds)
	if rpcErr != nil {
		klog.CtxErrorf(ctx, "邀请成员时获取用户名失败:%v", rpcErr)
		nameMap = make(map[int64]string)
	}
	success, err := s.groupService.InviteMembers(ctx, req.InviterId, req.GroupId, req.UserIds, nameMap)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]邀请成员到群[%d]时发生系统错误:%v", req.InviterId, req.GroupId, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

func (s *GroupServiceImpl) KickMembers(ctx context.Context, req *group.KickMembersReq) (resp *group.CommonRes, err error) {
	success, err := s.groupService.KickMembers(ctx, req.OperatorId, req.GroupId, req.UserIds)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]踢出[%d]群成员时发生系统错误:%v", req.OperatorId, req.GroupId, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

func (s *GroupServiceImpl) GetGroupInfo(ctx context.Context, req *group.GetGroupInfoReq) (resp *group.GetGroupInfoRes, err error) {
	groupInfo, err := s.groupService.GetGroupInfo(req.GroupId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询群聊[%d]时发生系统错误:%v", req.GroupId, err)
		return &group.GetGroupInfoRes{}, err
	}
	if groupInfo.GroupId == 0 {
		return &group.GetGroupInfoRes{}, nil
	}
	var members []*group.GroupMember
	for _, member := range groupInfo.Members {
		members = append(members, &group.GroupMember{
			GroupId:  req.GroupId,
			UserId:   member.UserID,
			Name:     member.Name,
			Role:     member.Role,
			IsMuted:  member.IsMute,
			JoinTime: member.JoinTime.Unix(),
		})
	}
	resp = &group.GetGroupInfoRes{
		Group: &group.Group{
			GroupId:     groupInfo.GroupId,
			Name:        groupInfo.Name,
			OwnerId:     groupInfo.OwnerID,
			OwnerName:   groupInfo.OwnerName,
			Notice:      groupInfo.Notice,
			CreateTime:  groupInfo.CreatedAt.Unix(),
			GroupNumber: groupInfo.GroupNumber,
		},
		Members: members,
	}
	return resp, nil
}

func (s *GroupServiceImpl) ChangeOwner(ctx context.Context, req *group.ChangeOwnerReq) (resp *group.CommonRes, err error) {
	success, err := s.groupService.ChangeOwner(req.OldId, req.GroupId, req.NewId_)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]修改[%d]群主给[%d]发生系统错误:%v", req.OldId, req.GroupId, req.NewId_, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

func (s *GroupServiceImpl) ChangeNotice(ctx context.Context, req *group.ChangeNoticeReq) (resp *group.CommonRes, err error) {
	success, err := s.groupService.ChangeNotice(req.OperatorId, req.GroupId, req.Notice)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]修改[%d]群公告时发生系统错误:%v", req.OperatorId, req.GroupId, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

func (s *GroupServiceImpl) Muted(ctx context.Context, req *group.MutedReq) (resp *group.CommonRes, err error) {
	success, err := s.groupService.Muted(req.OperatorId, req.GroupId, req.MutedId, req.IsMuted)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]修改[%d]群聊成员[%d]禁言:%v", req.OperatorId, req.GroupId, req.MutedId, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

func (s *GroupServiceImpl) SetAdmin(ctx context.Context, req *group.SetAdminReq) (resp *group.CommonRes, err error) {
	success, err := s.groupService.SetAdmin(req.OperatorId, req.GroupId, req.TargetId, req.Role)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]设置[%d]群聊成员[%d]管理员状态:%v", req.OperatorId, req.GroupId, req.TargetId, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

func (s *GroupServiceImpl) GetUserGroups(ctx context.Context, req *group.GetUserGroupsReq) (resp *group.GetUserGroupsRes, err error) {
	groups, err := s.groupService.GetUserGroups(req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]获取群聊列表时发生系统错误:%v", req.UserId, err)
		return nil, err
	}
	var list []*group.UserGroupInfo
	for _, g := range groups {
		list = append(list, &group.UserGroupInfo{
			GroupId:     g.GroupID,
			Name:        g.Name,
			GroupNumber: g.GroupNumber,
		})
	}
	return &group.GetUserGroupsRes{Groups: list}, nil
}

func (s *GroupServiceImpl) SearchGroupByNumber(ctx context.Context, req *group.SearchGroupByNumberReq) (resp *group.SearchGroupByNumberRes, err error) {
	result, err := s.groupService.SearchGroupByNumber(req.GroupNumber)
	if err != nil {
		klog.CtxErrorf(ctx, "搜索群号[%d]时发生系统错误:%v", req.GroupNumber, err)
		return nil, err
	}
	resp = &group.SearchGroupByNumberRes{
		GroupInfo: &group.GroupSearchResult_{
			GroupId:     result.GroupId,
			Name:        result.Name,
			OwnerName:   result.OwnerName,
			GroupNumber: result.GroupNumber,
		},
	}
	return resp, nil
}

func (s *GroupServiceImpl) JoinGroup(ctx context.Context, req *group.JoinGroupReq) (resp *group.CommonRes, err error) {
	success, err := s.groupService.JoinGroup(req.UserId, req.GroupNumber, req.Message)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]申请加入群[%d]时发生系统错误:%v", req.UserId, req.GroupNumber, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

func (s *GroupServiceImpl) HandleJoinReq(ctx context.Context, req *group.HandleJoinReqReq) (resp *group.CommonRes, err error) {
	var userName string
	if req.Accept {
		nameMap, rpcErr := rpc.GetUserNames(ctx, []int64{req.UserId})
		if rpcErr != nil {
			klog.CtxErrorf(ctx, "处理入群申请时获取用户名失败:%v", rpcErr)
		} else {
			userName = nameMap[req.UserId]
		}
	}
	success, err := s.groupService.HandleJoinRequest(ctx, req.OperatorId, req.GroupId, req.UserId, req.Accept, userName)
	if err != nil {
		klog.CtxErrorf(ctx, "处理入群申请时发生系统错误:%v", err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

func (s *GroupServiceImpl) GetJoinRequests(ctx context.Context, req *group.GetJoinRequestsReq) (resp *group.GetJoinRequestsRes, err error) {
	requests, err := s.groupService.GetJoinRequests(req.GroupId)
	if err != nil {
		klog.CtxErrorf(ctx, "获取群[%d]入群申请时发生系统错误:%v", req.GroupId, err)
		return nil, err
	}
	var userIds []int64
	for _, r := range requests {
		userIds = append(userIds, r.UserID)
	}
	nameMap := make(map[int64]string)
	if len(userIds) > 0 {
		nameMap, _ = rpc.GetUserNames(ctx, userIds)
	}
	var list []*group.JoinRequestInfo
	for _, r := range requests {
		list = append(list, &group.JoinRequestInfo{
			UserId:  r.UserID,
			Name:    nameMap[r.UserID],
			Message: r.Message,
			Status:  r.Status,
		})
	}
	return &group.GetJoinRequestsRes{Requests: list}, nil
}
