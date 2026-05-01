package mysql

import (
	"gorm.io/gorm"
)

type gor struct {
	db *gorm.DB
}

func NewChatDao(db *gorm.DB) ChatDao {
	return &gor{db: db}
}

type ChatDao interface {
}
