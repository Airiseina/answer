package mysql

import (
	"github.com/Airiseina/answer/kitex_service/admin_service/internal/model"

	"gorm.io/gorm"
)

type gor struct {
	db *gorm.DB
}

func NewAdminDao(db *gorm.DB) AdminDao {
	return &gor{db}
}

type AdminDao interface {
	CreateAdmin(account, name, hash, role string) (int64, error)
	GetAdminByAccount(account string) (model.AdminUser, error)
	GetAdminById(id int64) (model.AdminUser, error)
	ListAdmins() ([]model.AdminUser, error)
	CountAdmins() (int64, error)
	UpdatePassword(id int64, hash string) error
	UpdateStatus(id int64, status int16) error
	UpdateLastLogin(id int64, ts int64) error
	UpdateNameAndRole(id int64, name, role string) error
}
