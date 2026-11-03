package mysql

import (
	"errors"

	"github.com/Airiseina/answer/kitex_service/billing_service/internal/model"

	"gorm.io/gorm"
)

// ErrDuplicateUsage 表示同一 msg_id 的用量已存在（幂等去重命中）。
var ErrDuplicateUsage = errors.New("用量记录已存在")

// TokenTotals 是一次汇总的合计值。
type TokenTotals struct {
	PromptTokens     int64
	CompletionTokens int64
	Cost             float64
	RequestCount     int64
}

// TrendRow 是按天的用量趋势行。
type TrendRow struct {
	BizDate          string
	PromptTokens     int64
	CompletionTokens int64
	Cost             float64
	RequestCount     int64
}

// GroupRow 是按 Bot 或模型聚合的占比行。
type GroupRow struct {
	Key              string
	PromptTokens     int64
	CompletionTokens int64
	Cost             float64
	RequestCount     int64
}

type gor struct {
	db *gorm.DB
}

// NewBillingDao 基于 GORM 创建用量/单价/配额的读写入口。
func NewBillingDao(db *gorm.DB) BillingDao {
	return &gor{db: db}
}

// BillingDao 汇集 billing_service 三张表的读写，避免 service 层直接接触 GORM。
type BillingDao interface {
	// 用量明细
	CreateUsage(u *model.TokenUsage) error
	GetUsageByMsgID(msgID int64) (model.TokenUsage, bool, error)
	ListUsage(userID int64, startDate, endDate, modelName string, botID int64, limit, offset int) ([]model.TokenUsage, int64, error)
	AggregateTotals(userID int64, startDate, endDate string) (TokenTotals, error)
	AggregateTrend(userID int64, startDate, endDate string) ([]TrendRow, error)
	AggregateByBot(userID int64, startDate, endDate string) ([]GroupRow, error)
	AggregateByModel(userID int64, startDate, endDate string) ([]GroupRow, error)
	CountMissingPrice(userID int64, startDate, endDate string) (int64, error)

	// 参考单价
	ListModelPrices(onlyEnabled bool, limit, offset int) ([]model.ModelPrice, int64, error)
	GetModelPriceByModel(modelName string) (model.ModelPrice, bool, error)
	GetModelPriceByID(id int64) (model.ModelPrice, bool, error)
	CreateModelPrice(p *model.ModelPrice) error
	UpdateModelPrice(p *model.ModelPrice) error
	DeleteModelPrice(id int64) error

	// 用量配额
	ListUsageQuotas(ownerType int16, ownerID int64) ([]model.UsageQuota, error)
	UpsertUsageQuota(q *model.UsageQuota) error
}
