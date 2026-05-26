package dal

import (
	"errors"
	"fmt"
	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"

	"gorm.io/gorm"
)

func (d *botDao) CreateBot(bot model.Bot) error {
	err := d.db.Create(&bot).Error
	if err != nil {
		return fmt.Errorf("创建Bot失败: %w", err)
	}
	return nil
}

func (d *botDao) GetBot(botId int64) (model.Bot, error) {
	var bot model.Bot
	err := d.db.Where("id = ?", botId).First(&bot).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Bot{}, nil
		}
		return model.Bot{}, fmt.Errorf("查询Bot失败: %w", err)
	}
	return bot, nil
}

func (d *botDao) GetSystemBot() (model.Bot, error) {
	var bot model.Bot
	err := d.db.Where("is_system = ?", true).First(&bot).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Bot{}, nil
		}
		return model.Bot{}, fmt.Errorf("查询系统Bot失败: %w", err)
	}
	return bot, nil
}

func (d *botDao) GetUserBots(creatorId int64) ([]model.Bot, error) {
	var bots []model.Bot
	err := d.db.Where("creator_id = ? OR is_system = ?", creatorId, true).Find(&bots).Error
	if err != nil {
		return nil, fmt.Errorf("查询用户Bot列表失败: %w", err)
	}
	return bots, nil
}

func (d *botDao) UpdateBot(botId int64, updates map[string]interface{}) error {
	err := d.db.Model(&model.Bot{}).Where("id = ?", botId).Updates(updates).Error
	if err != nil {
		return fmt.Errorf("更新Bot失败: %w", err)
	}
	return nil
}

func (d *botDao) DeleteBot(botId int64) error {
	err := d.db.Where("id = ?", botId).Delete(&model.Bot{}).Error
	if err != nil {
		return fmt.Errorf("删除Bot失败: %w", err)
	}
	return nil
}

func (d *botDao) IsBot(userId int64) (bool, error) {
	var count int64
	err := d.db.Model(&model.Bot{}).Where("user_id = ?", userId).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("查询IsBot失败: %w", err)
	}
	return count > 0, nil
}

func (d *botDao) GetBotByUserId(userId int64) (model.Bot, error) {
	var bot model.Bot
	err := d.db.Where("user_id = ?", userId).First(&bot).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Bot{}, nil
		}
		return model.Bot{}, fmt.Errorf("查询Bot失败: %w", err)
	}
	return bot, nil
}
