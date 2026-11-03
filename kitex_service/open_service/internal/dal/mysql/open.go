package mysql

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/open_service/internal/model"

	"gorm.io/gorm"
)

// ===== 应用 =====

func (db *gor) CreateApp(app *model.OpenApp) error {
	if err := db.db.Create(app).Error; err != nil {
		return fmt.Errorf("创建应用失败: %w", err)
	}
	return nil
}

func (db *gor) GetAppByAppID(appID string) (model.OpenApp, error) {
	var app model.OpenApp
	err := db.db.Where("app_id = ?", appID).First(&app).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.OpenApp{}, nil
		}
		return model.OpenApp{}, fmt.Errorf("查询应用失败: %w", err)
	}
	return app, nil
}

func (db *gor) ListApps(ownerID, tenantID int64) ([]model.OpenApp, error) {
	var apps []model.OpenApp
	q := db.db.Where("owner_id = ?", ownerID)
	// tenantID<=0 视为不过滤，供管理后台等系统内部调用。
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.Order("id desc").Find(&apps).Error; err != nil {
		return nil, fmt.Errorf("查询应用列表失败: %w", err)
	}
	return apps, nil
}

func (db *gor) UpdateAppSecret(appID, secretHash, secretCipher string) error {
	err := db.db.Model(&model.OpenApp{}).Where("app_id = ?", appID).
		Updates(map[string]interface{}{
			"app_secret_hash":   secretHash,
			"app_secret_cipher": secretCipher,
		}).Error
	if err != nil {
		return fmt.Errorf("更新应用密钥失败: %w", err)
	}
	return nil
}

func (db *gor) UpdateAppStatus(appID string, status int16) error {
	err := db.db.Model(&model.OpenApp{}).Where("app_id = ?", appID).Update("status", status).Error
	if err != nil {
		return fmt.Errorf("更新应用状态失败: %w", err)
	}
	return nil
}

// ===== 回调 =====

func (db *gor) CreateWebhook(wh *model.OpenWebhook) error {
	if err := db.db.Create(wh).Error; err != nil {
		return fmt.Errorf("创建回调失败: %w", err)
	}
	return nil
}

func (db *gor) ListWebhooks(appID string, botID int64) ([]model.OpenWebhook, error) {
	var list []model.OpenWebhook
	q := db.db.Where("app_id = ?", appID)
	if botID > 0 {
		q = q.Where("bot_id = ?", botID)
	}
	if err := q.Order("id desc").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("查询回调列表失败: %w", err)
	}
	return list, nil
}

// ListEnabledWebhooks 返回某应用下订阅了指定事件类型（或订阅全部）的启用回调。
func (db *gor) ListEnabledWebhooks(appID string, eventType string) ([]model.OpenWebhook, error) {
	var list []model.OpenWebhook
	q := db.db.Where("app_id = ? AND enabled = ?", appID, true)
	if eventType != "" {
		q = q.Where("events = '' OR events LIKE ?", "%"+eventType+"%")
	}
	if err := q.Order("id asc").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("查询启用回调失败: %w", err)
	}
	return list, nil
}

func (db *gor) GetWebhook(appID string, id int64) (model.OpenWebhook, error) {
	var wh model.OpenWebhook
	err := db.db.Where("app_id = ? AND id = ?", appID, id).First(&wh).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.OpenWebhook{}, nil
		}
		return model.OpenWebhook{}, fmt.Errorf("查询回调失败: %w", err)
	}
	return wh, nil
}

func (db *gor) UpdateWebhookStatus(id int64, status string) error {
	err := db.db.Model(&model.OpenWebhook{}).Where("id = ?", id).Update("status", status).Error
	if err != nil {
		return fmt.Errorf("更新回调状态失败: %w", err)
	}
	return nil
}

func (db *gor) DeleteWebhook(appID string, id int64) error {
	err := db.db.Where("app_id = ? AND id = ?", appID, id).Delete(&model.OpenWebhook{}).Error
	if err != nil {
		return fmt.Errorf("删除回调失败: %w", err)
	}
	return nil
}

// ===== 事件 =====

func (db *gor) AppendEvent(ev *model.OpenEvent) error {
	if err := db.db.Create(ev).Error; err != nil {
		return fmt.Errorf("写入事件失败: %w", err)
	}
	return nil
}

func (db *gor) ListEvents(appID string, sinceID int64, limit int) ([]model.OpenEvent, error) {
	var list []model.OpenEvent
	err := db.db.Where("app_id = ? AND id > ?", appID, sinceID).
		Order("id asc").Limit(limit).Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("查询事件失败: %w", err)
	}
	return list, nil
}

func (db *gor) AckEvents(appID string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	err := db.db.Model(&model.OpenEvent{}).
		Where("app_id = ? AND id IN ?", appID, ids).
		Update("status", model.EventStatusAcked).Error
	if err != nil {
		return fmt.Errorf("确认事件失败: %w", err)
	}
	return nil
}

func (db *gor) MarkEventsDelivered(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	err := db.db.Model(&model.OpenEvent{}).
		Where("id IN ? AND status = ?", ids, model.EventStatusPending).
		Update("status", model.EventStatusDelivered).Error
	if err != nil {
		return fmt.Errorf("标记事件已投递失败: %w", err)
	}
	return nil
}

// PurgeEventsBefore 清理过期事件，返回删除条数；保留期由配置控制。
func (db *gor) PurgeEventsBefore(ts int64) (int64, error) {
	res := db.db.Where("created_at < ?", ts).Delete(&model.OpenEvent{})
	if res.Error != nil {
		return 0, fmt.Errorf("清理过期事件失败: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// ===== 投递 =====

func (db *gor) CreateDelivery(d *model.OpenWebhookDelivery) error {
	if err := db.db.Create(d).Error; err != nil {
		return fmt.Errorf("创建投递记录失败: %w", err)
	}
	return nil
}

func (db *gor) GetDeliveryByEventWebhook(webhookID, eventID int64) (model.OpenWebhookDelivery, error) {
	var d model.OpenWebhookDelivery
	err := db.db.Where("webhook_id = ? AND event_id = ?", webhookID, eventID).First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.OpenWebhookDelivery{}, nil
		}
		return model.OpenWebhookDelivery{}, fmt.Errorf("查询投递记录失败: %w", err)
	}
	return d, nil
}

func (db *gor) UpdateDelivery(d *model.OpenWebhookDelivery) error {
	err := db.db.Model(&model.OpenWebhookDelivery{}).Where("id = ?", d.ID).
		Updates(map[string]interface{}{
			"status":        d.Status,
			"attempts":      d.Attempts,
			"last_error":    d.LastError,
			"next_retry_at": d.NextRetryAt,
		}).Error
	if err != nil {
		return fmt.Errorf("更新投递记录失败: %w", err)
	}
	return nil
}

func (db *gor) ListDueDeliveries(now int64, limit int) ([]model.OpenWebhookDelivery, error) {
	var list []model.OpenWebhookDelivery
	err := db.db.Where("status = ? AND next_retry_at > 0 AND next_retry_at <= ?",
		model.DeliveryStatusPending, now).
		Order("next_retry_at asc").Limit(limit).Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("查询待重试投递失败: %w", err)
	}
	return list, nil
}
