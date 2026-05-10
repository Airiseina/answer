package model

import "gorm.io/gorm"

type BaseModel struct {
	ID        int64          `gorm:"primarykey"`
	CreatedAt int64          `gorm:"autoCreateTime"`
	UpdatedAt int64          `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type Chat struct {
	BaseModel
	SessionID int64  `json:"sessionID" gorm:"index;not null"`
	AiMessage string `json:"ai_message" gorm:"type:text;not null"`
	Question  string `json:"question" gorm:"type:text;not null"`
}

type Session struct {
	BaseModel
	UserID      int64  `json:"userId" gorm:"index;not null"`
	Title       string `json:"title" gorm:"type:varchar(20);not null"`
	RoleSetting string `json:"roleSetting" gorm:"type:text;not null"`
	Chats       []Chat `json:"chats" gorm:"foreignkey:SessionID"`
}
