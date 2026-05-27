package dal

import (
	"github.com/Airiseina/answer/kitex_service/group_service/internal/model"

	"gorm.io/gorm"
)

type gro struct {
	db *gorm.DB
}

func NewGroupDao(db *gorm.DB) GroupDao {
	return &gro{db}
}

type GroupDao interface {
	CreateGorp(group model.Group, members []model.GroupMember) (int64, error)
	FindMembers(groupId int64) ([]model.GroupMember, error)
	InviteMembers(members []model.GroupMember) error
	GetGroupInfo(groupId int64) (model.Group, error)
	GetGroupAllInfo(groupId int64) (model.Group, error)
	KickMembers(groupId int64, userIds []int64) error
	ChangeOwner(groupId int64, newOwnerId int64) error
	TransferOwner(groupId int64, oldOwnerId int64, newOwnerId int64) error
	ChangeNotice(groupId int64, notice string) error
	Muted(groupId int64, mutedId int64, isMute bool) error
	SetAdmin(groupId int64, targetId int64, role int64) error
	GetUserGroups(userId int64) ([]model.Group, error)
	SearchGroupByNumber(groupNumber int64) (model.Group, error)
	CreateJoinRequest(req model.GroupJoinRequest) error
	HandleJoinRequest(groupId int64, userId int64, accept bool, userName string) error
	GetJoinRequests(groupId int64) ([]model.GroupJoinRequest, error)
	UpdateMemberRole(groupId int64, userId int64, role int64) error
	IsGroupMember(groupId int64, userId int64) (bool, error)
	GetMemberRole(groupId int64, userId int64) (int64, error)
	CheckMuted(groupId int64, userId int64) (bool, error)
	// UpdateConversationID 将 chat_service 返回的 conversationID 回写到群组记录
	// 建立群组与会话的关联，后续邀请/踢人时通过此 ID 同步会话成员
	UpdateConversationID(groupId int64, conversationID int64) error

	// DeleteGroup 删除群组及其所有成员记录（事务操作）
	// 用于 CreateGroup 补偿回滚：会话创建失败时删除已创建的群组
	DeleteGroup(groupId int64) error

	CreateNotice(notice model.GroupNotice) error
	GetNotices(groupId int64) ([]model.GroupNotice, error)
}
