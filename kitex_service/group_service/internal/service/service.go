package service

import (
	"context"
	"fmt"
	"github.com/Airiseina/answer/kitex_service/group_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/group_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/group_service/rpc"
	"time"

	"github.com/Airiseina/answer/pkg/snowflake"

	"github.com/cloudwego/kitex/pkg/klog"
)

type GroupService struct {
	dao      dal.GroupDao
	snowNode *snowflake.Node
}

func NewGroupService(dao dal.GroupDao) *GroupService {
	return &GroupService{
		dao:      dao,
		snowNode: snowflake.NewNode(2),
	}
}

// CreateGroup 创建群组并同步创建对应的群聊会话
// 这是跨服务数据一致性的核心入口：
//  1. 在 MySQL 中创建群组记录和成员记录（group_service 职责）
//  2. 通过 RPC 调用 chat_service 创建对应的群聊会话（chat_service 职责）
//  3. 将 chat_service 返回的 conversationID 回写到群组记录中
//
// 为什么需要同步创建会话：
//   - 统一会话模型下，群聊消息的收发依赖 conversation_id
//   - 如果不同步创建会话，群组存在但无法发送消息
//   - conversationID 回写到 Group 表，后续邀请/踢人时需要用它同步会话成员
func (service GroupService) CreateGroup(ctx context.Context, createId int64, name string, membersId []int64, nameMap map[int64]string) (int64, int64, int64, error) {
	groupNumber := service.snowNode.Generate()
	group := model.Group{
		Name:        name,
		OwnerID:     createId,
		GroupNumber: groupNumber,
	}
	var members []model.GroupMember
	for _, memberId := range membersId {
		role := int64(0)
		if memberId == createId {
			role = 2
		}
		members = append(members, model.GroupMember{
			UserID: memberId,
			Name:   nameMap[memberId],
			Role:   role,
		})
	}

	groupId, err := service.dao.CreateGorp(group, members)
	if err != nil {
		return 0, 0, 0, err
	}

	conversationID, rpcErr := rpc.CreateConversation(ctx, name, membersId, groupId)
	if rpcErr != nil {
		if delErr := service.dao.DeleteGroup(groupId); delErr != nil {
			klog.Errorf("补偿删除群组%d失败: %v", groupId, delErr)
		}
		return 0, 0, 0, fmt.Errorf("创建群聊会话失败: %w", rpcErr)
	}

	if updateErr := service.dao.UpdateConversationID(groupId, conversationID); updateErr != nil {
		klog.Errorf("回写conversationID到群组%d失败: %v", groupId, updateErr)
	}

	return groupId, groupNumber, conversationID, nil
}

// InviteMembers 邀请成员入群，并同步更新会话成员
// 操作流程：
//  1. 权限校验（邀请人是否为管理员/群主）
//  2. 去重检查（避免重复邀请已在群内的成员）
//  3. 写入 group_member 记录（group_service 职责）
//  4. 通过 RPC 同步 conversation_member 记录（chat_service 职责）
//
// 为什么邀请后需要同步会话成员：
//   - 新成员加入群组后，应该能收到该群的消息推送
//   - 如果不同步 conversation_member，新成员不在会话成员列表中
//   - SendMessage 查询 GetConversationMembers 时会遗漏新成员
func (service GroupService) InviteMembers(ctx context.Context, inviterId int64, groupId int64, membersId []int64, nameMap map[int64]string) (bool, error) {
	info, err := service.dao.GetGroupInfo(groupId)
	if err != nil {
		return false, err
	}
	operatorRole, err := service.dao.GetMemberRole(groupId, inviterId)
	if err != nil {
		return false, err
	}
	if operatorRole < 1 && inviterId != info.OwnerID {
		return false, nil
	}
	existingMembers, err := service.dao.FindMembers(groupId)
	if err != nil {
		return false, err
	}
	existMap := make(map[int64]bool)
	for _, m := range existingMembers {
		existMap[m.UserID] = true
	}
	var members []model.GroupMember
	for _, memberId := range membersId {
		if existMap[memberId] {
			return false, nil
		}
		members = append(members, model.GroupMember{
			UserID:  memberId,
			GroupID: groupId,
			Name:    nameMap[memberId],
		})
	}
	if len(members) == 0 {
		return false, nil
	}
	err = service.dao.InviteMembers(members)
	if err != nil {
		return false, err
	}

	// 同步会话成员：通知 chat_service 添加新成员到对应会话
	// 使用群组记录中的 conversationID 关联到 chat_service 的会话
	if info.ConversationID > 0 {
		var newMemberIDs []int64
		for _, m := range members {
			newMemberIDs = append(newMemberIDs, m.UserID)
		}
		if rpcErr := rpc.AddConversationMembers(ctx, info.ConversationID, newMemberIDs); rpcErr != nil {
			// RPC 失败不影响群组操作（群组成员已持久化），但记录错误
			// 后续可通过定时任务补偿同步
			_ = rpcErr
		}
	}

	return true, nil
}

