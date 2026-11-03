package mysql

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/admin_service/internal/model"

	"gorm.io/gorm"
)

func (db *gor) CreateAdmin(account, name, hash, role string) (int64, error) {
	admin := model.AdminUser{
		Account: account,
		Name:    name,
		Hash:    hash,
		Role:    role,
		Status:  model.StatusEnabled,
	}
	if err := db.db.Create(&admin).Error; err != nil {
		return 0, fmt.Errorf("创建管理员失败: %w", err)
	}
	return admin.ID, nil
}

func (db *gor) GetAdminByAccount(account string) (model.AdminUser, error) {
	var admin model.AdminUser
	err := db.db.Where("account = ?", account).First(&admin).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.AdminUser{}, nil
		}
		return model.AdminUser{}, fmt.Errorf("查询管理员失败: %w", err)
	}
	return admin, nil
}

func (db *gor) GetAdminById(id int64) (model.AdminUser, error) {
	var admin model.AdminUser
	err := db.db.Where("id = ?", id).First(&admin).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.AdminUser{}, nil
		}
		return model.AdminUser{}, fmt.Errorf("查询管理员失败: %w", err)
	}
	return admin, nil
}

func (db *gor) ListAdmins() ([]model.AdminUser, error) {
	var admins []model.AdminUser
	if err := db.db.Order("id asc").Find(&admins).Error; err != nil {
		return nil, fmt.Errorf("查询管理员列表失败: %w", err)
	}
	return admins, nil
}

func (db *gor) CountAdmins() (int64, error) {
	var count int64
	if err := db.db.Model(&model.AdminUser{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计管理员数量失败: %w", err)
	}
	return count, nil
}

func (db *gor) UpdatePassword(id int64, hash string) error {
	err := db.db.Model(&model.AdminUser{}).Where("id = ?", id).Update("hash", hash).Error
	if err != nil {
		return fmt.Errorf("更新管理员密码失败: %w", err)
	}
	return nil
}

func (db *gor) UpdateStatus(id int64, status int16) error {
	err := db.db.Model(&model.AdminUser{}).Where("id = ?", id).Update("status", status).Error
	if err != nil {
		return fmt.Errorf("更新管理员状态失败: %w", err)
	}
	return nil
}

func (db *gor) UpdateLastLogin(id int64, ts int64) error {
	err := db.db.Model(&model.AdminUser{}).Where("id = ?", id).Update("last_login_at", ts).Error
	if err != nil {
		return fmt.Errorf("更新登录时间失败: %w", err)
	}
	return nil
}

func (db *gor) UpdateNameAndRole(id int64, name, role string) error {
	err := db.db.Model(&model.AdminUser{}).Where("id = ?", id).
		Updates(map[string]interface{}{"name": name, "role": role}).Error
	if err != nil {
		return fmt.Errorf("更新管理员信息失败: %w", err)
	}
	return nil
}
