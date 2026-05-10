package dal

import (
	"group_service/internal/model"

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
	ChangeNotice(groupId int64, notice string) error
	Muted(groupId int64, mutedId int64, isMute bool) error
	SetAdmin(groupId int64, targetId int64, role int64) error
}