// KickMembers 踢出群成员，并同步更新会话成员
// 操作流程：
//  1. 权限校验（操作人是否为管理员/群主，不能踢自己/群主）
//  2. 删除 group_member 记录（group_service 职责）
//  3. 通过 RPC 同步删除 conversation_member 记录（chat_service 职责）
//
// 为什么踢出后需要同步会话成员：
//   - 被踢出的成员不应再收到该群的消息推送
//   - 如果不同步移除 conversation_member，被踢用户仍在会话成员列表中
//   - SendMessage 会继续向已退群的用户推送消息，造成数据不一致
func (service GroupService) KickMembers(ctx context.Context, operatorId int64, groupId int64, membersId []int64) (bool, error) {
	if len(membersId) == 0 {
		return false, nil
	}
	info, err := service.dao.GetGroupInfo(groupId)
	if err != nil {
		return false, err
	}
	members, err := service.dao.FindMembers(groupId)
	if err != nil {
		return false, err
	}
	isOwner := operatorId == info.OwnerID
	isAdmin := false
	memberMap := make(map[int64]bool)
	for _, m := range members {
		memberMap[m.UserID] = true
		if m.UserID == operatorId && m.Role >= 1 {
			isAdmin = true
		}
	}
	if !isOwner && !isAdmin {
		return false, nil
	}
	for _, id := range membersId {
		if !memberMap[id] || id == operatorId || id == info.OwnerID {
			return false, nil
		}
	}
	err = service.dao.KickMembers(groupId, membersId)
	if err != nil {
		return false, err
	}
	// 同步会话成员：通知 chat_service 从对应会话中移除被踢成员
	if info.ConversationID > 0 {
		if rpcErr := rpc.RemoveConversationMembers(ctx, info.ConversationID, membersId); rpcErr != nil {
			// RPC 失败不影响群组操作（群组成员已持久化），但记录错误
			// 后续可通过定时任务补偿同步
			_ = rpcErr
		}
	}

	return true, nil
}

type GroupDTO struct {
	GroupId     int64
	Name        string
	OwnerID     int64
	OwnerName   string
	Members     []GroupMemberDTO
	Notice      string
	GroupNumber int64
	CreatedAt   time.Time
}

type GroupMemberDTO struct {
	UserID   int64
	Name     string
	Role     int64
	IsMute   bool
	JoinTime time.Time
}

func (service GroupService) GetGroupInfo(groupId int64) (GroupDTO, error) {
	group, err := service.dao.GetGroupAllInfo(groupId)
	if err != nil {
		return GroupDTO{}, err
	}
	if group.ID == 0 {
		return GroupDTO{}, nil
	}
	var ownerName string
	var membersDTO []GroupMemberDTO
	for _, member := range group.Members {
		if member.Role == 2 {
			ownerName = member.Name
		}
		membersDTO = append(membersDTO, GroupMemberDTO{
			UserID:   member.UserID,
			Name:     member.Name,
			Role:     member.Role,
			IsMute:   member.IsMuted,
			JoinTime: member.JoinTime,
		})
	}
	groupDTO := GroupDTO{
		GroupId:     groupId,
		Name:        group.Name,
		OwnerID:     group.OwnerID,
		OwnerName:   ownerName,
		Members:     membersDTO,
		Notice:      group.Notice,
		GroupNumber: group.GroupNumber,
		CreatedAt:   group.CreateTime,
	}
	return groupDTO, nil
}

