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
	Files       []File `json:"files" gorm:"foreignkey:SessionID"`
}

type File struct {
	gorm.Model
	ObjectName string `json:"objectName" gorm:"type:varchar(30);not null"`
	SessionID  uint   `json:"sessionID" gorm:"index;not null"`
	Status     int    `json:"status"` //0排队中；1处理中；2处理成功；3处理失败
}
