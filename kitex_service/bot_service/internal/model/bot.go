package model

import "time"

// Bot 来源，决定是否纳入用户用量统计。
const (
	BotOriginUser    int16 = 0 // 用户自建
	BotOriginMarket  int16 = 1 // 从市场安装
	BotOriginOpenAPI int16 = 2 // 通过 OpenAPI 接入（不计入用量统计）
)

// ValidOrigin 判断 Bot 来源取值是否合法，非法时调用方应回退到 BotOriginUser。
func ValidOrigin(origin int16) bool {
	switch origin {
	case BotOriginUser, BotOriginMarket, BotOriginOpenAPI:
		return true
	default:
		return false
	}
}

type Bot struct {
	ID           int64  `gorm:"primaryKey;autoIncrement:false" json:"id"`
	TenantID     int64  `gorm:"not null;default:0;index" json:"tenant_id"`
	UserID       int64  `gorm:"not null;index" json:"user_id"`
	CreatorID    int64  `gorm:"not null;index" json:"creator_id"`
	SystemPrompt string `gorm:"type:text;not null" json:"system_prompt"`
	ApiKey       string `gorm:"type:varchar(500);not null" json:"api_key"`
	Model        string `gorm:"type:varchar(100);not null" json:"model"`
	BaseURL      string `gorm:"type:varchar(500);default:''" json:"base_url"`
	IsSystem     bool   `gorm:"default:false;index" json:"is_system"`
	// Origin 标记 Bot 来源：0=用户自建 1=市场安装 2=OpenAPI 接入（2 不计入用量统计）
	Origin    int16     `gorm:"not null;default:0;index" json:"origin"`
	AppID     string    `gorm:"type:varchar(64);default:'';index" json:"app_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Bot) TableName() string {
	return "bot_table"
}