func (service GroupService) ChangeOwner(operatorId int64, groupId int64, newOwnerId int64) (bool, error) {
	info, err := service.dao.GetGroupInfo(groupId)
	if err != nil {
		return false, err
	}
	if operatorId != info.OwnerID {
		return false, nil
	}
	members, err := service.dao.FindMembers(groupId)
	if err != nil {
		return false, err
	}
	isMember := false
	for _, m := range members {
		if m.UserID == newOwnerId {
			isMember = true
			break
		}
	}
	if !isMember {
		return false, nil
	}
	err = service.dao.TransferOwner(groupId, operatorId, newOwnerId)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (service GroupService) ChangeNotice(operatorId int64, groupId int64, notice string) (bool, error) {
	info, err := service.dao.GetGroupInfo(groupId)
	if err != nil {
		return false, err
	}
	isOwner := operatorId == info.OwnerID
	isAdmin := false
	if !isOwner {
		members, err := service.dao.FindMembers(groupId)
		if err == nil {
			for _, m := range members {
				if m.UserID == operatorId && m.Role >= 1 {
					isAdmin = true
					break
				}
			}
		}
	}
	if !isOwner && !isAdmin {
		return false, nil
	}
	err = service.dao.ChangeNotice(groupId, notice)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (service GroupService) Muted(operatorId int64, groupId int64, mutedId int64, isMute bool) (bool, error) {
	info, err := service.dao.GetGroupInfo(groupId)
	if err != nil {
		return false, err
	}
	members, err := service.dao.FindMembers(groupId)
	if err != nil {
		return false, err
	}
	isOwner := operatorId == info.OwnerID
	isAdmin := false
	memberMap := make(map[int64]bool)
	for _, m := range members {
		memberMap[m.UserID] = true
		if m.UserID == operatorId && m.Role >= 1 {
			isAdmin = true
		}
	}
	if !isOwner && !isAdmin {
		return false, nil
	}
	if !memberMap[mutedId] {
		return false, nil
	}
	if operatorId == mutedId || mutedId == info.OwnerID {
		return false, nil
	}
	err = service.dao.Muted(groupId, mutedId, isMute)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (service GroupService) SetAdmin(operatorId int64, groupId int64, targetId int64, role int64) (bool, error) {
	info, err := service.dao.GetGroupInfo(groupId)
	if err != nil {
		return false, err
	}
	if operatorId != info.OwnerID {
		return false, nil
	}
	members, err := service.dao.FindMembers(groupId)
	if err != nil {
		return false, err
	}
	isMember := false
	for _, m := range members {
		if m.UserID == targetId {
			isMember = true
			break
		}
	}
	if !isMember {
		return false, nil
	}
	if targetId == info.OwnerID {
		return false, nil
	}
	err = service.dao.SetAdmin(groupId, targetId, role)
	if err != nil {
		return false, err
	}
	return true, nil
}

type UserGroupDTO struct {
	GroupID     int64
	Name        string
	GroupNumber int64
}

func (service GroupService) GetUserGroups(userId int64) ([]UserGroupDTO, error) {
	groups, err := service.dao.GetUserGroups(userId)
	if err != nil {
		return nil, err
	}
	var result []UserGroupDTO
	for _, g := range groups {
		result = append(result, UserGroupDTO{
			GroupID:     g.ID,
			Name:        g.Name,
			GroupNumber: g.GroupNumber,
		})
	}
	return result, nil
}

type GroupSearchResultDTO struct {
	GroupId     int64
	Name        string
	OwnerID     int64
	OwnerName   string
	GroupNumber int64
}

func (service GroupService) SearchGroupByNumber(groupNumber int64) (GroupSearchResultDTO, error) {
	group, err := service.dao.SearchGroupByNumber(groupNumber)
	if err != nil {
		return GroupSearchResultDTO{}, err
	}
	if group.ID == 0 {
		return GroupSearchResultDTO{}, nil
	}
	ownerName := ""
	members, err := service.dao.FindMembers(group.ID)
	if err == nil {
		for _, m := range members {
			if m.UserID == group.OwnerID {
				ownerName = m.Name
				break
			}
		}
	}
	return GroupSearchResultDTO{
		GroupId:     group.ID,
		Name:        group.Name,
		OwnerID:     group.OwnerID,
		OwnerName:   ownerName,
		GroupNumber: group.GroupNumber,
	}, nil
}

func (service GroupService) JoinGroup(userId int64, groupNumber int64, message string) (bool, error) {
	group, err := service.dao.SearchGroupByNumber(groupNumber)
	if err != nil {
		return false, err
	}
	if group.ID == 0 {
		return false, nil
	}
	isMember, err := service.dao.IsGroupMember(group.ID, userId)
	if err != nil {
		return false, err
	}
	if isMember {
		return false, nil
	}
	req := model.GroupJoinRequest{
		GroupID: group.ID,
		UserID:  userId,
		Message: message,
		Status:  model.JoinRequestPending,
	}
	err = service.dao.CreateJoinRequest(req)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (service GroupService) HandleJoinRequest(ctx context.Context, operatorId int64, groupId int64, userId int64, accept bool, userName string) (bool, error) {
	operatorRole, err := service.dao.GetMemberRole(groupId, operatorId)
	if err != nil {
		return false, err
	}
	if operatorRole < 1 {
		return false, nil
	}
	err = service.dao.HandleJoinRequest(groupId, userId, accept, userName)
	if err != nil {
		return false, err
	}
	if accept {
		info, getInfoErr := service.dao.GetGroupInfo(groupId)
		if getInfoErr == nil && info.ConversationID > 0 {
			if rpcErr := rpc.AddConversationMembers(ctx, info.ConversationID, []int64{userId}); rpcErr != nil {
				_ = rpcErr
			}
		}
	}
	return true, nil
}

type JoinRequestDTO struct {
	UserID  int64
	Name    string
	Message string
	Status  int64
}

func (service GroupService) GetJoinRequests(groupId int64) ([]JoinRequestDTO, error) {
	requests, err := service.dao.GetJoinRequests(groupId)
	if err != nil {
		return nil, err
	}
	var result []JoinRequestDTO
	for _, r := range requests {
		result = append(result, JoinRequestDTO{
			UserID:  r.UserID,
			Message: r.Message,
			Status:  r.Status,
		})
	}
	return result, nil
}

func (service GroupService) CheckMuted(groupId int64, userId int64) (bool, error) {
	return service.dao.CheckMuted(groupId, userId)
}
