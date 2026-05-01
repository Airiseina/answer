package model

import "gorm.io/gorm"

type Chat struct {
	gorm.Model
	SessionID uint   `json:"sessionID" gorm:"index;not null"`
	AiMessage string `json:"ai_message" gorm:"type:text;not null"`
	Question  string `json:"question" gorm:"type:text;not null"`
}

type Session struct {
	gorm.Model
	UserID      uint   `json:"userId" gorm:"index;not null"`
	Title       string `json:"title" gorm:"type:varchar(20);not null"`
	RoleSetting string `json:"roleSetting" gorm:"type:text;not null"`
	Chats       []Chat `json:"chats" gorm:"foreignkey:SessionID"`
}
