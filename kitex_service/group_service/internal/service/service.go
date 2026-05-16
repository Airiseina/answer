package service

import (
	"group_service/internal/dal"
	"group_service/internal/model"
	"time"

	"answer_pkg/snowflake"
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

func (service GroupService) CreateGroup(createId int64, name string, membersId []int64, nameMap map[int64]string) (int64, int64, error) {
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
		return 0, 0, err
	}
	return groupId, groupNumber, nil
}

func (service GroupService) InviteMembers(inviterId int64, groupId int64, membersId []int64, nameMap map[int64]string) (bool, error) {
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
	return true, nil
}

func (service GroupService) KickMembers(operatorId int64, groupId int64, membersId []int64) (bool, error) {
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
	err = service.dao.ChangeOwner(groupId, newOwnerId)
	if err != nil {
		return false, err
	}
	err = service.dao.UpdateMemberRole(groupId, operatorId, 0)
	if err != nil {
		return false, err
	}
	err = service.dao.UpdateMemberRole(groupId, newOwnerId, 2)
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

func (service GroupService) HandleJoinRequest(operatorId int64, groupId int64, userId int64, accept bool, userName string) (bool, error) {
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
