package model

import "time"

type BotKnowledge struct {
	ID        int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	BotID     int64     `gorm:"not null;uniqueIndex:idx_bot_kb" json:"bot_id"`
	KBID      int64     `gorm:"not null;uniqueIndex:idx_bot_kb;index" json:"kb_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (BotKnowledge) TableName() string {
	return "bot_knowledge"
}
