package dal

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"

	"gorm.io/gorm"
)

type userSkillDao struct {
	db *gorm.DB
}

func NewUserSkillDao(db *gorm.DB) UserSkillDao {
	return &userSkillDao{db}
}

type UserSkillDao interface {
	CreateUserSkill(skill model.UserSkill) error
	GetUserSkill(tenantId, skillId int64) (model.UserSkill, error)
	GetUserSkillByName(tenantId, ownerId int64, name string) (model.UserSkill, error)
	ListUserSkills(tenantId, ownerId int64) ([]model.UserSkill, error)
	UpdateUserSkill(tenantId, skillId int64, updates map[string]interface{}) error
	DeleteUserSkill(tenantId, skillId int64) error

	CreateSkillFile(file model.UserSkillFile) error
	ListSkillFiles(tenantId, skillId int64) ([]model.UserSkillFile, error)
	DeleteSkillFiles(tenantId, skillId int64) error

	UpsertBotSkill(bind model.BotSkill) error
	DeleteBotSkill(tenantId, botId, skillId int64) error
	ListBotSkills(tenantId, botId int64) ([]model.BotSkill, error)
	ListBotSkillsBySkill(tenantId, skillId int64) ([]model.BotSkill, error)
	DeleteBotSkillsBySkill(tenantId, skillId int64) error
}

func (d *userSkillDao) CreateUserSkill(skill model.UserSkill) error {
	if err := d.db.Create(&skill).Error; err != nil {
		return fmt.Errorf("创建Skill失败: %w", err)
	}
	return nil
}

// GetUserSkill 按「租户 + Skill」读取，租户条件强制生效，避免跨租户读取。
func (d *userSkillDao) GetUserSkill(tenantId, skillId int64) (model.UserSkill, error) {
	var skill model.UserSkill
	err := d.db.Where("id = ? AND tenant_id = ?", skillId, tenantId).First(&skill).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.UserSkill{}, nil
		}
		return model.UserSkill{}, fmt.Errorf("查询Skill失败: %w", err)
	}
	return skill, nil
}

// GetUserSkillByName 用于同一用户重复上传同名 Skill 时定位旧记录，做覆盖更新。
func (d *userSkillDao) GetUserSkillByName(tenantId, ownerId int64, name string) (model.UserSkill, error) {
	var skill model.UserSkill
	err := d.db.Where("tenant_id = ? AND owner_id = ? AND name = ?", tenantId, ownerId, name).
		First(&skill).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.UserSkill{}, nil
		}
		return model.UserSkill{}, fmt.Errorf("查询Skill失败: %w", err)
	}
	return skill, nil
}

func (d *userSkillDao) ListUserSkills(tenantId, ownerId int64) ([]model.UserSkill, error) {
	var skills []model.UserSkill
	err := d.db.Where("tenant_id = ? AND owner_id = ?", tenantId, ownerId).
		Order("updated_at DESC, id DESC").Find(&skills).Error
	if err != nil {
		return nil, fmt.Errorf("查询Skill列表失败: %w", err)
	}
	return skills, nil
}

func (d *userSkillDao) UpdateUserSkill(tenantId, skillId int64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	err := d.db.Model(&model.UserSkill{}).Where("id = ? AND tenant_id = ?", skillId, tenantId).
		Updates(updates).Error
	if err != nil {
		return fmt.Errorf("更新Skill失败: %w", err)
	}
	return nil
}

func (d *userSkillDao) DeleteUserSkill(tenantId, skillId int64) error {
	err := d.db.Where("id = ? AND tenant_id = ?", skillId, tenantId).Delete(&model.UserSkill{}).Error
	if err != nil {
		return fmt.Errorf("删除Skill失败: %w", err)
	}
	return nil
}

func (d *userSkillDao) CreateSkillFile(file model.UserSkillFile) error {
	if err := d.db.Create(&file).Error; err != nil {
		return fmt.Errorf("写入Skill文件失败: %w", err)
	}
	return nil
}

