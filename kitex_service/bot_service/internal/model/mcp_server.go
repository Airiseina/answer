package model

import "time"

type McpServer struct {
	ID          int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	BotID       int64     `gorm:"not null;index" json:"bot_id"`
	Name        string    `gorm:"type:varchar(64);not null" json:"name"`
	Description string    `gorm:"type:varchar(256);default:''" json:"description"`
	Transport   string    `gorm:"type:varchar(16);not null;default:'sse'" json:"transport"`
	URL         string    `gorm:"type:varchar(500);not null" json:"url"`
	AuthType    string    `gorm:"type:varchar(16);default:'none'" json:"auth_type"`
	AuthToken   string    `gorm:"type:varchar(500);default:''" json:"auth_token"`
	Enabled     bool      `gorm:"default:true;index" json:"enabled"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (McpServer) TableName() string {
	return "bot_mcp_server"
}
