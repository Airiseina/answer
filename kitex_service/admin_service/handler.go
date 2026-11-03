package main

import (
	"context"

	"github.com/Airiseina/answer/kitex_service/admin_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/admin_service/kitex_gen/admin"

	"github.com/cloudwego/kitex/pkg/klog"
)

type AdminServiceImpl struct {
	adminService *service.AdminService
}

func (s *AdminServiceImpl) Login(ctx context.Context, req *admin.AdminLoginReq) (resp *admin.AdminLoginRes, err error) {
	dto, err := s.adminService.Login(ctx, req.Account, req.Password)
	if err != nil {
		klog.CtxErrorf(ctx, "管理员[%s]登录失败: %v", req.Account, err)
		return nil, err
	}
	if dto.Id == 0 {
		return &admin.AdminLoginRes{}, nil
	}
	return &admin.AdminLoginRes{
		Id:      dto.Id,
		Account: dto.Account,
		Name:    dto.Name,
		Role:    dto.Role,
	}, nil
}

func (s *AdminServiceImpl) ChangePassword(ctx context.Context, req *admin.ChangePasswordReq) (resp *admin.CommonRes, err error) {
	ok, err := s.adminService.ChangePassword(ctx, req.AdminId, req.OldPassword, req.UpdatePassword)
	if err != nil {
		klog.CtxErrorf(ctx, "管理员[%d]修改密码失败: %v", req.AdminId, err)
		return nil, err
	}
	return &admin.CommonRes{Success: ok}, nil
}

func (s *AdminServiceImpl) GetAdmin(ctx context.Context, req *admin.GetAdminReq) (resp *admin.GetAdminRes, err error) {
	dto, err := s.adminService.Get(ctx, req.AdminId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询管理员[%d]失败: %v", req.AdminId, err)
		return nil, err
	}
	if dto.Id == 0 {
		return &admin.GetAdminRes{}, nil
	}
	return &admin.GetAdminRes{Admin: toAdminInfo(dto)}, nil
}

func (s *AdminServiceImpl) ListAdmins(ctx context.Context, req *admin.ListAdminsReq) (resp *admin.ListAdminsRes, err error) {
	dtos, err := s.adminService.List(ctx)
	if err != nil {
		klog.CtxErrorf(ctx, "查询管理员列表失败: %v", err)
		return nil, err
	}
	list := make([]*admin.AdminInfo, 0, len(dtos))
	for _, dto := range dtos {
		list = append(list, toAdminInfo(dto))
	}
	return &admin.ListAdminsRes{Admins: list}, nil
}

func (s *AdminServiceImpl) CreateAdmin(ctx context.Context, req *admin.CreateAdminReq) (resp *admin.CreateAdminRes, err error) {
	ok, id, err := s.adminService.Create(ctx, req.OperatorId, req.Account, req.Name, req.Role, req.Password)
	if err != nil {
		klog.CtxErrorf(ctx, "创建管理员[%s]失败: %v", req.Account, err)
		return nil, err
	}
	return &admin.CreateAdminRes{Success: ok, AdminId: id}, nil
}

func (s *AdminServiceImpl) UpdateAdminStatus(ctx context.Context, req *admin.UpdateAdminStatusReq) (resp *admin.CommonRes, err error) {
	ok, err := s.adminService.UpdateStatus(ctx, req.OperatorId, req.AdminId, req.Status)
	if err != nil {
		klog.CtxErrorf(ctx, "更新管理员[%d]状态失败: %v", req.AdminId, err)
		return nil, err
	}
	return &admin.CommonRes{Success: ok}, nil
}

func toAdminInfo(dto service.AdminDTO) *admin.AdminInfo {
	return &admin.AdminInfo{
		Id:          dto.Id,
		Account:     dto.Account,
		Name:        dto.Name,
		Role:        dto.Role,
		Status:      dto.Status,
		LastLoginAt: dto.LastLoginAt,
	}
}
