package service

import (
	"group_service/internal/dal"
	"group_service/internal/model"
	"time"
)

type GroupService struct {
	dao dal.GroupDao
}

func NewGroupService(dao dal.GroupDao) *GroupService {
	return &GroupService{dao: dao}
}

func (service GroupService) CreateGroup(createId int64, name string, membersId []int64) (int64, error) {
	group := model.Group{
		Name:    name,
		OwnerID: createId,
	}
	var members []model.GroupMember
	for _, memberId := range membersId {
		members = append(members, model.GroupMember{
			UserID: memberId,
		})
	}
	return service.dao.CreateGorp(group, members)
}

func (service GroupService) InviteMembers(inviterId int64, groupId int64, membersId []int64) (bool, error) {
	info, err := service.dao.GetGroupInfo(groupId)
	if err != nil {
		return false, err
	}
	if inviterId != info.OwnerID {
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
		})
	}
	err = service.dao.InviteMembers(members)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (service GroupService) KickMembers(operatorId int64, groupId int64, membersId []int64) (bool, error) { //权限不足或未选取拉取成员
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
	// 校验成员是否存在以及排除踢自己和群主
	for _, id := range membersId {
		if !memberMap[id] && id == operatorId && id == info.OwnerID {
			return false, nil
		}
	}
	err = service.dao.KickMembers(groupId, membersId)
	if err != nil {
		return false, err
	}
	return true, nil
}

type GroupDTO struct {
	GroupId   int64
	Name      string
	OwnerID   int64
	Members   []GroupMemberDTO
	Notice    string
	CreatedAt time.Time
}
type GroupMemberDTO struct {
	UserID   int64
	Role     int64
	IsMute   bool
	JoinTime time.Time
}

func (service GroupService) GetGroupInfo(groupId int64) (GroupDTO, error) {
	group, err := service.dao.GetGroupAllInfo(groupId)
	if err != nil {
		return GroupDTO{}, err
	}
	var membersDTO []GroupMemberDTO
	for _, member := range group.Members {
		membersDTO = append(membersDTO, GroupMemberDTO{
			UserID:   member.UserID,
			Role:     member.Role,
			IsMute:   member.IsMuted,
			JoinTime: member.JoinTime,
		})
	}
	groupDTO := GroupDTO{
		GroupId:   groupId,
		Name:      group.Name,
		OwnerID:   group.OwnerID,
		Members:   membersDTO,
		Notice:    group.Notice,
		CreatedAt: group.CreateTime,
	}
	return groupDTO, nil
}

func (service GroupService) ChangeOwner(operatorId int64, groupId int64, newOwnerId int64) (bool, error) { //权限不足或新群主不在群里
	info, err := service.dao.GetGroupInfo(groupId)
	if err != nil {
		return false, err
	}
	if operatorId != info.OwnerID {
		return false, nil
	}
	// 确认新群主在群内
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
	err = service.dao.ChangeOwner(groupId, newOwnerId)
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
		return false, nil // 成员不在群内
	}
	if operatorId == mutedId || mutedId == info.OwnerID {
		return false, nil // 不能禁言自己或群主
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
		return false, nil // 只有群主可以设置管理员
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
		return false, nil // 目标成员不在群内
	}
	if targetId == info.OwnerID {
		return false, nil // 不能给群主设置/取消管理员
	}
	err = service.dao.SetAdmin(groupId, targetId, role)
	if err != nil {
		return false, err
	}
	return true, nil
}
