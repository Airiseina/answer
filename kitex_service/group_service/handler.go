package main

import (
	"context"
	"group_service/internal/service"
	"group_service/kitex_gen/group"
	"group_service/rpc"

	"github.com/cloudwego/kitex/pkg/klog"
)

// GroupServiceImpl implements the last service interface defined in the IDL.
type GroupServiceImpl struct {
	groupService *service.GroupService
}

// CreateGroup implements the GroupServiceImpl interface.
func (s *GroupServiceImpl) CreateGroup(ctx context.Context, req *group.CreateGroupReq) (resp *group.CreateGroupRes, err error) {
	// TODO: Your code here...
	f, err := rpc.CheckUsersExist(ctx, req.InitialMembers)
	if err != nil {
		return nil, err
	}
	if !f {
		return &group.CreateGroupRes{GroupId: 0}, nil
	}
	groupId, err := s.groupService.CreateGroup(req.CreatorId, req.Name, req.InitialMembers)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]创建群聊时发生系统错误:%v", req.CreatorId, err)
		return nil, err
	}
	resp = &group.CreateGroupRes{
		GroupId: groupId,
	}
	return resp, nil
}

// InviteMembers implements the GroupServiceImpl interface.
func (s *GroupServiceImpl) InviteMembers(ctx context.Context, req *group.InviteMembersReq) (resp *group.CommonRes, err error) {
	// TODO: Your code here...
	success, err := s.groupService.InviteMembers(req.InviterId, req.GroupId, req.UserIds)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]创建[%d]群聊时发生系统错误:%v", req.InviterId, req.GroupId, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

// KickMembers implements the GroupServiceImpl interface.
func (s *GroupServiceImpl) KickMembers(ctx context.Context, req *group.KickMembersReq) (resp *group.CommonRes, err error) {
	// TODO: Your code here...
	success, err := s.groupService.KickMembers(req.OperatorId, req.GroupId, req.UserIds)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]踢出[%d]群成员时发生系统错误:%v", req.OperatorId, req.GroupId, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

// GetGroupInfo implements the GroupServiceImpl interface.
func (s *GroupServiceImpl) GetGroupInfo(ctx context.Context, req *group.GetGroupInfoReq) (resp *group.GetGroupInfoRes, err error) {
	// TODO: Your code here...
	groupInfo, err := s.groupService.GetGroupInfo(req.GroupId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询群聊[%d]时发生系统错误:%v", req.GroupId, err)
		return &group.GetGroupInfoRes{}, err
	}
	var members []*group.GroupMember
	for _, member := range groupInfo.Members {
		members = append(members, &group.GroupMember{
			GroupId:  req.GroupId,
			UserId:   member.UserID,
			Role:     member.Role,
			IsMuted:  member.IsMute,
			JoinTime: member.JoinTime.Unix(),
		})
	}
	resp = &group.GetGroupInfoRes{
		Group: &group.Group{
			GroupId:    groupInfo.GroupId,
			Name:       groupInfo.Name,
			OwnerId:    groupInfo.OwnerID,
			Notice:     groupInfo.Notice,
			CreateTime: groupInfo.CreatedAt.Unix(),
		},
		Members: members,
	}
	return resp, nil
}

// ChangeOwner implements the GroupServiceImpl interface.
func (s *GroupServiceImpl) ChangeOwner(ctx context.Context, req *group.ChangeOwnerReq) (resp *group.CommonRes, err error) {
	// TODO: Your code here...
	success, err := s.groupService.ChangeOwner(req.OldId, req.GroupId, req.NewId_)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]修改[%d]群主给[%d]发生系统错误:%v", req.OldId, req.GroupId, req.NewId_, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

// ChangeNotice implements the GroupServiceImpl interface.
func (s *GroupServiceImpl) ChangeNotice(ctx context.Context, req *group.ChangeNoticeReq) (resp *group.CommonRes, err error) {
	// TODO: Your code here...
	success, err := s.groupService.ChangeNotice(req.OperatorId, req.GroupId, req.Notice)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]修改[%d]群公告时发生系统错误:%v", req.OperatorId, req.GroupId, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

// Muted implements the GroupServiceImpl interface.
func (s *GroupServiceImpl) Muted(ctx context.Context, req *group.MutedReq) (resp *group.CommonRes, err error) {
	// TODO: Your code here...
	success, err := s.groupService.Muted(req.OperatorId, req.GroupId, req.MutedId, req.IsMuted)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]修改[%d]群聊成员[%d]禁言:%v", req.OperatorId, req.GroupId, req.MutedId, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}

// SetAdmin implements the GroupServiceImpl interface.
func (s *GroupServiceImpl) SetAdmin(ctx context.Context, req *group.SetAdminReq) (resp *group.CommonRes, err error) {
	// TODO: Your code here...
	success, err := s.groupService.SetAdmin(req.OperatorId, req.GroupId, req.TargetId, req.Role)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]设置[%d]群聊成员[%d]管理员状态:%v", req.OperatorId, req.GroupId, req.TargetId, err)
		return &group.CommonRes{Success: false}, err
	}
	return &group.CommonRes{Success: success}, nil
}
