package mysql

import "gorm.io/gorm"

type gor struct {
	db *gorm.DB
}

type ServiceDao interface {
	CreateFile(sessionId uint, objectName string) (uint, error)
	UpdateFileStatus(sessionId uint, id uint, status int) error
}
