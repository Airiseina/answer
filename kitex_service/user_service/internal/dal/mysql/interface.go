package mysql

import (
	"user_service/internal/model"

	"gorm.io/gorm"
)

type gor struct {
	db *gorm.DB
}

func NewUserDao(db *gorm.DB) UserDao {
	return &gor{db}
}

type UserDao interface {
	Register(account, name, hash string) error
	GetUser(account string) (model.User, error)
	CountUsersByIds(userIds []int64) (int64, error)
}
