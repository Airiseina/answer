package model

import "time"

type Bot struct {
	ID           int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	UserID       int64     `gorm:"not null;index" json:"user_id"`
	CreatorID    int64     `gorm:"not null;index" json:"creator_id"`
	Name         string    `gorm:"type:varchar(50);not null" json:"name"`
	AvatarURL    string    `gorm:"type:varchar(500);default:''" json:"avatar_url"`
	SystemPrompt string    `gorm:"type:text;not null" json:"system_prompt"`
	ApiKey       string    `gorm:"type:varchar(500);not null" json:"api_key"`
	Model        string    `gorm:"type:varchar(100);not null" json:"model"`
	BaseURL      string    `gorm:"type:varchar(500);default:''" json:"base_url"`
	IsSystem     bool      `gorm:"default:false;index" json:"is_system"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Bot) TableName() string {
	return "bot_table"
}
