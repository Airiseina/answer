package service

import (
	"context"
	"strings"
	"time"

	"github.com/Airiseina/answer/kitex_service/admin_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/admin_service/internal/dal/mysql"
	"github.com/Airiseina/answer/kitex_service/admin_service/internal/model"
	"github.com/Airiseina/answer/pkg/audit"

	"golang.org/x/crypto/bcrypt"
)

// AdminDTO 是服务层对外返回的管理员视图。
type AdminDTO struct {
	Id          int64
	Account     string
	Name        string
	Role        string
	Status      int16
	LastLoginAt int64
}

type AdminService struct {
	dao   mysql.AdminDao
	audit audit.Recorder
}

func NewAdminService(dao mysql.AdminDao, recorder audit.Recorder) *AdminService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	return &AdminService{dao: dao, audit: recorder}
}

// validRole 校验角色取值，避免写入未知角色导致权限判断失效。
func validRole(role string) bool {
	switch role {
	case model.RoleSuper, model.RoleOps, model.RoleReviewer:
		return true
	default:
		return false
	}
}

// Login 校验账号口令，成功后刷新最后登录时间。
func (s *AdminService) Login(ctx context.Context, account, password string) (AdminDTO, error) {
	if account == "" || password == "" {
		return AdminDTO{}, nil
	}
	admin, err := s.dao.GetAdminByAccount(account)
	if err != nil {
		return AdminDTO{}, err
	}
	if admin.ID == 0 || admin.Status != model.StatusEnabled {
		return AdminDTO{}, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Hash), []byte(password)); err != nil {
		return AdminDTO{}, nil
	}
	now := time.Now().UnixMilli()
	if err := s.dao.UpdateLastLogin(admin.ID, now); err != nil {
		return AdminDTO{}, err
	}
	admin.LastLoginAt = now
	return toDTO(admin), nil
}

func (s *AdminService) ChangePassword(ctx context.Context, id int64, oldPassword, newPassword string) (bool, error) {
	if newPassword == "" {
		return false, nil
	}
	admin, err := s.dao.GetAdminById(id)
	if err != nil {
		return false, err
	}
	if admin.ID == 0 {
		return false, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Hash), []byte(oldPassword)); err != nil {
		return false, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}
	if err := s.dao.UpdatePassword(id, string(hash)); err != nil {
		return false, err
	}
	return true, nil
}

func (s *AdminService) Get(ctx context.Context, id int64) (AdminDTO, error) {
	admin, err := s.dao.GetAdminById(id)
	if err != nil {
		return AdminDTO{}, err
	}
	return toDTO(admin), nil
}

func (s *AdminService) List(ctx context.Context) ([]AdminDTO, error) {
	admins, err := s.dao.ListAdmins()
	if err != nil {
		return nil, err
	}
	list := make([]AdminDTO, 0, len(admins))
	for _, a := range admins {
		list = append(list, toDTO(a))
	}
	return list, nil
}

// Create 由超级管理员新建后台账号。
func (s *AdminService) Create(ctx context.Context, operatorID int64, account, name, role, password string) (bool, int64, error) {
	account = strings.TrimSpace(account)
	if account == "" || name == "" || password == "" || !validRole(role) {
		return false, 0, nil
	}
	operator, err := s.dao.GetAdminById(operatorID)
	if err != nil {
		return false, 0, err
	}
	if operator.ID == 0 || operator.Role != model.RoleSuper {
		return false, 0, nil
	}
	exist, err := s.dao.GetAdminByAccount(account)
	if err != nil {
		return false, 0, err
	}
	if exist.ID != 0 {
		return false, 0, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, 0, err
	}
	id, err := s.dao.CreateAdmin(account, name, string(hash), role)
	if err != nil {
		return false, 0, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		EventType:  audit.EventAdminManage,
		ActorType:  audit.ActorAdmin,
		ActorID:    operatorID,
		Action:     "create",
		TargetType: "admin",
		TargetID:   account,
		Detail:     "创建管理后台账号，角色=" + role,
		Result:     audit.ResultSuccess,
	})
	return true, id, nil
}

// UpdateStatus 启停一个后台账号；不允许停用自己。
func (s *AdminService) UpdateStatus(ctx context.Context, operatorID, id int64, status int16) (bool, error) {
	if id == operatorID {
		return false, nil
	}
	if status != model.StatusEnabled && status != model.StatusDisabled {
		return false, nil
	}
	operator, err := s.dao.GetAdminById(operatorID)
	if err != nil {
		return false, err
	}
	if operator.ID == 0 || operator.Role != model.RoleSuper {
		return false, nil
	}
	target, err := s.dao.GetAdminById(id)
	if err != nil {
		return false, err
	}
	if target.ID == 0 {
		return false, nil
	}
	if err := s.dao.UpdateStatus(id, status); err != nil {
		return false, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		EventType:  audit.EventAdminManage,
		ActorType:  audit.ActorAdmin,
		ActorID:    operatorID,
		Action:     "update_status",
		TargetType: "admin",
		TargetID:   target.Account,
		Detail:     "更新管理后台账号状态",
		Result:     audit.ResultSuccess,
	})
	return true, nil
}

// EnsureBootstrap 按配置幂等创建初始管理员。
//
// 已存在的账号会被跳过，仅在角色/名称发生变化时同步，避免每次重启覆盖口令。
func (s *AdminService) EnsureBootstrap(ctx context.Context, entries []config.AdminBootstrap) error {
	for _, e := range entries {
		account := strings.TrimSpace(e.Account)
		if account == "" || e.Password == "" {
			continue
		}
		role := e.Role
		if !validRole(role) {
			role = model.RoleSuper
		}
		name := e.Name
		if name == "" {
			name = account
		}
		exist, err := s.dao.GetAdminByAccount(account)
		if err != nil {
			return err
		}
		if exist.ID != 0 {
			if exist.Role != role || exist.Name != name {
				if err := s.dao.UpdateNameAndRole(exist.ID, name, role); err != nil {
					return err
				}
			}
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(e.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if _, err := s.dao.CreateAdmin(account, name, string(hash), role); err != nil {
			return err
		}
		_ = s.audit.Record(ctx, audit.Event{
			EventType:  audit.EventAdminManage,
			ActorType:  audit.ActorSystem,
			Action:     "bootstrap",
			TargetType: "admin",
			TargetID:   account,
			Detail:     "配置文件引导创建初始管理员，角色=" + role,
			Result:     audit.ResultSuccess,
		})
	}
	return nil
}

func toDTO(a model.AdminUser) AdminDTO {
	if a.ID == 0 {
		return AdminDTO{}
	}
	return AdminDTO{
		Id:          a.ID,
		Account:     a.Account,
		Name:        a.Name,
		Role:        a.Role,
		Status:      a.Status,
		LastLoginAt: a.LastLoginAt,
	}
}
