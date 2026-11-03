package model

import "time"

// Bot 来源取值，与 bot_service/internal/model/bot.go 保持一致。
// 开放平台创建/管理的 Bot 固定为 BotOriginOpenAPI，并据此判定是否计入计费。
const (
	BotOriginUser    int16 = 0 // 用户自建
	BotOriginMarket  int16 = 1 // 从市场安装
	BotOriginOpenAPI int16 = 2 // 通过 OpenAPI 接入（不计入用量统计）
)

// 应用状态。
const (
	AppStatusEnabled  int16 = 0
	AppStatusDisabled int16 = 1
)

// 事件状态：0 待投递 1 已投递 2 已确认 3 死信。
const (
	EventStatusPending  int16 = 0
	EventStatusDelivered int16 = 1
	EventStatusAcked    int16 = 2
	EventStatusDead     int16 = 3
)

// 回调投递状态：0 待投递 1 成功 2 死信。
const (
	DeliveryStatusPending int16 = 0
	DeliveryStatusSuccess int16 = 1
	DeliveryStatusDead    int16 = 2
)

// 事件类型（对外文档需同步）。
const (
	EventMessageReceived = "message.received" // 有人给 Bot 发消息
	EventMessageReply    = "message.reply"    // Bot 生成回复
)

// OpenApp 是开放平台应用（AppID + AppSecret）。
//
// AppSecretHash 只存 SHA-256 摘要，用于「重置后旧密钥立即失效」的快速比对；
// AppSecretCipher 存 AES-GCM 密文，供 HMAC 验签时还原明文（哈希无法用于验签）。
// 明文仅在创建/重置响应中返回一次，之后不再落地。
type OpenApp struct {
	ID            int64     `gorm:"primarykey" json:"id"`
	TenantID      int64     `gorm:"not null;default:0;index" json:"tenant_id"`
	OwnerID       int64     `gorm:"not null;index" json:"owner_id"`
	Name          string    `gorm:"type:varchar(64);not null" json:"name"`
	AppID         string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"app_id"`
	AppSecretHash string    `gorm:"type:varchar(128);not null" json:"-"`
	AppSecretCipher string  `gorm:"type:varchar(256);not null" json:"-"`
	Scopes        string    `gorm:"type:varchar(512);not null" json:"scopes"`
	Status        int16     `gorm:"type:tinyint;default:0;not null;index" json:"status"`
	QPSLimit      int32     `gorm:"default:0" json:"qps_limit"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (OpenApp) TableName() string {
	return "open_app"
}

// OpenWebhook Bot 回调地址（可选，多个 App 可注册多条）。
type OpenWebhook struct {
	ID        int64     `gorm:"primarykey" json:"id"`
	AppID     string    `gorm:"type:varchar(64);not null;index" json:"app_id"`
	BotID     int64     `gorm:"not null;default:0;index" json:"bot_id"`
	URL       string    `gorm:"type:varchar(512);not null" json:"url"`
	Secret    string    `gorm:"type:varchar(128);not null" json:"-"`
	Events    string    `gorm:"type:varchar(512);not null;default:''" json:"events"`
	Enabled   bool      `gorm:"default:true;index" json:"enabled"`
	Status    string    `gorm:"type:varchar(32);default:'pending'" json:"status"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (OpenWebhook) TableName() string {
	return "open_webhook"
}

// OpenEvent 是轮询与回调共用的事件源头，保证不丢。
// ID 使用自增，天然满足游标分页的单调递增。
type OpenEvent struct {
	ID        int64     `gorm:"primarykey;autoIncrement" json:"id"`
	AppID     string    `gorm:"type:varchar(64);not null;index:idx_open_event_app,priority:1" json:"app_id"`
	BotID     int64     `gorm:"not null;default:0;index" json:"bot_id"`
	EventType string    `gorm:"type:varchar(64);not null;index" json:"event_type"`
	Payload   string    `gorm:"type:text" json:"payload"`
	Status    int16     `gorm:"type:tinyint;default:0;not null;index" json:"status"`
	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (OpenEvent) TableName() string {
	return "open_event"
}

// OpenWebhookDelivery 记录每次回调投递，含重试与死信。
type OpenWebhookDelivery struct {
	ID          int64     `gorm:"primarykey;autoIncrement" json:"id"`
	AppID       string    `gorm:"type:varchar(64);not null;index" json:"app_id"`
	WebhookID   int64     `gorm:"not null;index" json:"webhook_id"`
	EventID     int64     `gorm:"not null;index" json:"event_id"`
	Status      int16     `gorm:"type:tinyint;default:0;not null;index" json:"status"`
	Attempts    int32     `gorm:"default:0" json:"attempts"`
	LastError   string    `gorm:"type:varchar(512);default:''" json:"last_error"`
	NextRetryAt int64     `gorm:"default:0;index" json:"next_retry_at"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (OpenWebhookDelivery) TableName() string {
	return "open_webhook_delivery"
}
