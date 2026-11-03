package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Airiseina/answer/kitex_service/user_service/internal/dal/mysql"
	"github.com/Airiseina/answer/kitex_service/user_service/internal/model"
)

// 租户业务的哨兵错误，供 handler 区分「系统错误」与「业务拒绝」。
var (
	ErrTenantNotFound  = errors.New("租户不存在")
	ErrAccountNotFound = errors.New("账号不存在")
	ErrNoPermission    = errors.New("无权限")
)

type TenantService struct {
	userDao   mysql.UserDao
	tenantDao mysql.TenantDao
}

func NewTenantService(userDao mysql.UserDao, tenantDao mysql.TenantDao) *TenantService {
	return &TenantService{userDao, tenantDao}
}

type TenantDTO struct {
	Id          int64
	Name        string
	Code        string
	LogoURL     string
	OwnerID     int64
	IsPersonal  bool
	Role        int16
	MemberCount int64
	Status      int16
}

type TenantMemberDTO struct {
	UserID    int64
	Account   string
	Name      string
	AvatarURL string
	Role      int16
	DeptID    int64
	Status    int16
}

type DepartmentDTO struct {
	Id       int64
	TenantID int64
	ParentID int64
	Name     string
	Order    int32
}

// EnsurePlatformTenant 幂等创建平台租户，供系统 Bot 与全局资源归属使用。
func (s *TenantService) EnsurePlatformTenant() error {
	return s.tenantDao.EnsurePlatformTenant("平台")
}

func (s *TenantService) CreateTenant(userID int64, name, logoURL string) (TenantDTO, error) {
	if userID == 0 || name == "" {
		return TenantDTO{}, nil
	}
	tenant, err := s.tenantDao.CreateTenantWithOwner(name, newTenantCode(), logoURL, userID)
	if err != nil {
		return TenantDTO{}, err
	}
	return TenantDTO{
		Id:          tenant.ID,
		Name:        tenant.Name,
		Code:        tenant.Code,
		LogoURL:     tenant.LogoURL,
		OwnerID:     tenant.OwnerID,
		IsPersonal:  tenant.IsPersonal,
		Role:        model.TenantRoleOwner,
		MemberCount: 1,
		Status:      tenant.Status,
	}, nil
}

func (s *TenantService) ListTenants(userID int64) ([]TenantDTO, error) {
	if userID == 0 {
		return nil, nil
	}
	rows, err := s.tenantDao.ListTenantsByUser(userID)
	if err != nil {
		return nil, err
	}
	var result []TenantDTO
	for _, t := range rows {
		result = append(result, TenantDTO{
			Id:         t.ID,
			Name:       t.Name,
			Code:       t.Code,
			LogoURL:    t.LogoURL,
			OwnerID:    t.OwnerID,
			IsPersonal: t.IsPersonal,
			Role:       t.Role,
			Status:     t.Status,
		})
	}
	return result, nil
}

// GetTenant 返回租户详情；调用者必须是该租户成员。
func (s *TenantService) GetTenant(userID, tenantID int64) (TenantDTO, error) {
	member, err := s.tenantDao.GetTenantMember(tenantID, userID)
	if err != nil {
		return TenantDTO{}, err
	}
	if member.TenantID == 0 || member.Status == model.TenantMemberRemoved {
		return TenantDTO{}, ErrNoPermission
	}
	tenant, err := s.tenantDao.GetTenant(tenantID)
	if err != nil {
		return TenantDTO{}, err
	}
	if tenant.ID == 0 {
		return TenantDTO{}, ErrTenantNotFound
	}
	count, err := s.tenantDao.CountTenantMembers(tenantID)
	if err != nil {
		return TenantDTO{}, err
	}
	return TenantDTO{
		Id:          tenant.ID,
		Name:        tenant.Name,
		Code:        tenant.Code,
		LogoURL:     tenant.LogoURL,
		OwnerID:     tenant.OwnerID,
		IsPersonal:  tenant.IsPersonal,
		Role:        member.Role,
		MemberCount: count,
		Status:      tenant.Status,
	}, nil
}

// UpdateTenant 更新租户基本信息，需要 admin 及以上角色。
func (s *TenantService) UpdateTenant(userID, tenantID int64, name, logoURL string) (bool, error) {
	if name == "" && logoURL == "" {
		return false, nil
	}
	if _, err := s.requireRole(tenantID, userID, model.TenantRoleAdmin); err != nil {
		return false, err
	}
	if err := s.tenantDao.UpdateTenant(tenantID, name, logoURL); err != nil {
		return false, err
	}
	return true, nil
}

