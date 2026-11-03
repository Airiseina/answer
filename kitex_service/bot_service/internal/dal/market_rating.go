package dal

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"

	"gorm.io/gorm"
)

type marketRatingDao struct {
	db *gorm.DB
}

func NewMarketRatingDao(db *gorm.DB) MarketRatingDao {
	return &marketRatingDao{db}
}

type MarketRatingDao interface {
	UpsertRating(rating model.MarketBotRating) error
}

// UpsertRating 写入或更新评分，并在同一事务内维护市场条目的评分汇总。
// 同一用户对同一 Bot 重复评分视为改分，评分人数不重复累加。
func (d *marketRatingDao) UpsertRating(rating model.MarketBotRating) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		var existing model.MarketBotRating
		err := tx.Where("market_bot_id = ? AND user_id = ?", rating.MarketBotID, rating.UserID).
			First(&existing).Error

		switch {
		case err == nil:
			if existing.Score == rating.Score {
				return nil
			}
			delta := int64(rating.Score) - int64(existing.Score)
			if err := tx.Model(&model.MarketBotRating{}).Where("id = ?", existing.ID).
				Updates(map[string]interface{}{"score": rating.Score}).Error; err != nil {
				return fmt.Errorf("更新评分失败: %w", err)
			}
			if err := tx.Model(&model.MarketBot{}).Where("id = ?", rating.MarketBotID).
				UpdateColumn("rating_sum", gorm.Expr("rating_sum + ?", delta)).Error; err != nil {
				return fmt.Errorf("更新评分汇总失败: %w", err)
			}
			return nil
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := tx.Create(&rating).Error; err != nil {
				return fmt.Errorf("写入评分失败: %w", err)
			}
			if err := tx.Model(&model.MarketBot{}).Where("id = ?", rating.MarketBotID).
				Updates(map[string]interface{}{
					"rating_sum":   gorm.Expr("rating_sum + ?", rating.Score),
					"rating_count": gorm.Expr("rating_count + ?", 1),
				}).Error; err != nil {
				return fmt.Errorf("更新评分汇总失败: %w", err)
			}
			return nil
		default:
			return fmt.Errorf("查询评分失败: %w", err)
		}
	})
}
