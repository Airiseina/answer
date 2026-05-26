package dal

import (
	"fmt"
	"github.com/Airiseina/answer/kitex_service/group_service/internal/model"

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

func (db *gro) TransferOwner(groupId int64, oldOwnerId int64, newOwnerId int64) error {
	err := db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Group{}).Where("id = ?", groupId).Update("owner_id", newOwnerId).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", groupId, oldOwnerId).Update("role", 0).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", groupId, newOwnerId).Update("role", 2).Error; err != nil {
			return err
		}
		return nil
	})
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

func (db *gro) GetUserGroups(userId int64) ([]model.Group, error) {
	var groups []model.Group
	err := db.db.Joins("JOIN group_members ON group_members.group_id = groups.id").
		Where("group_members.user_id = ?", userId).
		Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("查询用户群聊列表失败: %w", err)
	}
	return groups, nil
}

func (db *gro) SearchGroupByNumber(groupNumber int64) (model.Group, error) {
	var group model.Group
	err := db.db.Where("group_number = ?", groupNumber).Find(&group).Error
	if err != nil {
		return model.Group{}, fmt.Errorf("搜索群号失败: %w", err)
	}
	return group, nil
}

func (db *gro) CreateJoinRequest(req model.GroupJoinRequest) error {
	var existing model.GroupJoinRequest
	err := db.db.Where("group_id = ? AND user_id = ? AND status = ?", req.GroupID, req.UserID, model.JoinRequestPending).First(&existing).Error
	if err == nil {
		return fmt.Errorf("已存在待处理的入群申请")
	}
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("查询入群申请失败: %w", err)
	}
	err = db.db.Where("group_id = ? AND user_id = ?", req.GroupID, req.UserID).
		Assign(model.GroupJoinRequest{Status: model.JoinRequestPending, Message: req.Message}).
		FirstOrCreate(&req).Error
	if err != nil {
		return fmt.Errorf("创建入群申请失败: %w", err)
	}
	return nil
}

func (db *gro) HandleJoinRequest(groupId int64, userId int64, accept bool, userName string) error {
	var req model.GroupJoinRequest
	err := db.db.Where("group_id = ? AND user_id = ? AND status = ?", groupId, userId, model.JoinRequestPending).First(&req).Error
	if err != nil {
		return fmt.Errorf("未找到待处理的入群申请: %w", err)
	}
	status := model.JoinRequestRejected
	if accept {
		status = model.JoinRequestAccepted
	}
	result := db.db.Model(&req).Where("status = ?", model.JoinRequestPending).Update("status", status)
	if result.RowsAffected == 0 {
		return fmt.Errorf("入群申请已被处理")
	}
	if accept {
		member := model.GroupMember{
			GroupID: groupId,
			UserID:  userId,
			Name:    userName,
			Role:    0,
		}
		err = db.db.Create(&member).Error
		if err != nil {
			return fmt.Errorf("添加群成员失败: %w", err)
		}
	}
	return nil
}

func (db *gro) GetJoinRequests(groupId int64) ([]model.GroupJoinRequest, error) {
	var requests []model.GroupJoinRequest
	err := db.db.Where("group_id = ?", groupId).Find(&requests).Error
	if err != nil {
		return nil, fmt.Errorf("查询入群申请列表失败: %w", err)
	}
	return requests, nil
}

func (db *gro) UpdateMemberRole(groupId int64, userId int64, role int64) error {
	err := db.db.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", groupId, userId).Update("role", role).Error
	if err != nil {
		return fmt.Errorf("更新成员角色失败: %w", err)
	}
	return nil
}

func (db *gro) IsGroupMember(groupId int64, userId int64) (bool, error) {
	var count int64
	err := db.db.Model(&model.GroupMember{}).Where("group_id = ? AND user_id = ?", groupId, userId).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("查询群成员失败: %w", err)
	}
	return count > 0, nil
}

func (db *gro) GetMemberRole(groupId int64, userId int64) (int64, error) {
	var member model.GroupMember
	err := db.db.Where("group_id = ? AND user_id = ?", groupId, userId).First(&member).Error
	if err != nil {
		return -1, fmt.Errorf("查询成员角色失败: %w", err)
	}
	return member.Role, nil
}

func (db *gro) CheckMuted(groupId int64, userId int64) (bool, error) {
	var member model.GroupMember
	err := db.db.Select("is_muted").Where("group_id = ? AND user_id = ?", groupId, userId).First(&member).Error
	if err != nil {
		return false, fmt.Errorf("查询禁言状态失败: %w", err)
	}
	return member.IsMuted, nil
}

// UpdateConversationID 将 chat_service 返回的 conversationID 回写到群组记录
// 建立群组与会话的关联关系，后续邀请/踢人时通过此 ID 同步会话成员
func (db *gro) UpdateConversationID(groupId int64, conversationID int64) error {
	err := db.db.Model(&model.Group{}).Where("id = ?", groupId).Update("conversation_id", conversationID).Error
	if err != nil {
		return fmt.Errorf("更新会话ID失败: %w", err)
	}
	return nil
}

func (db *gro) DeleteGroup(groupId int64) error {
	err := db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupId).Delete(&model.GroupMember{}).Error; err != nil {
			return fmt.Errorf("删除群成员失败: %w", err)
		}
		if err := tx.Where("id = ?", groupId).Delete(&model.Group{}).Error; err != nil {
			return fmt.Errorf("删除群组失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("删除群组失败: %w", err)
	}
	return nil
}
