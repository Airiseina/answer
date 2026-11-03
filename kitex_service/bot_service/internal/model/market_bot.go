package model

import "time"

// 市场 Bot 审核状态机。
const (
	MarketStatusDraft     int16 = 0 // 草稿（保存但未提交审核）
	MarketStatusPending   int16 = 1 // 待审核
	MarketStatusPublished int16 = 2 // 已发布（市场全局可见）
	MarketStatusRejected  int16 = 3 // 已驳回
	MarketStatusOffline   int16 = 4 // 已下架
)

// ValidMarketStatus 判断状态取值是否在状态机范围内。
func ValidMarketStatus(status int16) bool {
	switch status {
	case MarketStatusDraft, MarketStatusPending, MarketStatusPublished, MarketStatusRejected, MarketStatusOffline:
		return true
	default:
		return false
	}
}

// MarketBot 是市场中的 Bot 定义。
//
// 按需求约束：只分发「定义」本身（提示词、推荐模型、分类标签），
// 刻意不包含知识库与 MCP 字段，安装者需要自带 api_key/model/base_url。
type MarketBot struct {
	ID           int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	TenantID     int64     `gorm:"not null;default:0;index" json:"tenant_id"` // 发布者所属租户
	PublisherID  int64     `gorm:"not null;index" json:"publisher_id"`
	Name         string    `gorm:"type:varchar(64);not null" json:"name"`
	Description  string    `gorm:"type:varchar(512);default:''" json:"description"`
	AvatarURL    string    `gorm:"type:varchar(500);default:''" json:"avatar_url"`
	SystemPrompt string    `gorm:"type:text;not null" json:"system_prompt"`
	Model        string    `gorm:"type:varchar(100);default:''" json:"model"`
	BaseURL      string    `gorm:"type:varchar(500);default:''" json:"base_url"`
	Category     string    `gorm:"type:varchar(64);default:'';index" json:"category"`
	Tags         string    `gorm:"type:varchar(255);default:''" json:"tags"`
	Status       int16     `gorm:"not null;default:0;index" json:"status"`
	ReviewNote   string    `gorm:"type:varchar(255);default:''" json:"review_note"`
	ReviewedBy   int64     `gorm:"default:0" json:"reviewed_by"`
	ReviewedAt   int64     `gorm:"default:0" json:"reviewed_at"`
	Version      int32     `gorm:"not null;default:0" json:"version"` // 0=未上架，审核通过后递增
	InstallCount int64     `gorm:"not null;default:0" json:"install_count"`
	RatingSum    int64     `gorm:"not null;default:0" json:"rating_sum"`
	RatingCount  int64     `gorm:"not null;default:0" json:"rating_count"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 固定表名，避免 GORM 复数化差异。
func (MarketBot) TableName() string {
	return "market_bot"
}