func (s *TenantService) ListMembers(operatorID, tenantID int64) ([]TenantMemberDTO, error) {
	if _, err := s.requireRole(tenantID, operatorID, model.TenantRoleAdmin); err != nil {
		return nil, err
	}
	members, err := s.tenantDao.ListTenantMembers(tenantID)
	if err != nil {
		return nil, err
	}
	var userIDs []int64
	for _, m := range members {
		userIDs = append(userIDs, m.UserID)
	}
	users, err := s.userDao.GetUsersByIds(userIDs)
	if err != nil {
		return nil, err
	}
	infoMap := make(map[int64]model.User, len(users))
	for _, u := range users {
		infoMap[u.ID] = u
	}
	var result []TenantMemberDTO
	for _, m := range members {
		u := infoMap[m.UserID]
		result = append(result, TenantMemberDTO{
			UserID:    m.UserID,
			Account:   u.Account,
			Name:      u.Name,
			AvatarURL: u.AvatarURL,
			Role:      m.Role,
			DeptID:    m.DeptID,
			Status:    m.Status,
		})
	}
	return result, nil
}

// AddMember 由管理员按账号直接添加成员（本轮不做邀请码）。
func (s *TenantService) AddMember(operatorID, tenantID int64, account string, role int16, deptID int64) (bool, error) {
	operator, err := s.requireRole(tenantID, operatorID, model.TenantRoleAdmin)
	if err != nil {
		return false, err
	}
	if role < model.TenantRoleMember || role >= model.TenantRoleOwner {
		return false, nil
	}
	if operator.Role < model.TenantRoleOwner && role >= model.TenantRoleAdmin {
		return false, nil
	}
	target, err := s.userDao.GetUser(account)
	if err != nil {
		return false, err
	}
	if target.ID == 0 || target.IsBot {
		return false, nil
	}
	if deptID != 0 {
		dept, err := s.tenantDao.GetDepartment(deptID)
		if err != nil {
			return false, err
		}
		if dept.ID == 0 || dept.TenantID != tenantID {
			return false, nil
		}
	}
	if err = s.tenantDao.UpsertTenantMember(tenantID, target.ID, role, deptID); err != nil {
		return false, err
	}
	return true, nil
}

func (s *TenantService) SetMemberRole(operatorID, tenantID, targetUserID int64, role int16) (bool, error) {
	if role < model.TenantRoleMember || role >= model.TenantRoleOwner {
		return false, nil
	}
	operator, err := s.requireRole(tenantID, operatorID, model.TenantRoleAdmin)
	if err != nil {
		return false, err
	}
	if operator.Role < model.TenantRoleOwner {
		// 仅 owner 可以授予/收回 admin。
		return false, nil
	}
	if targetUserID == operatorID {
		return false, nil
	}
	target, err := s.tenantDao.GetTenantMember(tenantID, targetUserID)
	if err != nil {
		return false, err
	}
	if target.TenantID == 0 || target.Status == model.TenantMemberRemoved {
		return false, nil
	}
	if err = s.tenantDao.UpdateTenantMemberRole(tenantID, targetUserID, role); err != nil {
		return false, err
	}
	return true, nil
}

func (s *TenantService) RemoveMember(operatorID, tenantID, targetUserID int64) (bool, error) {
	operator, err := s.requireRole(tenantID, operatorID, model.TenantRoleAdmin)
	if err != nil {
		return false, err
	}
	if targetUserID == operatorID {
		return false, nil
	}
	target, err := s.tenantDao.GetTenantMember(tenantID, targetUserID)
	if err != nil {
		return false, err
	}
	if target.TenantID == 0 || target.Status == model.TenantMemberRemoved {
		return false, nil
	}
	if target.Role == model.TenantRoleOwner {
		return false, nil
	}
	if target.Role == model.TenantRoleAdmin && operator.Role < model.TenantRoleOwner {
		return false, nil
	}
	if err = s.tenantDao.RemoveTenantMember(tenantID, targetUserID); err != nil {
		return false, err
	}
	return true, nil
}

func (s *TenantService) ListDepartments(operatorID, tenantID int64) ([]DepartmentDTO, error) {
	if _, err := s.requireRole(tenantID, operatorID, model.TenantRoleMember); err != nil {
		return nil, err
	}
	depts, err := s.tenantDao.ListDepartments(tenantID)
	if err != nil {
		return nil, err
	}
	var result []DepartmentDTO
	for _, d := range depts {
		result = append(result, DepartmentDTO{
			Id:       d.ID,
			TenantID: d.TenantID,
			ParentID: d.ParentID,
			Name:     d.Name,
			Order:    d.Order,
		})
	}
	return result, nil
}