// ListSkillFiles 按相对路径升序返回，保证前端展示顺序稳定。
func (d *userSkillDao) ListSkillFiles(tenantId, skillId int64) ([]model.UserSkillFile, error) {
	var files []model.UserSkillFile
	err := d.db.Where("tenant_id = ? AND skill_id = ?", tenantId, skillId).
		Order("rel_path ASC, id ASC").Find(&files).Error
	if err != nil {
		return nil, fmt.Errorf("查询Skill文件失败: %w", err)
	}
	return files, nil
}

func (d *userSkillDao) DeleteSkillFiles(tenantId, skillId int64) error {
	err := d.db.Where("tenant_id = ? AND skill_id = ?", tenantId, skillId).
		Delete(&model.UserSkillFile{}).Error
	if err != nil {
		return fmt.Errorf("删除Skill文件失败: %w", err)
	}
	return nil
}

// UpsertBotSkill 以「租户 + Bot + Skill」为复合主键，重复绑定时只刷新启用状态。
func (d *userSkillDao) UpsertBotSkill(bind model.BotSkill) error {
	var existing model.BotSkill
	err := d.db.Where("tenant_id = ? AND bot_id = ? AND skill_id = ?",
		bind.TenantID, bind.BotID, bind.SkillID).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("查询Bot Skill绑定失败: %w", err)
	}
	if existing.SkillID != 0 {
		updates := map[string]interface{}{"enabled": bind.Enabled, "owner_id": bind.OwnerID}
		if err := d.db.Model(&model.BotSkill{}).
			Where("tenant_id = ? AND bot_id = ? AND skill_id = ?", bind.TenantID, bind.BotID, bind.SkillID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("更新Bot Skill绑定失败: %w", err)
		}
		return nil
	}
	if err := d.db.Create(&bind).Error; err != nil {
		return fmt.Errorf("创建Bot Skill绑定失败: %w", err)
	}
	return nil
}

func (d *userSkillDao) DeleteBotSkill(tenantId, botId, skillId int64) error {
	err := d.db.Where("tenant_id = ? AND bot_id = ? AND skill_id = ?", tenantId, botId, skillId).
		Delete(&model.BotSkill{}).Error
	if err != nil {
		return fmt.Errorf("解除Bot Skill绑定失败: %w", err)
	}
	return nil
}

func (d *userSkillDao) ListBotSkills(tenantId, botId int64) ([]model.BotSkill, error) {
	var binds []model.BotSkill
	err := d.db.Where("tenant_id = ? AND bot_id = ?", tenantId, botId).
		Order("created_at ASC, skill_id ASC").Find(&binds).Error
	if err != nil {
		return nil, fmt.Errorf("查询Bot Skill绑定失败: %w", err)
	}
	return binds, nil
}

// ListBotSkillsBySkill 反查某个 Skill 被绑定到了哪些 Bot，供删除前逐个解绑知识库。
func (d *userSkillDao) ListBotSkillsBySkill(tenantId, skillId int64) ([]model.BotSkill, error) {
	var binds []model.BotSkill
	err := d.db.Where("tenant_id = ? AND skill_id = ?", tenantId, skillId).
		Order("bot_id ASC").Find(&binds).Error
	if err != nil {
		return nil, fmt.Errorf("查询Skill绑定失败: %w", err)
	}
	return binds, nil
}

// DeleteBotSkillsBySkill 在删除 Skill 时清理所有 Bot 上的残留绑定，避免悬空引用。
func (d *userSkillDao) DeleteBotSkillsBySkill(tenantId, skillId int64) error {
	err := d.db.Where("tenant_id = ? AND skill_id = ?", tenantId, skillId).
		Delete(&model.BotSkill{}).Error
	if err != nil {
		return fmt.Errorf("清理Bot Skill绑定失败: %w", err)
	}
	return nil
}
