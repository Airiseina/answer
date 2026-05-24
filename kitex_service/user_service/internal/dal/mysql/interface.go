package mysql

import (
	"user_service/internal/model"

	"gorm.io/gorm"
)

type gor struct {
	db *gorm.DB
}

func NewUserDao(db *gorm.DB) UserDao {
	return &gor{db}
}

type UserDao interface {
	Register(account, name, hash string) error
	GetUser(account string) (model.User, error)
	GetUserById(id int64) (model.User, error)
	CountUsersByIds(userIds []int64) (int64, error)
	CreateFriendRequest(sender, receiver int64, message string) error
	GetFriendRequest(sender, receiver int64) (model.FriendRequest, error)
	GetFriendRequestBetweenUsers(userA, userB int64) (model.FriendRequest, error)
	DeleteFriendRequest(sender, receiver int64) error
	DeleteFriendRequestsBetweenUsers(userA, userB int64) error
	UpdateFriendRequestStatus(sender, receiver, status int64) error
	GetFriendRequestsByReceiver(receiver int64) ([]model.FriendRequest, error)
	CreateFriend(userID, friendID, groupID int64) error
	CreateFriendPair(userA, userB int64, groupID int64) error
	GetFriend(userID, friendID int64) (model.Friend, error)
	DeleteFriend(userID, friendID int64) error
	GetFriendList(userID int64) ([]model.Friend, error)
	UpdateFriendGroupID(userID, friendID, groupID int64) error
	UpdateFriendRemark(userID, friendID int64, remark string) error
	CreateFriendGroup(userID int64, name string) (int64, error)
	GetFriendGroup(groupID int64) (model.FriendGroup, error)
	UpdateFriendGroup(groupID int64, name string) error
	DeleteFriendGroup(groupID int64) error
	ResetFriendsGroupID(groupID int64) error
	GetFriendGroupsByUserId(userID int64) ([]model.FriendGroup, error)
	GetUsersByIds(userIds []int64) ([]model.User, error)
	GetUsersByAccounts(accounts []string) ([]model.User, error)
	UpdateAvatar(userID int64, avatarURL string) error
	CreateBotUser(name, avatarURL string) (int64, error)
}
