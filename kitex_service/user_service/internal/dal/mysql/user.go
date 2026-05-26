package mysql

import (
	"errors"
	"fmt"
	"github.com/Airiseina/answer/kitex_service/user_service/internal/model"
	"time"

	"gorm.io/gorm"
)

func (db *gor) Register(account, name, hash string) error {
	userInfo := model.User{
		Account: account,
		Name:    name,
		Hash:    hash,
	}
	err := db.db.Create(&userInfo).Error
	if err != nil {
		return fmt.Errorf("存入数据失败: %w", err)
	}
	return nil
}

func (db *gor) GetUser(account string) (model.User, error) {
	var user model.User
	err := db.db.Where("account = ?", account).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, nil
		}
		return model.User{}, fmt.Errorf("查询用户失败: %w", err)
	}
	return user, nil
}

func (db *gor) GetUserById(id int64) (model.User, error) {
	var user model.User
	err := db.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, nil
		}
		return model.User{}, fmt.Errorf("查询用户失败: %w", err)
	}
	return user, nil
}

func (db *gor) CountUsersByIds(userIds []int64) (int64, error) {
	var count int64
	err := db.db.Model(&model.User{}).Where("id IN ?", userIds).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("查询用户失败: %w", err)
	}
	return count, nil
}

func (db *gor) CreateFriendRequest(sender, receiver int64, message string) error {
	req := model.FriendRequest{
		Sender:   sender,
		Receiver: receiver,
		Status:   model.FriendRequestPending,
		Message:  message,
	}
	err := db.db.Create(&req).Error
	if err != nil {
		return fmt.Errorf("创建好友请求失败: %w", err)
	}
	return nil
}

func (db *gor) GetFriendRequest(sender, receiver int64) (model.FriendRequest, error) {
	var req model.FriendRequest
	err := db.db.Where("sender = ? AND receiver = ?", sender, receiver).First(&req).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.FriendRequest{}, nil
		}
		return model.FriendRequest{}, fmt.Errorf("查询好友请求失败: %w", err)
	}
	return req, nil
}

func (db *gor) GetFriendRequestBetweenUsers(userA, userB int64) (model.FriendRequest, error) {
	var req model.FriendRequest
	err := db.db.Unscoped().Where(
		"(sender = ? AND receiver = ?) OR (sender = ? AND receiver = ?)",
		userA, userB, userB, userA,
	).First(&req).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.FriendRequest{}, nil
		}
		return model.FriendRequest{}, fmt.Errorf("查询好友请求失败: %w", err)
	}
	return req, nil
}

func (db *gor) DeleteFriendRequest(sender, receiver int64) error {
	err := db.db.Unscoped().Where("sender = ? AND receiver = ?", sender, receiver).Delete(&model.FriendRequest{}).Error
	if err != nil {
		return fmt.Errorf("删除好友请求失败: %w", err)
	}
	return nil
}

func (db *gor) DeleteFriendRequestsBetweenUsers(userA, userB int64) error {
	err := db.db.Unscoped().Where(
		"(sender = ? AND receiver = ?) OR (sender = ? AND receiver = ?)",
		userA, userB, userB, userA,
	).Delete(&model.FriendRequest{}).Error
	if err != nil {
		return fmt.Errorf("删除好友请求失败: %w", err)
	}
	return nil
}

func (db *gor) UpdateFriendRequestStatus(sender, receiver, status int64) error {
	err := db.db.Model(&model.FriendRequest{}).
		Where("sender = ? AND receiver = ?", sender, receiver).
		Update("status", status).Error
	if err != nil {
		return fmt.Errorf("更新好友请求状态失败: %w", err)
	}
	return nil
}

func (db *gor) GetFriendRequestsByReceiver(receiver int64) ([]model.FriendRequest, error) {
	var requests []model.FriendRequest
	err := db.db.Where("receiver = ?", receiver).Find(&requests).Error
	if err != nil {
		return nil, fmt.Errorf("查询好友请求列表失败: %w", err)
	}
	return requests, nil
}

func (db *gor) CreateFriend(userID, friendID, groupID int64) error {
	friend := model.Friend{
		UserID:   userID,
		FriendID: friendID,
		GroupID:  groupID,
	}
	err := db.db.Create(&friend).Error
	if err != nil {
		return fmt.Errorf("创建好友关系失败: %w", err)
	}
	return nil
}

func (db *gor) CreateFriendPair(userA, userB int64, groupID int64) error {
	err := db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model.Friend{UserID: userA, FriendID: userB, GroupID: groupID}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Friend{UserID: userB, FriendID: userA, GroupID: 0}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("创建双向好友关系失败: %w", err)
	}
	return nil
}

func (db *gor) GetFriend(userID, friendID int64) (model.Friend, error) {
	var friend model.Friend
	err := db.db.Unscoped().Where("user_id = ? AND friend_id = ?", userID, friendID).First(&friend).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Friend{}, nil
		}
		return model.Friend{}, fmt.Errorf("查询好友关系失败: %w", err)
	}
	return friend, nil
}

