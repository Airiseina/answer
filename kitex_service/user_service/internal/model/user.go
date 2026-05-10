package model

import (
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        int64          `gorm:"primarykey"`
	CreatedAt int64          `gorm:"autoCreateTime"`
	UpdatedAt int64          `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type User struct {
	BaseModel
	Account string `json:"account" gorm:"type:varchar(20);uniqueIndex;not null"`
	Name    string `json:"name" gorm:"type:varchar(20);not null"`
	Hash    string `json:"hash" gorm:"not null"`
}
