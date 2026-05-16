package model

import (
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        int64          `gorm:"primarykey"`
	CreatedAt int64          `gorm:"autoCreateTime"`
	UpdatedAt int64          `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type User struct {
	BaseModel
	Account string `json:"account" gorm:"type:varchar(20);uniqueIndex;not null"`
	Name    string `json:"name" gorm:"type:varchar(20);not null"`
	Hash    string `json:"hash" gorm:"not null"`
}

const (
	FriendRequestPending  = 0
	FriendRequestAccepted = 1
	FriendRequestRejected = 2
)

type FriendRequest struct {
	BaseModel
	Sender   int64  `json:"sender" gorm:"index:idx_sender_receiver,unique;not null"`
	Receiver int64  `json:"receiver" gorm:"index:idx_sender_receiver,unique;not null"`
	Status   int64  `json:"status" gorm:"type:tinyint;default:0;not null"`
	Message  string `json:"message" gorm:"type:varchar(200)"`
}

type Friend struct {
	BaseModel
	UserID   int64  `json:"user_id" gorm:"index:idx_user_friend,unique;not null"`
	FriendID int64  `json:"friend_id" gorm:"index:idx_user_friend,unique;not null"`
	GroupID  int64  `json:"group_id" gorm:"index:idx_group;default:0;not null"`
	Remark   string `json:"remark" gorm:"type:varchar(50)"`
}

type FriendGroup struct {
	BaseModel
	UserID int64  `json:"user_id" gorm:"index:idx_user_group,unique;not null"`
	Name   string `json:"name" gorm:"type:varchar(50);index:idx_user_group,unique;not null"`
}
