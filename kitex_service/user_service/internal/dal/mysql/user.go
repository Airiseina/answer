package mysql

import (
	"answer_pkg/logger"
	"errors"
	"user_service/internal/model"

	"go.uber.org/zap"
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
		logger.Error(" 存入数据失败", zap.Error(err))
		return err
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
		logger.Error("查询用户失败", zap.Error(err))
		return model.User{}, err
	}
	return user, nil
}
