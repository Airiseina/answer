package model

import (
	"time"
)

type Group struct {
	ID          int64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string        `gorm:"type:varchar(100);not null" json:"name"`
	OwnerID     int64         `gorm:"not null;index" json:"owner_id"`
	Members     []GroupMember `gorm:"foreignKey:GroupID;references:ID"`
	Notice      string        `gorm:"type:text" json:"notice"`
	GroupNumber int64         `gorm:"uniqueIndex;not null" json:"group_number"`
	CreateTime  time.Time     `gorm:"autoCreateTime" json:"create_time"`
}

type GroupMember struct {
	ID       int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupID  int64     `gorm:"index:idx_group_user,unique;not null" json:"group_id"`
	UserID   int64     `gorm:"index:idx_group_user,unique;not null" json:"user_id"`
	Name     string    `gorm:"type:varchar(100);not null" json:"name"`
	Role     int64     `gorm:"type:tinyint;default:0;comment:角色 0:普通用户 1:管理员 2:群主"`
	IsMuted  bool      `gorm:"default:false" json:"is_muted"`
	JoinTime time.Time `gorm:"autoCreateTime" json:"join_time"`
}

const (
	JoinRequestPending  = 0
	JoinRequestAccepted = 1
	JoinRequestRejected = 2
)

type GroupJoinRequest struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupID    int64     `gorm:"index:idx_group_user_join,unique;not null" json:"group_id"`
	UserID     int64     `gorm:"index:idx_group_user_join,unique;not null" json:"user_id"`
	Message    string    `gorm:"type:varchar(200)" json:"message"`
	Status     int64     `gorm:"type:tinyint;default:0;not null" json:"status"`
	CreateTime time.Time `gorm:"autoCreateTime" json:"create_time"`
}
