package main

import (
	"context"
	"errors"

	"github.com/Airiseina/answer/kitex_service/user_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user"

	"github.com/cloudwego/kitex/pkg/klog"
)

// isTenantBizErr 判断是否为可预期的业务拒绝（无需按系统错误记录）。
func isTenantBizErr(err error) bool {
	return errors.Is(err, service.ErrNoPermission) ||
		errors.Is(err, service.ErrTenantNotFound) ||
		errors.Is(err, service.ErrAccountNotFound)
}

func toTenantInfo(dto service.TenantDTO) *user.TenantInfo {
	return &user.TenantInfo{
		Id:          dto.Id,
		Name:        dto.Name,
		Code:        dto.Code,
		LogoUrl:     dto.LogoURL,
		OwnerId:     dto.OwnerID,
		IsPersonal:  dto.IsPersonal,
		Role:        dto.Role,
		MemberCount: dto.MemberCount,
		Status:      dto.Status,
	}
}

func (s *LoginServiceImpl) CreateTenant(ctx context.Context, req *user.CreateTenantReq) (resp *user.CreateTenantRes, err error) {
	tenant, err := s.tenantService.CreateTenant(req.UserId, req.Name, req.LogoUrl)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]创建租户时发生系统错误: %v", req.UserId, err)
		return nil, err
	}
	if tenant.Id == 0 {
		return &user.CreateTenantRes{Success: false}, nil
	}
	return &user.CreateTenantRes{Success: true, Tenant: toTenantInfo(tenant)}, nil
}

func (s *LoginServiceImpl) ListTenants(ctx context.Context, req *user.ListTenantsReq) (resp *user.ListTenantsRes, err error) {
	tenants, err := s.tenantService.ListTenants(req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]获取租户列表时发生系统错误: %v", req.UserId, err)
		return nil, err
	}
	var list []*user.TenantInfo
	for _, t := range tenants {
		list = append(list, toTenantInfo(t))
	}
	return &user.ListTenantsRes{Tenants: list}, nil
}

func (s *LoginServiceImpl) GetTenant(ctx context.Context, req *user.GetTenantReq) (resp *user.GetTenantRes, err error) {
	tenant, err := s.tenantService.GetTenant(req.UserId, req.TenantId)
	if err != nil {
		if !isTenantBizErr(err) {
			klog.CtxErrorf(ctx, "用户[%d]获取租户[%d]信息时发生系统错误: %v", req.UserId, req.TenantId, err)
			return nil, err
		}
		return &user.GetTenantRes{}, nil
	}
	return &user.GetTenantRes{Tenant: toTenantInfo(tenant)}, nil
}

