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

// 管理后台角色。与普通用户体系完全独立，不复用 user 表。
const (
	RoleSuper    = "super"    // 超级管理员：全部权限
	RoleOps      = "ops"      // 运营：市场审核、租户治理
	RoleReviewer = "reviewer" // 审核员：仅市场审核
)

// 账号状态。
const (
	StatusEnabled  = int16(0) // 正常
	StatusDisabled = int16(1) // 停用
)

// AdminUser 管理后台账号。
// 初始账号由配置文件 admin.bootstrap 幂等创建，后续在后台维护。
type AdminUser struct {
	BaseModel
	Account     string `json:"account" gorm:"type:varchar(32);uniqueIndex;not null"`
	Name        string `json:"name" gorm:"type:varchar(32);not null"`
	Hash        string `json:"hash" gorm:"not null"`
	Role        string `json:"role" gorm:"type:varchar(32);not null;index"`
	Status      int16  `json:"status" gorm:"type:tinyint;default:0;not null;index"`
	LastLoginAt int64  `json:"last_login_at" gorm:"default:0"`
}
