package mysql

import (
	"errors"
	"fmt"
	"user_service/internal/model"

	"gorm.io/gorm"
)

func (db *gor) Register(account, name, hash string) error {
	userInfo := model.User{
		Account: account,
		Name:    name,
		Hash:    hash,
	}
	err := db.db.Create(&userInfo).Error
	if err != nil {
		return fmt.Errorf("存入数据失败: %w", err)
	}
	return nil
}

func (db *gor) GetUser(account string) (model.User, error) {
	var user model.User
	err := db.db.Where("account = ?", account).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, nil
		}
		return model.User{}, fmt.Errorf("查询用户失败: %w", err)
	}
	return user, nil
}

func (db *gor) CountUsersByIds(userIds []int64) (int64, error) {
	var count int64
	err := db.db.Model(&model.User{}).Where("id IN ?", userIds).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("查询用户失败: %w", err)
	}
	return count, nil
}
