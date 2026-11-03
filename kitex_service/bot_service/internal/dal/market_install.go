package dal

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"

	"gorm.io/gorm"
)

type marketInstallDao struct {
	db *gorm.DB
}

func NewMarketInstallDao(db *gorm.DB) MarketInstallDao {
	return &marketInstallDao{db}
}

type MarketInstallDao interface {
	CreateMarketInstall(install model.MarketBotInstall) error
	GetUserInstall(tenantId, userId, marketBotId int64) (model.MarketBotInstall, error)
}

func (d *marketInstallDao) CreateMarketInstall(install model.MarketBotInstall) error {
	if err := d.db.Create(&install).Error; err != nil {
		return fmt.Errorf("写入市场安装记录失败: %w", err)
	}
	return nil
}

// GetUserInstall 查询某用户在指定租户下是否已安装过该市场 Bot，用于防止重复安装。
func (d *marketInstallDao) GetUserInstall(tenantId, userId, marketBotId int64) (model.MarketBotInstall, error) {
	var install model.MarketBotInstall
	err := d.db.Where("tenant_id = ? AND user_id = ? AND market_bot_id = ?", tenantId, userId, marketBotId).
		First(&install).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.MarketBotInstall{}, nil
		}
		return model.MarketBotInstall{}, fmt.Errorf("查询市场安装记录失败: %w", err)
	}
	return install, nil
}
