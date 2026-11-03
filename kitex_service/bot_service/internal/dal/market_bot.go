package dal

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"

	"gorm.io/gorm"
)

type marketBotDao struct {
	db *gorm.DB
}

func NewMarketBotDao(db *gorm.DB) MarketBotDao {
	return &marketBotDao{db}
}

type MarketBotDao interface {
	CreateMarketBot(bot model.MarketBot) error
	GetMarketBot(id int64) (model.MarketBot, error)
	GetMarketBotForPublisher(tenantId, id int64) (model.MarketBot, error)
	UpdateMarketBot(id int64, updates map[string]interface{}) error
	ListPublished(keyword, category string, offset, limit int) ([]model.MarketBot, int64, error)
	ListByPublisher(tenantId, publisherId int64, status int16, offset, limit int) ([]model.MarketBot, int64, error)
	ListByStatus(status int16, offset, limit int) ([]model.MarketBot, int64, error)
	IncreaseInstallCount(id int64) error
}

func (d *marketBotDao) CreateMarketBot(bot model.MarketBot) error {
	if err := d.db.Create(&bot).Error; err != nil {
		return fmt.Errorf("创建市场Bot失败: %w", err)
	}
	return nil
}

// GetMarketBot 按 ID 全局读取市场条目。
//
// 市场是全局可见的，因此这里刻意不加租户过滤；可见范围（是否已发布）由 service 层判定。
func (d *marketBotDao) GetMarketBot(id int64) (model.MarketBot, error) {
	var bot model.MarketBot
	err := d.db.Where("id = ?", id).First(&bot).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.MarketBot{}, nil
		}
		return model.MarketBot{}, fmt.Errorf("查询市场Bot失败: %w", err)
	}
	return bot, nil
}

// GetMarketBotForPublisher 供发布者管理自己条目时使用，强制限定发布租户，避免跨租户篡改。
func (d *marketBotDao) GetMarketBotForPublisher(tenantId, id int64) (model.MarketBot, error) {
	var bot model.MarketBot
	err := d.db.Where("id = ? AND tenant_id = ?", id, tenantId).First(&bot).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.MarketBot{}, nil
		}
		return model.MarketBot{}, fmt.Errorf("查询市场Bot失败: %w", err)
	}
	return bot, nil
}

func (d *marketBotDao) UpdateMarketBot(id int64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	err := d.db.Model(&model.MarketBot{}).Where("id = ?", id).Updates(updates).Error
	if err != nil {
		return fmt.Errorf("更新市场Bot失败: %w", err)
	}
	return nil
}

// 列表类查询的计数与取列表刻意使用两个独立的 builder。
//
// GORM 在同一个 Statement 上连续执行 Count + Find 是常见写法：非 DryRun 会话会在
// 每次执行后重置 SQL 缓冲，因此第二次会重建语句；但 DryRun 会话（测试用）不会重置，
// 第二次查询会直接复用计数语句。拆成两个 builder 后，两条语句都能被正确构造，
// 既让生产路径与测试路径行为一致，也让 DryRun 断言能覆盖到排序等子句。

func (d *marketBotDao) ListPublished(keyword, category string, offset, limit int) ([]model.MarketBot, int64, error) {
	build := func() *gorm.DB {
		return applyMarketFilter(
			d.db.Model(&model.MarketBot{}).Where("status = ?", model.MarketStatusPublished),
			keyword, category,
		)
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计市场Bot失败: %w", err)
	}
	var bots []model.MarketBot
	if err := build().Order("install_count DESC, id DESC").Offset(offset).Limit(limit).Find(&bots).Error; err != nil {
		return nil, 0, fmt.Errorf("查询市场Bot列表失败: %w", err)
	}
	return bots, total, nil
}

func (d *marketBotDao) ListByPublisher(tenantId, publisherId int64, status int16, offset, limit int) ([]model.MarketBot, int64, error) {
	build := func() *gorm.DB {
		q := d.db.Model(&model.MarketBot{}).Where("tenant_id = ? AND publisher_id = ?", tenantId, publisherId)
		if model.ValidMarketStatus(status) {
			q = q.Where("status = ?", status)
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计我的发布失败: %w", err)
	}
	var bots []model.MarketBot
	if err := build().Order("updated_at DESC, id DESC").Offset(offset).Limit(limit).Find(&bots).Error; err != nil {
		return nil, 0, fmt.Errorf("查询我的发布失败: %w", err)
	}
	return bots, total, nil
}

func (d *marketBotDao) ListByStatus(status int16, offset, limit int) ([]model.MarketBot, int64, error) {
	build := func() *gorm.DB {
		return d.db.Model(&model.MarketBot{}).Where("status = ?", status)
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计市场审核列表失败: %w", err)
	}
	var bots []model.MarketBot
	if err := build().Order("updated_at ASC, id ASC").Offset(offset).Limit(limit).Find(&bots).Error; err != nil {
		return nil, 0, fmt.Errorf("查询市场审核列表失败: %w", err)
	}
	return bots, total, nil
}

func (d *marketBotDao) IncreaseInstallCount(id int64) error {
	err := d.db.Model(&model.MarketBot{}).Where("id = ?", id).
		UpdateColumn("install_count", gorm.Expr("install_count + ?", 1)).Error
	if err != nil {
		return fmt.Errorf("累加安装量失败: %w", err)
	}
	return nil
}

// applyMarketFilter 给市场查询附加关键词与分类过滤。
func applyMarketFilter(db *gorm.DB, keyword, category string) *gorm.DB {
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR description LIKE ? OR tags LIKE ?", like, like, like)
	}
	if category != "" {
		db = db.Where("category = ?", category)
	}
	return db
}