func (s *TenantService) CreateDepartment(operatorID, tenantID, parentID int64, name string) (int64, error) {
	if name == "" {
		return 0, nil
	}
	if _, err := s.requireRole(tenantID, operatorID, model.TenantRoleAdmin); err != nil {
		return 0, err
	}
	if parentID != 0 {
		parent, err := s.tenantDao.GetDepartment(parentID)
		if err != nil {
			return 0, err
		}
		if parent.ID == 0 || parent.TenantID != tenantID {
			return 0, nil
		}
	}
	dept, err := s.tenantDao.CreateDepartment(model.TenantDepartment{
		TenantID: tenantID,
		ParentID: parentID,
		Name:     name,
	})
	if err != nil {
		return 0, err
	}
	return dept.ID, nil
}

func (s *TenantService) RenameDepartment(operatorID, tenantID, deptID int64, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	if _, err := s.requireRole(tenantID, operatorID, model.TenantRoleAdmin); err != nil {
		return false, err
	}
	dept, err := s.tenantDao.GetDepartment(deptID)
	if err != nil {
		return false, err
	}
	if dept.ID == 0 || dept.TenantID != tenantID {
		return false, nil
	}
	if err = s.tenantDao.UpdateDepartmentName(deptID, name); err != nil {
		return false, err
	}
	return true, nil
}

func (s *TenantService) MoveDepartment(operatorID, tenantID, deptID, parentID int64) (bool, error) {
	if _, err := s.requireRole(tenantID, operatorID, model.TenantRoleAdmin); err != nil {
		return false, err
	}
	dept, err := s.tenantDao.GetDepartment(deptID)
	if err != nil {
		return false, err
	}
	if dept.ID == 0 || dept.TenantID != tenantID {
		return false, nil
	}
	if parentID != 0 {
		if parentID == deptID {
			return false, nil
		}
		parent, err := s.tenantDao.GetDepartment(parentID)
		if err != nil {
			return false, err
		}
		if parent.ID == 0 || parent.TenantID != tenantID {
			return false, nil
		}
		if cycle, err := s.isDescendant(tenantID, parentID, deptID); err != nil {
			return false, err
		} else if cycle {
			return false, nil
		}
	}
	if err = s.tenantDao.UpdateDepartmentParent(deptID, parentID); err != nil {
		return false, err
	}
	return true, nil
}

func (s *TenantService) RemoveDepartment(operatorID, tenantID, deptID int64) (bool, error) {
	if _, err := s.requireRole(tenantID, operatorID, model.TenantRoleAdmin); err != nil {
		return false, err
	}
	dept, err := s.tenantDao.GetDepartment(deptID)
	if err != nil {
		return false, err
	}
	if dept.ID == 0 || dept.TenantID != tenantID {
		return false, nil
	}
	if err = s.tenantDao.RemoveDepartment(dept); err != nil {
		return false, err
	}
	return true, nil
}

func (s *TenantService) SetMemberDepartment(operatorID, tenantID, userID, deptID int64) (bool, error) {
	if _, err := s.requireRole(tenantID, operatorID, model.TenantRoleAdmin); err != nil {
		return false, err
	}
	member, err := s.tenantDao.GetTenantMember(tenantID, userID)
	if err != nil {
		return false, err
	}
	if member.TenantID == 0 || member.Status == model.TenantMemberRemoved {
		return false, nil
	}
	if deptID != 0 {
		dept, err := s.tenantDao.GetDepartment(deptID)
		if err != nil {
			return false, err
		}
		if dept.ID == 0 || dept.TenantID != tenantID {
			return false, nil
		}
	}
	if err = s.tenantDao.UpdateTenantMemberDept(tenantID, userID, deptID); err != nil {
		return false, err
	}
	return true, nil
}

// requireRole 校验操作者在该租户内的角色，返回成员记录。
func (s *TenantService) requireRole(tenantID, userID int64, min int16) (model.TenantMember, error) {
	member, err := s.tenantDao.GetTenantMember(tenantID, userID)
	if err != nil {
		return model.TenantMember{}, err
	}
	if member.TenantID == 0 || member.Status == model.TenantMemberRemoved || member.Role < min {
		return model.TenantMember{}, ErrNoPermission
	}
	return member, nil
}

// isDescendant 判断 candidate 是否位于 root 的子树中，用于阻止部门成环。
func (s *TenantService) isDescendant(tenantID, candidate, root int64) (bool, error) {
	visited := 0
	for candidate != 0 {
		if candidate == root {
			return true, nil
		}
		dept, err := s.tenantDao.GetDepartment(candidate)
		if err != nil {
			return false, err
		}
		if dept.ID == 0 || dept.TenantID != tenantID {
			return false, nil
		}
		candidate = dept.ParentID
		visited++
		if visited > 256 {
			return false, nil
		}
	}
	return false, nil
}

// newTenantCode 生成组织租户的展示用标识（本轮仅展示，入群走管理员添加）。
func newTenantCode() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("t_%d", time.Now().UnixNano())
	}
	return "t_" + hex.EncodeToString(buf)
}
