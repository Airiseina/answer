package dal

import (
	"fmt"
	"group_service/internal/model"

	"gorm.io/gorm"
)

func (db *gro) CreateGorp(group model.Group, members []model.GroupMember) (int64, error) {
	err := db.db.Transaction(func(db *gorm.DB) error {
		err := db.Create(&group).Error
		if err != nil {
			return err
		}
		for i := range members {
			members[i].GroupID = group.ID
		}
		return db.Create(&members).Error
	})
	if err != nil {
		return 0, fmt.Errorf("创建群聊失败: %w", err)
	}
	return group.ID, nil
}

func (db *gro) FindMembers(groupId int64) ([]model.GroupMember, error) {
	var members []model.GroupMember
	err := db.db.Where("group_id = ?", groupId).Find(&members).Error
	if err != nil {
		return nil, fmt.Errorf("查找群聊成员失败: %w", err)
	}
	return members, nil
}

func (db *gro) InviteMembers(members []model.GroupMember) error {
	err := db.db.Create(&members).Error
	if err != nil {
		return fmt.Errorf("邀请群成员失败: %w", err)
	}
	return nil
}

func (db *gro) GetGroupInfo(groupId int64) (model.Group, error) {
	var group model.Group
	err := db.db.Where("id = ?", groupId).Find(&group).Error
	if err != nil {
		return model.Group{}, fmt.Errorf("查询群聊信息失败: %w", err)
	}
	return group, nil
}

func (db *gro) GetGroupAllInfo(groupId int64) (model.Group, error) {
	var info model.Group
	err := db.db.Where("id = ?", groupId).Preload("Members").Find(&info).Error
	if err != nil {
		return model.Group{}, fmt.Errorf("查询群聊成员失败: %w", err)
	}
	return info, nil
}

func (db *gro) KickMembers(groupId int64, userIds []int64) error {
	var members []model.GroupMember
	for _, userId := range userIds {
		members = append(members, model.GroupMember{
			GroupID: groupId,
			UserID:  userId,
		})
	}
	err := db.db.Where("group_id = ? AND user_id IN ?", groupId, userIds).Delete(&members).Error
	if err != nil {
		return fmt.Errorf("移出群成员失败: %w", err)
	}
	return nil
}

func (db *gro) ChangeOwner(groupId int64, newOwnerId int64) error {
	err := db.db.Model(&model.Group{}).Where("id = ?", groupId).Update("owner_id", newOwnerId).Error
	if err != nil {
		return fmt.Errorf("转让群主失败: %w", err)
	}
	return nil
}

func (db *gro) ChangeNotice(groupId int64, notice string) error {
	err := db.db.Model(&model.Group{}).Where("id = ?", groupId).Update("notice", notice).Error
	if err != nil {
		return fmt.Errorf("修改群公告失败: %w", err)
	}
	return nil
}

func (db *gro) Muted(groupId int64, mutedId int64, isMute bool) error {
	err := db.db.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", groupId, mutedId).Update("is_muted", isMute).Error
	if err != nil {
		return fmt.Errorf("修改禁言状态失败: %w", err)
	}
	return nil
}

func (db *gro) SetAdmin(groupId int64, targetId int64, role int64) error {
	err := db.db.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", groupId, targetId).Update("role", role).Error
	if err != nil {
		return fmt.Errorf("设置管理员状态失败: %w", err)
	}
	return nil
}
