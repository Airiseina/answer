package model

import "time"

// Bot 来源取值，与 bot_service/internal/model/bot.go 保持一致。
// origin=openapi（2）的 Bot 由用户自行部署，平台不参与调用，不写入用量。
const (
	BotOriginUser    int16 = 0 // 用户自建
	BotOriginMarket  int16 = 1 // 从市场安装
	BotOriginOpenAPI int16 = 2 // 通过 OpenAPI 接入（不统计）
)

// 配额归属类型。
const (
	QuotaOwnerUser   int16 = 0
	QuotaOwnerTenant int16 = 1
)

// 配额周期。
const (
	QuotaPeriodDaily int16 = 0
	QuotaPeriodMonth int16 = 1
)

// TokenUsage 是一条 LLM token 用量明细。
// MsgID 唯一索引用于把「Bot 回复消息」作为幂等去重键，避免重试导致重复计数。
// OpenAPI 接入（origin=2）的 Bot 不写入本表。
type TokenUsage struct {
	ID               int64     `gorm:"primarykey" json:"id"`
	BizDate          string    `gorm:"type:varchar(16);not null;index" json:"biz_date"`
	UserID           int64     `gorm:"not null;default:0;index" json:"user_id"`
	TenantID         int64     `gorm:"not null;default:0;index" json:"tenant_id"`
	BotID            int64     `gorm:"not null;default:0;index" json:"bot_id"`
	Origin           int16     `gorm:"type:tinyint;default:0;not null" json:"origin"`
	ConversationID   int64     `gorm:"not null;default:0;index" json:"conversation_id"`
	MsgID            int64     `gorm:"not null;default:0;uniqueIndex" json:"msg_id"`
	Model            string    `gorm:"type:varchar(128);not null;default:'';index" json:"model"`
	PromptTokens     int64     `gorm:"not null;default:0" json:"prompt_tokens"`
	CompletionTokens int64     `gorm:"not null;default:0" json:"completion_tokens"`
	// Cost 为按后台参考单价折算的预估成本，仅用于展示，不代表真实账单。
	Cost float64 `gorm:"not null;default:0" json:"cost"`
	// PriceMissing 标记写入时未命中参考单价（成本记 0），用于看板提示与缺失统计。
	PriceMissing bool      `gorm:"not null;default:false;index" json:"price_missing"`
	CreatedAt    time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TokenUsage) TableName() string {
	return "token_usage"
}

// ModelPrice 是参考单价表，由管理后台手工维护，仅用于把 token 折算成估算成本。
// Provider + Model 唯一；未配置单价时仍记录 token，成本记 0 并计入缺失统计。
type ModelPrice struct {
	ID          int64     `gorm:"primarykey" json:"id"`
	Provider    string    `gorm:"type:varchar(64);not null;default:''" json:"provider"`
	Model       string    `gorm:"type:varchar(128);not null;index" json:"model"`
	InputPer1K  float64   `gorm:"not null;default:0" json:"input_per_1k"`
	OutputPer1K float64   `gorm:"not null;default:0" json:"output_per_1k"`
	Currency    string    `gorm:"type:varchar(16);not null;default:'CNY'" json:"currency"`
	Enabled     bool      `gorm:"not null;default:true;index" json:"enabled"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ModelPrice) TableName() string {
	return "model_price"
}

// UsageQuota 是用户/租户的 token 用量上限（预留：本轮仅用于阈值告警，不做扣费）。
type UsageQuota struct {
	ID         int64     `gorm:"primarykey" json:"id"`
	OwnerType  int16     `gorm:"type:tinyint;not null;default:0" json:"owner_type"`
	OwnerID    int64     `gorm:"not null;default:0" json:"owner_id"`
	Period     int16     `gorm:"type:tinyint;not null;default:0" json:"period"`
	TokenLimit int64     `gorm:"not null;default:0" json:"token_limit"`
	Enabled    bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (UsageQuota) TableName() string {
	return "usage_quota"
}
