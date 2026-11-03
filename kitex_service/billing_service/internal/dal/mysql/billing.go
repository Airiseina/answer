package mysql

import (
	"github.com/Airiseina/answer/kitex_service/billing_service/internal/model"

	"gorm.io/gorm"
)

// ===== 用量明细 =====

// CreateUsage 写入一条用量明细；msg_id 已存在时返回 ErrDuplicateUsage（幂等去重）。
func (g *gor) CreateUsage(u *model.TokenUsage) error {
	var existing model.TokenUsage
	err := g.db.Where("msg_id = ?", u.MsgID).First(&existing).Error
	if err == nil {
		return ErrDuplicateUsage
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	if err = g.db.Create(u).Error; err != nil {
		// 并发下唯一索引兜底：再次确认是否已存在，命中则仍按幂等处理。
		var again model.TokenUsage
		if e := g.db.Where("msg_id = ?", u.MsgID).First(&again).Error; e == nil {
			return ErrDuplicateUsage
		}
		return err
	}
	return nil
}

// GetUsageByMsgID 按 msg_id 查询用量，用于幂等排查。
func (g *gor) GetUsageByMsgID(msgID int64) (model.TokenUsage, bool, error) {
	var u model.TokenUsage
	err := g.db.Where("msg_id = ?", msgID).First(&u).Error
	if err == gorm.ErrRecordNotFound {
		return model.TokenUsage{}, false, nil
	}
	if err != nil {
		return model.TokenUsage{}, false, err
	}
	return u, true, nil
}

// usageScope 统一按「本人 + 可选日期/模型/Bot」过滤，用量看板仅本人可见。
func (g *gor) usageScope(userID int64, startDate, endDate, modelName string, botID int64) *gorm.DB {
	tx := g.db.Model(&model.TokenUsage{}).Where("user_id = ?", userID)
	if startDate != "" {
		tx = tx.Where("biz_date >= ?", startDate)
	}
	if endDate != "" {
		tx = tx.Where("biz_date <= ?", endDate)
	}
	if modelName != "" {
		tx = tx.Where("model = ?", modelName)
	}
	if botID > 0 {
		tx = tx.Where("bot_id = ?", botID)
	}
	return tx
}

// ListUsage 分页返回用量明细与总数。
func (g *gor) ListUsage(userID int64, startDate, endDate, modelName string, botID int64, limit, offset int) ([]model.TokenUsage, int64, error) {
	tx := g.usageScope(userID, startDate, endDate, modelName, botID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.TokenUsage
	if err := tx.Order("id DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// AggregateTotals 汇总区间的 token 与成本合计。
func (g *gor) AggregateTotals(userID int64, startDate, endDate string) (TokenTotals, error) {
	var row struct {
		PromptTokens     int64
		CompletionTokens int64
		Cost             float64
		RequestCount     int64
	}
	err := g.usageScope(userID, startDate, endDate, "", 0).
		Select("COALESCE(SUM(prompt_tokens),0) AS prompt_tokens, COALESCE(SUM(completion_tokens),0) AS completion_tokens, COALESCE(SUM(cost),0) AS cost, COUNT(*) AS request_count").
		Scan(&row).Error
	if err != nil {
		return TokenTotals{}, err
	}
	return TokenTotals{
		PromptTokens:     row.PromptTokens,
		CompletionTokens: row.CompletionTokens,
		Cost:             row.Cost,
		RequestCount:     row.RequestCount,
	}, nil
}

// AggregateTrend 按天汇总用量趋势。
func (g *gor) AggregateTrend(userID int64, startDate, endDate string) ([]TrendRow, error) {
	var rows []TrendRow
	err := g.usageScope(userID, startDate, endDate, "", 0).
		Select("biz_date, COALESCE(SUM(prompt_tokens),0) AS prompt_tokens, COALESCE(SUM(completion_tokens),0) AS completion_tokens, COALESCE(SUM(cost),0) AS cost, COUNT(*) AS request_count").
		Group("biz_date").
		Order("biz_date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// AggregateByBot 按 Bot 聚合用量占比（取 token 最多的前 20 个）。
func (g *gor) AggregateByBot(userID int64, startDate, endDate string) ([]GroupRow, error) {
	var rows []GroupRow
	err := g.usageScope(userID, startDate, endDate, "", 0).
		Select("CAST(bot_id AS CHAR) AS `key`, COALESCE(SUM(prompt_tokens),0) AS prompt_tokens, COALESCE(SUM(completion_tokens),0) AS completion_tokens, COALESCE(SUM(cost),0) AS cost, COUNT(*) AS request_count").
		Group("bot_id").
		Order("(SUM(prompt_tokens) + SUM(completion_tokens)) DESC").
		Limit(20).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// AggregateByModel 按模型聚合用量占比（取 token 最多的前 20 个）。
func (g *gor) AggregateByModel(userID int64, startDate, endDate string) ([]GroupRow, error) {
	var rows []GroupRow
	err := g.usageScope(userID, startDate, endDate, "", 0).
		Select("model AS `key`, COALESCE(SUM(prompt_tokens),0) AS prompt_tokens, COALESCE(SUM(completion_tokens),0) AS completion_tokens, COALESCE(SUM(cost),0) AS cost, COUNT(*) AS request_count").
		Group("model").
		Order("(SUM(prompt_tokens) + SUM(completion_tokens)) DESC").
		Limit(20).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// CountMissingPrice 统计区间内未命中参考单价的记录数。
func (g *gor) CountMissingPrice(userID int64, startDate, endDate string) (int64, error) {
	var count int64
	err := g.usageScope(userID, startDate, endDate, "", 0).
		Where("price_missing = ?", true).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ===== 参考单价 =====

func (g *gor) ListModelPrices(onlyEnabled bool, limit, offset int) ([]model.ModelPrice, int64, error) {
	tx := g.db.Model(&model.ModelPrice{})
	if onlyEnabled {
		tx = tx.Where("enabled = ?", true)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var prices []model.ModelPrice
	if err := tx.Order("id DESC").Limit(limit).Offset(offset).Find(&prices).Error; err != nil {
		return nil, 0, err
	}
	return prices, total, nil
}

// GetModelPriceByModel 按模型名查启用的单价，供成本折算使用。
func (g *gor) GetModelPriceByModel(modelName string) (model.ModelPrice, bool, error) {
	var p model.ModelPrice
	err := g.db.Where("model = ? AND enabled = ?", modelName, true).Order("id DESC").First(&p).Error
	if err == gorm.ErrRecordNotFound {
		return model.ModelPrice{}, false, nil
	}
	if err != nil {
		return model.ModelPrice{}, false, err
	}
	return p, true, nil
}

func (g *gor) GetModelPriceByID(id int64) (model.ModelPrice, bool, error) {
	var p model.ModelPrice
	err := g.db.Where("id = ?", id).First(&p).Error
	if err == gorm.ErrRecordNotFound {
		return model.ModelPrice{}, false, nil
	}
	if err != nil {
		return model.ModelPrice{}, false, err
	}
	return p, true, nil
}

func (g *gor) CreateModelPrice(p *model.ModelPrice) error {
	return g.db.Create(p).Error
}

func (g *gor) UpdateModelPrice(p *model.ModelPrice) error {
	return g.db.Save(p).Error
}

func (g *gor) DeleteModelPrice(id int64) error {
	return g.db.Where("id = ?", id).Delete(&model.ModelPrice{}).Error
}

// ===== 用量配额 =====

func (g *gor) ListUsageQuotas(ownerType int16, ownerID int64) ([]model.UsageQuota, error) {
	tx := g.db.Model(&model.UsageQuota{})
	if ownerID > 0 {
		tx = tx.Where("owner_type = ? AND owner_id = ?", ownerType, ownerID)
	}
	var quotas []model.UsageQuota
	if err := tx.Order("id DESC").Find(&quotas).Error; err != nil {
		return nil, err
	}
	return quotas, nil
}

// UpsertUsageQuota 按 (owner_type, owner_id, period) 唯一写入或更新配额。
func (g *gor) UpsertUsageQuota(q *model.UsageQuota) error {
	var existing model.UsageQuota
	err := g.db.Where("owner_type = ? AND owner_id = ? AND period = ?", q.OwnerType, q.OwnerID, q.Period).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return g.db.Create(q).Error
	}
	if err != nil {
		return err
	}
	existing.TokenLimit = q.TokenLimit
	existing.Enabled = q.Enabled
	return g.db.Save(&existing).Error
}
