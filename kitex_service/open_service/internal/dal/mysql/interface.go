package mysql

import (
	"github.com/Airiseina/answer/kitex_service/open_service/internal/model"

	"gorm.io/gorm"
)

type gor struct {
	db *gorm.DB
}

func NewOpenDao(db *gorm.DB) OpenDao {
	return &gor{db: db}
}

// OpenDao 汇集开放平台四张表的读写入口，避免 service 层直接接触 GORM。
type OpenDao interface {
	// 应用
	CreateApp(app *model.OpenApp) error
	GetAppByAppID(appID string) (model.OpenApp, error)
	ListApps(ownerID, tenantID int64) ([]model.OpenApp, error)
	UpdateAppSecret(appID, secretHash, secretCipher string) error
	UpdateAppStatus(appID string, status int16) error

	// 回调
	CreateWebhook(wh *model.OpenWebhook) error
	ListWebhooks(appID string, botID int64) ([]model.OpenWebhook, error)
	ListEnabledWebhooks(appID string, eventType string) ([]model.OpenWebhook, error)
	GetWebhook(appID string, id int64) (model.OpenWebhook, error)
	UpdateWebhookStatus(id int64, status string) error
	DeleteWebhook(appID string, id int64) error

	// 事件
	AppendEvent(ev *model.OpenEvent) error
	ListEvents(appID string, sinceID int64, limit int) ([]model.OpenEvent, error)
	AckEvents(appID string, ids []int64) error
	MarkEventsDelivered(ids []int64) error
	PurgeEventsBefore(ts int64) (int64, error)

	// 投递
	CreateDelivery(d *model.OpenWebhookDelivery) error
	GetDeliveryByEventWebhook(webhookID, eventID int64) (model.OpenWebhookDelivery, error)
	UpdateDelivery(d *model.OpenWebhookDelivery) error
	ListDueDeliveries(now int64, limit int) ([]model.OpenWebhookDelivery, error)
}