func (s *LoginServiceImpl) UpdateTenant(ctx context.Context, req *user.UpdateTenantReq) (resp *user.CommonRes, err error) {
	success, err := s.tenantService.UpdateTenant(req.UserId, req.TenantId, req.Name, req.LogoUrl)
	if err != nil {
		if !isTenantBizErr(err) {
			klog.CtxErrorf(ctx, "用户[%d]更新租户[%d]时发生系统错误: %v", req.UserId, req.TenantId, err)
			return nil, err
		}
		return &user.CommonRes{Success: false}, nil
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) ListTenantMembers(ctx context.Context, req *user.ListTenantMembersReq) (resp *user.ListTenantMembersRes, err error) {
	members, err := s.tenantService.ListMembers(req.OperatorId, req.TenantId)
	if err != nil {
		if !isTenantBizErr(err) {
			klog.CtxErrorf(ctx, "用户[%d]获取租户[%d]成员列表时发生系统错误: %v", req.OperatorId, req.TenantId, err)
			return nil, err
		}
		return &user.ListTenantMembersRes{}, nil
	}
	var list []*user.TenantMemberInfo
	for _, m := range members {
		list = append(list, &user.TenantMemberInfo{
			UserId:    m.UserID,
			Account:   m.Account,
			Name:      m.Name,
			AvatarUrl: m.AvatarURL,
			Role:      m.Role,
			DeptId:    m.DeptID,
			Status:    m.Status,
		})
	}
	return &user.ListTenantMembersRes{Members: list}, nil
}

func (s *LoginServiceImpl) AddTenantMember(ctx context.Context, req *user.AddTenantMemberReq) (resp *user.CommonRes, err error) {
	success, err := s.tenantService.AddMember(req.OperatorId, req.TenantId, req.Account, req.Role, req.DeptId)
	if err != nil {
		if !isTenantBizErr(err) {
			klog.CtxErrorf(ctx, "用户[%d]向租户[%d]添加成员[%s]时发生系统错误: %v", req.OperatorId, req.TenantId, req.Account, err)
			return nil, err
		}
		return &user.CommonRes{Success: false}, nil
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) SetTenantMemberRole(ctx context.Context, req *user.SetTenantMemberRoleReq) (resp *user.CommonRes, err error) {
	success, err := s.tenantService.SetMemberRole(req.OperatorId, req.TenantId, req.UserId, req.Role)
	if err != nil {
		if !isTenantBizErr(err) {
			klog.CtxErrorf(ctx, "用户[%d]修改租户[%d]成员[%d]角色时发生系统错误: %v", req.OperatorId, req.TenantId, req.UserId, err)
			return nil, err
		}
		return &user.CommonRes{Success: false}, nil
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) RemoveTenantMember(ctx context.Context, req *user.RemoveTenantMemberReq) (resp *user.CommonRes, err error) {
	success, err := s.tenantService.RemoveMember(req.OperatorId, req.TenantId, req.UserId)
	if err != nil {
		if !isTenantBizErr(err) {
			klog.CtxErrorf(ctx, "用户[%d]移除租户[%d]成员[%d]时发生系统错误: %v", req.OperatorId, req.TenantId, req.UserId, err)
			return nil, err
		}
		return &user.CommonRes{Success: false}, nil
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) ListDepartments(ctx context.Context, req *user.ListDepartmentsReq) (resp *user.ListDepartmentsRes, err error) {
	depts, err := s.tenantService.ListDepartments(req.OperatorId, req.TenantId)
	if err != nil {
		if !isTenantBizErr(err) {
			klog.CtxErrorf(ctx, "用户[%d]获取租户[%d]部门列表时发生系统错误: %v", req.OperatorId, req.TenantId, err)
			return nil, err
		}
		return &user.ListDepartmentsRes{}, nil
	}
	var list []*user.TenantDepartmentInfo
	for _, d := range depts {
		list = append(list, &user.TenantDepartmentInfo{
			Id:       d.Id,
			TenantId: d.TenantID,
			ParentId: d.ParentID,
			Name:     d.Name,
			Order:    d.Order,
		})
	}
	return &user.ListDepartmentsRes{Departments: list}, nil
}

func (s *LoginServiceImpl) CreateDepartment(ctx context.Context, req *user.CreateDepartmentReq) (resp *user.CreateDepartmentRes, err error) {
	deptID, err := s.tenantService.CreateDepartment(req.OperatorId, req.TenantId, req.ParentId, req.Name)
	if err != nil {
		if !isTenantBizErr(err) {
			klog.CtxErrorf(ctx, "用户[%d]在租户[%d]创建部门时发生系统错误: %v", req.OperatorId, req.TenantId, err)
			return nil, err
		}
		return &user.CreateDepartmentRes{Success: false}, nil
	}
	if deptID == 0 {
		return &user.CreateDepartmentRes{Success: false}, nil
	}
	return &user.CreateDepartmentRes{Success: true, DeptId: deptID}, nil
}

func (s *LoginServiceImpl) RenameDepartment(ctx context.Context, req *user.RenameDepartmentReq) (resp *user.CommonRes, err error) {
	success, err := s.tenantService.RenameDepartment(req.OperatorId, req.TenantId, req.DeptId, req.Name)
	if err != nil {
		if !isTenantBizErr(err) {
			klog.CtxErrorf(ctx, "用户[%d]重命名租户[%d]部门[%d]时发生系统错误: %v", req.OperatorId, req.TenantId, req.DeptId, err)
			return nil, err
		}
		return &user.CommonRes{Success: false}, nil
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) MoveDepartment(ctx context.Context, req *user.MoveDepartmentReq) (resp *user.CommonRes, err error) {
	success, err := s.tenantService.MoveDepartment(req.OperatorId, req.TenantId, req.DeptId, req.ParentId)
	if err != nil {
		if !isTenantBizErr(err) {
			klog.CtxErrorf(ctx, "用户[%d]移动租户[%d]部门[%d]时发生系统错误: %v", req.OperatorId, req.TenantId, req.DeptId, err)
			return nil, err
		}
		return &user.CommonRes{Success: false}, nil
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) RemoveDepartment(ctx context.Context, req *user.RemoveDepartmentReq) (resp *user.CommonRes, err error) {
	success, err := s.tenantService.RemoveDepartment(req.OperatorId, req.TenantId, req.DeptId)
	if err != nil {
		if !isTenantBizErr(err) {
			klog.CtxErrorf(ctx, "用户[%d]删除租户[%d]部门[%d]时发生系统错误: %v", req.OperatorId, req.TenantId, req.DeptId, err)
			return nil, err
		}
		return &user.CommonRes{Success: false}, nil
	}
	return &user.CommonRes{Success: success}, nil
}

func (s *LoginServiceImpl) SetMemberDepartment(ctx context.Context, req *user.SetMemberDepartmentReq) (resp *user.CommonRes, err error) {
	success, err := s.tenantService.SetMemberDepartment(req.OperatorId, req.TenantId, req.UserId, req.DeptId)
	if err != nil {
		if !isTenantBizErr(err) {
			klog.CtxErrorf(ctx, "用户[%d]调整租户[%d]成员[%d]部门时发生系统错误: %v", req.OperatorId, req.TenantId, req.UserId, err)
			return nil, err
		}
		return &user.CommonRes{Success: false}, nil
	}
	return &user.CommonRes{Success: success}, nil
}
