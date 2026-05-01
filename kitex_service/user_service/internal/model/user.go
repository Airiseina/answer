package model

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Account string `json:"account" gorm:"type:varchar(20);primary_key;not null"`
	Name    string `json:"name" gorm:"type:varchar(20);not null"`
	Hash    string `json:"hash" gorm:"not null"`
}