func (db *gor) DeleteFriend(userID, friendID int64) error {
	err := db.db.Unscoped().Where("user_id = ? AND friend_id = ?", userID, friendID).Delete(&model.Friend{}).Error
	if err != nil {
		return fmt.Errorf("删除好友关系失败: %w", err)
	}
	return nil
}

func (db *gor) GetFriendList(userID int64) ([]model.Friend, error) {
	var friends []model.Friend
	err := db.db.Where("user_id = ?", userID).Find(&friends).Error
	if err != nil {
		return nil, fmt.Errorf("查询好友列表失败: %w", err)
	}
	return friends, nil
}

func (db *gor) UpdateFriendGroupID(userID, friendID, groupID int64) error {
	err := db.db.Model(&model.Friend{}).
		Where("user_id = ? AND friend_id = ?", userID, friendID).
		Update("group_id", groupID).Error
	if err != nil {
		return fmt.Errorf("更新好友分组失败: %w", err)
	}
	return nil
}

func (db *gor) UpdateFriendRemark(userID, friendID int64, remark string) error {
	err := db.db.Model(&model.Friend{}).
		Where("user_id = ? AND friend_id = ?", userID, friendID).
		Update("remark", remark).Error
	if err != nil {
		return fmt.Errorf("更新好友备注失败: %w", err)
	}
	return nil
}

func (db *gor) CreateFriendGroup(userID int64, name string) (int64, error) {
	group := model.FriendGroup{
		UserID: userID,
		Name:   name,
	}
	err := db.db.Create(&group).Error
	if err != nil {
		return 0, fmt.Errorf("创建好友分组失败: %w", err)
	}
	return group.ID, nil
}

func (db *gor) GetFriendGroup(groupID int64) (model.FriendGroup, error) {
	var group model.FriendGroup
	err := db.db.Where("id = ?", groupID).First(&group).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.FriendGroup{}, nil
		}
		return model.FriendGroup{}, fmt.Errorf("查询好友分组失败: %w", err)
	}
	return group, nil
}

func (db *gor) UpdateFriendGroup(groupID int64, name string) error {
	err := db.db.Model(&model.FriendGroup{}).Where("id = ?", groupID).Update("name", name).Error
	if err != nil {
		return fmt.Errorf("更新好友分组失败: %w", err)
	}
	return nil
}

func (db *gor) DeleteFriendGroup(groupID int64) error {
	err := db.db.Where("id = ?", groupID).Delete(&model.FriendGroup{}).Error
	if err != nil {
		return fmt.Errorf("删除好友分组失败: %w", err)
	}
	return nil
}

func (db *gor) ResetFriendsGroupID(groupID int64) error {
	err := db.db.Model(&model.Friend{}).Where("group_id = ?", groupID).Update("group_id", 0).Error
	if err != nil {
		return fmt.Errorf("重置好友分组ID失败: %w", err)
	}
	return nil
}

func (db *gor) GetFriendGroupsByUserId(userID int64) ([]model.FriendGroup, error) {
	var groups []model.FriendGroup
	err := db.db.Where("user_id = ?", userID).Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("查询好友分组列表失败: %w", err)
	}
	return groups, nil
}

func (db *gor) GetUsersByIds(userIds []int64) ([]model.User, error) {
	var users []model.User
	err := db.db.Where("id IN ?", userIds).Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("查询用户列表失败: %w", err)
	}
	return users, nil
}

func (db *gor) GetUsersByAccounts(accounts []string) ([]model.User, error) {
	var users []model.User
	err := db.db.Where("account IN ?", accounts).Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("查询用户列表失败: %w", err)
	}
	return users, nil
}

func (db *gor) UpdateAvatar(userID int64, avatarURL string) error {
	err := db.db.Model(&model.User{}).Where("id = ?", userID).Update("avatar_url", avatarURL).Error
	if err != nil {
		return fmt.Errorf("更新头像失败: %w", err)
	}
	return nil
}

func (db *gor) CreateBotUser(name, avatarURL string) (int64, error) {
	botUser := model.User{
		Account:   fmt.Sprintf("bot_%d", time.Now().UnixMilli()),
		Name:      name,
		Hash:      "-",
		AvatarURL: avatarURL,
		IsBot:     true,
	}
	err := db.db.Create(&botUser).Error
	if err != nil {
		return 0, fmt.Errorf("创建Bot用户记录失败: %w", err)
	}
	return botUser.ID, nil
}

func (db *gor) UpdateBotUserName(userID int64, name string) error {
	err := db.db.Model(&model.User{}).Where("id = ? AND is_bot = ?", userID, true).Update("name", name).Error
	if err != nil {
		return fmt.Errorf("更新Bot用户名失败: %w", err)
	}
	return nil
}
