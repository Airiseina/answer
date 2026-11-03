package model

import (
	"time"
)

type Group struct {
	ID             int64         `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID       int64         `gorm:"not null;default:0;index;uniqueIndex:idx_tenant_group_number" json:"tenant_id"`
	Name           string        `gorm:"type:varchar(100);not null" json:"name"`
	OwnerID        int64         `gorm:"not null;index" json:"owner_id"`
	Members        []GroupMember `gorm:"foreignKey:GroupID;references:ID"`
	Notice         string        `gorm:"type:text" json:"notice"`
	GroupNumber    int64         `gorm:"uniqueIndex:idx_tenant_group_number;not null" json:"group_number"`
	ConversationID int64         `gorm:"not null;index" json:"conversation_id"` // 关联的会话ID，创建群组时由 chat_service 返回
	CreateTime     time.Time     `gorm:"autoCreateTime" json:"create_time"`
}

type GroupMember struct {
	ID       int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID int64     `gorm:"not null;default:0;index;uniqueIndex:idx_tenant_group_user" json:"tenant_id"`
	GroupID  int64     `gorm:"index:idx_tenant_group_user,unique;not null" json:"group_id"`
	UserID   int64     `gorm:"index:idx_tenant_group_user,unique;not null" json:"user_id"`
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
	TenantID   int64     `gorm:"not null;default:0;index;uniqueIndex:idx_tenant_group_user_join" json:"tenant_id"`
	GroupID    int64     `gorm:"index:idx_tenant_group_user_join,unique;not null" json:"group_id"`
	UserID     int64     `gorm:"index:idx_tenant_group_user_join,unique;not null" json:"user_id"`
	Message    string    `gorm:"type:varchar(200)" json:"message"`
	Status     int64     `gorm:"type:tinyint;default:0;not null" json:"status"`
	CreateTime time.Time `gorm:"autoCreateTime" json:"create_time"`
}

type GroupNotice struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID   int64     `gorm:"not null;default:0;index" json:"tenant_id"`
	GroupID    int64     `gorm:"not null;index" json:"group_id"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	OperatorID int64     `gorm:"not null" json:"operator_id"`
	CreateTime time.Time `gorm:"autoCreateTime" json:"create_time"`
}
