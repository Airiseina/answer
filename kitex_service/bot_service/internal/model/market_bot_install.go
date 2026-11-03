package model

import "time"

// MarketBotInstall 记录「谁在哪个租户安装了哪个市场 Bot」。
//
// 安装会落一条真实 Bot 实例（bot_table，origin=market），
// 本表只做归属与版本溯源，安装者自带的 api_key 不在此表冗余存储。
type MarketBotInstall struct {
	ID          int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	TenantID    int64     `gorm:"not null;default:0;index" json:"tenant_id"`
	MarketBotID int64     `gorm:"not null;index" json:"market_bot_id"`
	BotID       int64     `gorm:"not null;index" json:"bot_id"`
	UserID      int64     `gorm:"not null;index" json:"user_id"`
	Version     int32     `gorm:"not null;default:1" json:"version"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 固定表名。
func (MarketBotInstall) TableName() string {
	return "market_bot_install"
}
