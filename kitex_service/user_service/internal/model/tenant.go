package model

import (
	"strings"

	"gorm.io/gorm"
)

// 租户成员角色：数字越大权限越高。
const (
	TenantRoleMember int16 = 0
	TenantRoleAdmin  int16 = 1
	TenantRoleOwner  int16 = 2
)

// 租户状态
const (
	TenantStatusNormal   int16 = 0
	TenantStatusDisabled int16 = 1
)

// 租户成员状态
const (
	TenantMemberNormal  int16 = 0
	TenantMemberPending int16 = 1
	TenantMemberRemoved int16 = 2
)

// 平台租户固定 ID：系统 Bot 与全局可见资源归属此租户。
const PlatformTenantID int64 = 1

// DefaultTenantSettings 是租户级配置的默认 JSON 文档。
//
// tenant.settings 在 MySQL 中以 JSON 列存储，MySQL 会直接拒绝空字符串（Error 3140），
// 而该字段是预留的扩展位，默认语义就是「没有任何额外配置」的空对象，
// 因此空值统一归一化为 '{}'，见 Tenant.BeforeSave。
const DefaultTenantSettings = "{}"

// Tenant 租户（个人租户 is_personal=1 / 组织租户 is_personal=0）
type Tenant struct {
	BaseModel
	Name       string `json:"name" gorm:"type:varchar(50);not null"`
	Code       string `json:"code" gorm:"type:varchar(32);uniqueIndex;not null"`
	LogoURL    string `json:"logo_url" gorm:"type:varchar(500);default:''"`
	OwnerID    int64  `json:"owner_id" gorm:"index;not null"`
	IsPersonal bool   `json:"is_personal" gorm:"default:false;index"`
	Status     int16  `json:"status" gorm:"type:tinyint;default:0;not null"`
	Plan       string `json:"plan" gorm:"type:varchar(20);default:'free'"`
	Settings   string `json:"settings" gorm:"type:json"`
}

// BeforeSave 保证 settings 始终是合法 JSON 文档。
//
// string 字段的零值是空字符串，而 MySQL 的 JSON 列不能接受空串，
// 新建租户（尤其是启动时的平台租户）会因此直接写入失败；
// 在写入前兜底既避免了逐个创建点重复赋值，也不会覆盖调用方显式传入的 JSON。
func (t *Tenant) BeforeSave(_ *gorm.DB) error {
	if strings.TrimSpace(t.Settings) == "" {
		t.Settings = DefaultTenantSettings
	}
	return nil
}

// TableName 固定租户表名，避免 GORM 复数化差异（DAO 中存在裸表名查询）。
func (Tenant) TableName() string {
	return "tenant"
}

// TenantMember 用户与租户的多对多关系
type TenantMember struct {
	TenantID int64 `json:"tenant_id" gorm:"primaryKey"`
	UserID   int64 `json:"user_id" gorm:"primaryKey"`
	Role     int16 `json:"role" gorm:"type:tinyint;default:0;not null"`
	DeptID   int64 `json:"dept_id" gorm:"index;default:0"`
	Status   int16 `json:"status" gorm:"type:tinyint;default:0;not null"`
	JoinedAt int64 `json:"joined_at" gorm:"autoCreateTime"`
}

// TableName 固定租户成员表名。
func (TenantMember) TableName() string {
	return "tenant_member"
}

// TenantDepartment 租户内的部门树
type TenantDepartment struct {
	BaseModel
	TenantID int64  `json:"tenant_id" gorm:"index;not null"`
	ParentID int64  `json:"parent_id" gorm:"index;default:0"`
	Name     string `json:"name" gorm:"type:varchar(50);not null"`
	Order    int32  `json:"order" gorm:"column:sort_order;default:0"`
}

// TableName 固定租户部门表名。
func (TenantDepartment) TableName() string {
	return "tenant_department"
}
