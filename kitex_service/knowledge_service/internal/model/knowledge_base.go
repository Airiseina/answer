package model

import "time"

type KnowledgeBase struct {
	ID          int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	OwnerID     int64     `gorm:"not null;index" json:"owner_id"`
	Name        string    `gorm:"type:varchar(128);not null" json:"name"`
	Description string    `gorm:"type:varchar(512);default:''" json:"description"`
	DocCount    int32     `gorm:"default:0" json:"doc_count"`
	ChunkCount  int32     `gorm:"default:0" json:"chunk_count"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (KnowledgeBase) TableName() string {
	return "knowledge_base"
}
