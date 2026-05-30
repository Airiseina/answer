package model

import "time"

const (
	DocStatusPending = "pending"
	DocStatusParsing = "parsing"
	DocStatusParsed  = "parsed"
	DocStatusFailed  = "failed"
)

type KbDocument struct {
	ID           int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	KBID         int64     `gorm:"not null;index" json:"kb_id"`
	FileName     string    `gorm:"type:varchar(256);not null" json:"file_name"`
	FileURL      string    `gorm:"type:varchar(500);not null" json:"file_url"`
	FileType     string    `gorm:"type:varchar(16);not null" json:"file_type"`
	FileSize     int64     `gorm:"default:0" json:"file_size"`
	Status       string    `gorm:"type:varchar(16);not null;default:'pending';index" json:"status"`
	ChunkCount   int32     `gorm:"default:0" json:"chunk_count"`
	ErrorMessage string    `gorm:"type:varchar(512);default:''" json:"error_message"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (KbDocument) TableName() string {
	return "kb_document"
}
