package service

import (
	"user_service/internal/dal/mysql"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	dao mysql.UserDao
}

func NewUserService(dao mysql.UserDao) *UserService {
	return &UserService{dao}
}

func (dao *UserService) Register(account, name, password string) (bool, error) {
	if account == "" || name == "" || password == "" {
		return false, nil
	}
	userInfo, err := dao.dao.GetUser(account)
	if err != nil {
		return false, err
	}
	if userInfo.Account != "" {
		return false, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, nil
	}
	return true, dao.dao.Register(account, name, string(hash))
}

type UserDTO struct {
	Id      int64  `json:"id"`
	Account string `json:"account"`
}

func (dao *UserService) Login(account string, password string) (UserDTO, error) {
	userInfo, err := dao.dao.GetUser(account)
	if err != nil {
		return UserDTO{}, err
	}
	if userInfo.Account == "" {
		return UserDTO{}, nil
	}
	err = bcrypt.CompareHashAndPassword([]byte(userInfo.Hash), []byte(password))
	if err != nil {
		return UserDTO{}, nil
	}
	return UserDTO{
		Id:      userInfo.ID,
		Account: userInfo.Account,
	}, nil
}
