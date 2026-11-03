package mysql

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/user_service/internal/model"

	"gorm.io/gorm"
)

// MemberTenant 是「用户可见的租户」视图，附带该用户在此租户内的角色。
type MemberTenant struct {
	model.Tenant
	Role int16 `json:"role"`
}

func NewTenantDao(db *gorm.DB) TenantDao {
	return &gor{db}
}

type TenantDao interface {
	CreateUserWithPersonalTenant(account, name, hash string) (model.User, model.Tenant, error)
	EnsurePlatformTenant(name string) error
	CreateTenantWithOwner(name, code, logoURL string, ownerID int64) (model.Tenant, error)
	GetTenant(tenantID int64) (model.Tenant, error)
	UpdateTenant(tenantID int64, name, logoURL string) error
	ListTenantsByUser(userID int64) ([]MemberTenant, error)
	CountTenantMembers(tenantID int64) (int64, error)
	GetTenantMember(tenantID, userID int64) (model.TenantMember, error)
	ListTenantMembers(tenantID int64) ([]model.TenantMember, error)
	UpsertTenantMember(tenantID, userID int64, role int16, deptID int64) error
	UpdateTenantMemberRole(tenantID, userID int64, role int16) error
	UpdateTenantMemberDept(tenantID, userID int64, deptID int64) error
	RemoveTenantMember(tenantID, userID int64) error
	CreateDepartment(dept model.TenantDepartment) (model.TenantDepartment, error)
	GetDepartment(deptID int64) (model.TenantDepartment, error)
	ListDepartments(tenantID int64) ([]model.TenantDepartment, error)
	UpdateDepartmentName(deptID int64, name string) error
	UpdateDepartmentParent(deptID, parentID int64) error
	RemoveDepartment(dept model.TenantDepartment) error
}

func (db *gor) CreateUserWithPersonalTenant(account, name, hash string) (model.User, model.Tenant, error) {
	var user model.User
	var tenant model.Tenant
	err := db.db.Transaction(func(tx *gorm.DB) error {
		user = model.User{Account: account, Name: name, Hash: hash}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		tenant = model.Tenant{
			Name:       name,
			Code:       fmt.Sprintf("p_%d", user.ID),
			OwnerID:    user.ID,
			IsPersonal: true,
			Status:     model.TenantStatusNormal,
			Plan:       "free",
		}
		if err := tx.Create(&tenant).Error; err != nil {
			return err
		}
		member := model.TenantMember{
			TenantID: tenant.ID,
			UserID:   user.ID,
			Role:     model.TenantRoleOwner,
			Status:   model.TenantMemberNormal,
		}
		return tx.Create(&member).Error
	})
	if err != nil {
		return model.User{}, model.Tenant{}, fmt.Errorf("注册并创建个人租户失败: %w", err)
	}
	return user, tenant, nil
}

func (db *gor) EnsurePlatformTenant(name string) error {
	var exist model.Tenant
	err := db.db.Unscoped().Where("id = ?", model.PlatformTenantID).First(&exist).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("查询平台租户失败: %w", err)
	}
	tenant := model.Tenant{
		BaseModel:  model.BaseModel{ID: model.PlatformTenantID},
		Name:       name,
		Code:       "platform",
		OwnerID:    0,
		IsPersonal: false,
		Status:     model.TenantStatusNormal,
		Plan:       "system",
	}
	if err = db.db.Create(&tenant).Error; err != nil {
		return fmt.Errorf("创建平台租户失败: %w", err)
	}
	return nil
}

func (db *gor) CreateTenantWithOwner(name, code, logoURL string, ownerID int64) (model.Tenant, error) {
	var tenant model.Tenant
	err := db.db.Transaction(func(tx *gorm.DB) error {
		tenant = model.Tenant{
			Name:       name,
			Code:       code,
			LogoURL:    logoURL,
			OwnerID:    ownerID,
			IsPersonal: false,
			Status:     model.TenantStatusNormal,
			Plan:       "free",
		}
		if err := tx.Create(&tenant).Error; err != nil {
			return err
		}
		member := model.TenantMember{
			TenantID: tenant.ID,
			UserID:   ownerID,
			Role:     model.TenantRoleOwner,
			Status:   model.TenantMemberNormal,
		}
		return tx.Create(&member).Error
	})
	if err != nil {
		return model.Tenant{}, fmt.Errorf("创建租户失败: %w", err)
	}
	return tenant, nil
}

func (db *gor) GetTenant(tenantID int64) (model.Tenant, error) {
	var tenant model.Tenant
	err := db.db.Where("id = ?", tenantID).First(&tenant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Tenant{}, nil
		}
		return model.Tenant{}, fmt.Errorf("查询租户失败: %w", err)
	}
	return tenant, nil
}

func (db *gor) UpdateTenant(tenantID int64, name, logoURL string) error {
	updates := map[string]interface{}{}
	if name != "" {
		updates["name"] = name
	}
	if logoURL != "" {
		updates["logo_url"] = logoURL
	}
	if len(updates) == 0 {
		return nil
	}
	err := db.db.Model(&model.Tenant{}).Where("id = ?", tenantID).Updates(updates).Error
	if err != nil {
		return fmt.Errorf("更新租户失败: %w", err)
	}
	return nil
}

