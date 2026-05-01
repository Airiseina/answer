package main

import (
	"context"
	"user_service/internal/service"
	"user_service/kitex_gen/user"
)

// LoginServiceImpl implements the last service interface defined in the IDL.
type LoginServiceImpl struct {
	userService *service.UserService
}

// Register implements the LoginServiceImpl interface.
func (s *LoginServiceImpl) Register(ctx context.Context, req *user.RegisterReq) (resp *user.RegisterRes, err error) {
	// TODO: Your code here...
	flag, err := s.userService.Register(req.Account, req.Name, req.Password)
	if err != nil {
		return nil, err
	}
	if !flag {
		return &user.RegisterRes{
			IsExit: true,
		}, nil
	}
	return &user.RegisterRes{
		IsExit: false,
	}, nil
}

// Login implements the LoginServiceImpl interface.
func (s *LoginServiceImpl) Login(ctx context.Context, req *user.LoginReq) (resp *user.LoginRes, err error) {
	// TODO: Your code here...
	userInfo, err := s.userService.Login(req.Account, req.Password)
	if err != nil {
		return nil, err
	}
	if userInfo.Account != req.Account {
		return nil, nil
	}
	return &user.LoginRes{Account: userInfo.Account, Id: int16(userInfo.Id)}, nil
}
