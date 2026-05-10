package main

import (
	"context"
	"user_service/internal/service"
	"user_service/kitex_gen/user"

	"github.com/cloudwego/kitex/pkg/klog"
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
		klog.CtxErrorf(ctx, "处理用户[%s]注册请求时发生系统错误: %v", req.Account, err)
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
		klog.CtxErrorf(ctx, "处理用户[%s]登录请求时发生系统错误: %v", req.Account, err)
		return nil, err
	}
	if userInfo.Account != req.Account {
		return nil, nil
	}
	return &user.LoginRes{Account: userInfo.Account, Id: userInfo.Id}, nil
}

// CheckUsersExist implements the LoginServiceImpl interface.
func (s *LoginServiceImpl) CheckUsersExist(ctx context.Context, req *user.CheckUsersExistReq) (resp *user.CheckUsersExistRes, err error) {
	// TODO: Your code here...
	flag, err := s.userService.CheckUsersExist(req.UserIds)
	if err != nil {
		klog.CtxErrorf(ctx, "处理用户创建群聊请求时发生系统错误: %v", err)
		return nil, err
	}
	if !flag {
		resp = &user.CheckUsersExistRes{
			AllExist: false,
		}
		return resp, nil
	}
	resp = &user.CheckUsersExistRes{
		AllExist: true,
	}
	return resp, nil
}