func (db *gor) ListTenantsByUser(userID int64) ([]MemberTenant, error) {
	var tenants []MemberTenant
	err := db.db.Table("tenant").
		Select("tenant.*, tenant_member.role AS role").
		Joins("JOIN tenant_member ON tenant_member.tenant_id = tenant.id").
		Where("tenant_member.user_id = ? AND tenant_member.status <> ? AND tenant.deleted_at IS NULL",
			userID, model.TenantMemberRemoved).
		Order("tenant.is_personal DESC, tenant.id ASC").
		Scan(&tenants).Error
	if err != nil {
		return nil, fmt.Errorf("查询用户租户列表失败: %w", err)
	}
	return tenants, nil
}

func (db *gor) CountTenantMembers(tenantID int64) (int64, error) {
	var count int64
	err := db.db.Model(&model.TenantMember{}).
		Where("tenant_id = ? AND status <> ?", tenantID, model.TenantMemberRemoved).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("统计租户成员失败: %w", err)
	}
	return count, nil
}

func (db *gor) GetTenantMember(tenantID, userID int64) (model.TenantMember, error) {
	var member model.TenantMember
	err := db.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.TenantMember{}, nil
		}
		return model.TenantMember{}, fmt.Errorf("查询租户成员失败: %w", err)
	}
	return member, nil
}

func (db *gor) ListTenantMembers(tenantID int64) ([]model.TenantMember, error) {
	var members []model.TenantMember
	err := db.db.Where("tenant_id = ? AND status <> ?", tenantID, model.TenantMemberRemoved).
		Order("role DESC, user_id ASC").
		Find(&members).Error
	if err != nil {
		return nil, fmt.Errorf("查询租户成员列表失败: %w", err)
	}
	return members, nil
}

func (db *gor) UpsertTenantMember(tenantID, userID int64, role int16, deptID int64) error {
	member := model.TenantMember{
		TenantID: tenantID,
		UserID:   userID,
		Role:     role,
		DeptID:   deptID,
		Status:   model.TenantMemberNormal,
	}
	err := db.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Assign(map[string]interface{}{
			"role":    role,
			"dept_id": deptID,
			"status":  model.TenantMemberNormal,
		}).
		FirstOrCreate(&member).Error
	if err != nil {
		return fmt.Errorf("写入租户成员失败: %w", err)
	}
	return nil
}

func (db *gor) UpdateTenantMemberRole(tenantID, userID int64, role int16) error {
	err := db.db.Model(&model.TenantMember{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update("role", role).Error
	if err != nil {
		return fmt.Errorf("更新租户成员角色失败: %w", err)
	}
	return nil
}

func (db *gor) UpdateTenantMemberDept(tenantID, userID int64, deptID int64) error {
	err := db.db.Model(&model.TenantMember{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update("dept_id", deptID).Error
	if err != nil {
		return fmt.Errorf("更新成员部门失败: %w", err)
	}
	return nil
}

func (db *gor) RemoveTenantMember(tenantID, userID int64) error {
	err := db.db.Model(&model.TenantMember{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update("status", model.TenantMemberRemoved).Error
	if err != nil {
		return fmt.Errorf("移除租户成员失败: %w", err)
	}
	return nil
}

func (db *gor) CreateDepartment(dept model.TenantDepartment) (model.TenantDepartment, error) {
	err := db.db.Create(&dept).Error
	if err != nil {
		return model.TenantDepartment{}, fmt.Errorf("创建部门失败: %w", err)
	}
	return dept, nil
}

func (db *gor) GetDepartment(deptID int64) (model.TenantDepartment, error) {
	var dept model.TenantDepartment
	err := db.db.Where("id = ?", deptID).First(&dept).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.TenantDepartment{}, nil
		}
		return model.TenantDepartment{}, fmt.Errorf("查询部门失败: %w", err)
	}
	return dept, nil
}

func (db *gor) ListDepartments(tenantID int64) ([]model.TenantDepartment, error) {
	var depts []model.TenantDepartment
	err := db.db.Where("tenant_id = ?", tenantID).
		Order("sort_order ASC, id ASC").
		Find(&depts).Error
	if err != nil {
		return nil, fmt.Errorf("查询部门列表失败: %w", err)
	}
	return depts, nil
}

func (db *gor) UpdateDepartmentName(deptID int64, name string) error {
	err := db.db.Model(&model.TenantDepartment{}).Where("id = ?", deptID).Update("name", name).Error
	if err != nil {
		return fmt.Errorf("重命名部门失败: %w", err)
	}
	return nil
}

func (db *gor) UpdateDepartmentParent(deptID, parentID int64) error {
	err := db.db.Model(&model.TenantDepartment{}).Where("id = ?", deptID).Update("parent_id", parentID).Error
	if err != nil {
		return fmt.Errorf("移动部门失败: %w", err)
	}
	return nil
}

func (db *gor) RemoveDepartment(dept model.TenantDepartment) error {
	err := db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.TenantDepartment{}).
			Where("tenant_id = ? AND parent_id = ?", dept.TenantID, dept.ID).
			Update("parent_id", dept.ParentID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.TenantMember{}).
			Where("tenant_id = ? AND dept_id = ?", dept.TenantID, dept.ID).
			Update("dept_id", 0).Error; err != nil {
			return err
		}
		return tx.Delete(&model.TenantDepartment{}, dept.ID).Error
	})
	if err != nil {
		return fmt.Errorf("删除部门失败: %w", err)
	}
	return nil
}
