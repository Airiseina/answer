package model

import "time"

// MarketBotRating 是用户对市场 Bot 的评分，每个用户对同一 Bot 只保留一条（重复评分即为改分）。
type MarketBotRating struct {
	ID          int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	TenantID    int64     `gorm:"not null;default:0;index" json:"tenant_id"`
	MarketBotID int64     `gorm:"not null;uniqueIndex:uk_market_rating_user" json:"market_bot_id"`
	UserID      int64     `gorm:"not null;uniqueIndex:uk_market_rating_user" json:"user_id"`
	Score       int16     `gorm:"not null" json:"score"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 固定表名。
func (MarketBotRating) TableName() string {
	return "market_bot_rating"
}
