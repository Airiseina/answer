package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Airiseina/answer/kitex_service/billing_service/internal/dal/mysql"
	"github.com/Airiseina/answer/kitex_service/billing_service/internal/model"
	"github.com/Airiseina/answer/pkg/audit"
	"github.com/Airiseina/answer/pkg/observability/meter"
	"github.com/Airiseina/answer/pkg/snowflake"

	"github.com/cloudwego/kitex/pkg/klog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// DefaultTimezone 用量按东八区切分自然日，与日/周总结保持一致。
const DefaultTimezone = "Asia/Shanghai"

// 单价默认币种。
const defaultCurrency = "CNY"

// RecordUsageInput 是一次用量上报的入参（由 work_service 组装）。
type RecordUsageInput struct {
	UserID           int64
	TenantID         int64
	BotID            int64
	Origin           int16
	ConversationID   int64
	MsgID            int64
	Model            string
	PromptTokens     int64
	CompletionTokens int64
}

// BillingService 承载用量记账、参考单价维护与配额管理。
type BillingService struct {
	dao       mysql.BillingDao
	recorder  audit.Recorder
	snowNode  *snowflake.Node
	timezone  *time.Location
}

func NewBillingService(dao mysql.BillingDao, recorder audit.Recorder) *BillingService {
	return &BillingService{
		dao:      dao,
		recorder: recorder,
		snowNode: snowflake.NewNode(snowflake.ShardBilling),
		timezone: loadTimezone(),
	}
}

func loadTimezone() *time.Location {
	if loc, err := time.LoadLocation(DefaultTimezone); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*60*60)
}

// bizDate 返回当前时刻在东八区的 yyyy-mm-dd。
func (s *BillingService) bizDate() string {
	return time.Now().In(s.timezone).Format("2006-01-02")
}

// RecordUsage 记录一次 Bot 回复的 token 用量。
// 返回 recorded=false 表示未写入（OpenAPI 接入 / 无 token / 命中幂等去重），两者都不视为错误。
func (s *BillingService) RecordUsage(ctx context.Context, in RecordUsageInput) (bool, error) {
	// OpenAPI 接入的 Bot 跑在用户自己的服务里，平台不参与调用，直接跳过。
	if in.Origin == model.BotOriginOpenAPI {
		return false, nil
	}
	if in.PromptTokens+in.CompletionTokens <= 0 {
		return false, nil
	}
	if in.UserID <= 0 || in.MsgID <= 0 {
		return false, fmt.Errorf("用量上报缺少用户或消息标识: user_id=%d, msg_id=%d", in.UserID, in.MsgID)
	}

	cost, missing := s.calcCost(ctx, in.Model, in.PromptTokens, in.CompletionTokens)
	usage := &model.TokenUsage{
		ID:               s.snowNode.Generate(),
		BizDate:          s.bizDate(),
		UserID:           in.UserID,
		TenantID:         in.TenantID,
		BotID:            in.BotID,
		Origin:           in.Origin,
		ConversationID:   in.ConversationID,
		MsgID:            in.MsgID,
		Model:            in.Model,
		PromptTokens:     in.PromptTokens,
		CompletionTokens: in.CompletionTokens,
		Cost:             cost,
		PriceMissing:     missing,
	}
	if err := s.dao.CreateUsage(usage); err != nil {
		if errors.Is(err, mysql.ErrDuplicateUsage) {
			// 重试或重复投递：按 msg_id 幂等去重，静默成功。
			return false, nil
		}
		return false, fmt.Errorf("写入用量明细失败: %w", err)
	}

	// 指标接线：token 消耗量 + 预估成本累计。
	if meter.M != nil {
		total := in.PromptTokens + in.CompletionTokens
		meter.M.BotTokenUsage.Add(ctx, total, metric.WithAttributes(
			attribute.Int64("bot_id", in.BotID),
			attribute.String("model", in.Model),
		))
		if cost > 0 {
			meter.M.BillingCostTotal.Add(ctx, cost, metric.WithAttributes(
				attribute.Int64("bot_id", in.BotID),
				attribute.String("model", in.Model),
			))
		}
	}
	return true, nil
}

// calcCost 按参考单价折算预估成本；未命中启用单价时成本记 0 并标记缺失。
func (s *BillingService) calcCost(ctx context.Context, modelName string, prompt, completion int64) (cost float64, missing bool) {
	if modelName == "" {
		klog.CtxWarnf(ctx, "用量上报缺少模型名，预估成本记 0")
		return 0, true
	}
	price, ok, err := s.dao.GetModelPriceByModel(modelName)
	if err != nil {
		klog.CtxWarnf(ctx, "查询模型[%s]参考单价失败: %v", modelName, err)
		return 0, true
	}
	if !ok {
		klog.CtxWarnf(ctx, "模型[%s]未配置参考单价，仅记录 token、成本记 0", modelName)
		return 0, true
	}
	// cost = prompt/1000 × input_per_1k + completion/1000 × output_per_1k
	return float64(prompt)/1000*price.InputPer1K + float64(completion)/1000*price.OutputPer1K, false
}

// GetUsageDetail 返回本人用量明细（user_id 由网关强制传入，禁止跨用户查询）。
func (s *BillingService) GetUsageDetail(ctx context.Context, userID int64, startDate, endDate, modelName string, botID int64, limit, offset int) ([]model.TokenUsage, int64, error) {
	if userID <= 0 {
		return nil, 0, fmt.Errorf("用量查询缺少用户标识")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.dao.ListUsage(userID, startDate, endDate, modelName, botID, limit, offset)
}

// UsageSummary 是本人用量的汇总视图。
type UsageSummary struct {
	PromptTokens     int64
	CompletionTokens int64
	Cost             float64
	RequestCount     int64
	Trend            []mysql.TrendRow
	ByBot            []mysql.GroupRow
	ByModel          []mysql.GroupRow
	MissingPrice     int64
}

// GetUsageSummary 汇总本人区间用量：合计、按天趋势、按 Bot/模型占比、缺失单价数。
func (s *BillingService) GetUsageSummary(ctx context.Context, userID int64, startDate, endDate string) (*UsageSummary, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用量查询缺少用户标识")
	}
	totals, err := s.dao.AggregateTotals(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("汇总用量失败: %w", err)
	}
	trend, err := s.dao.AggregateTrend(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("汇总用量趋势失败: %w", err)
	}
	byBot, err := s.dao.AggregateByBot(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("汇总 Bot 占比失败: %w", err)
	}
	byModel, err := s.dao.AggregateByModel(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("汇总模型占比失败: %w", err)
	}
	missing, err := s.dao.CountMissingPrice(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("统计缺失单价失败: %w", err)
	}
	return &UsageSummary{
		PromptTokens:     totals.PromptTokens,
		CompletionTokens: totals.CompletionTokens,
		Cost:             totals.Cost,
		RequestCount:     totals.RequestCount,
		Trend:            trend,
		ByBot:            byBot,
		ByModel:          byModel,
		MissingPrice:     missing,
	}, nil
}

// ===== 参考单价（管理后台手工维护）=====

func (s *BillingService) ListModelPrices(ctx context.Context, onlyEnabled bool, limit, offset int) ([]model.ModelPrice, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.dao.ListModelPrices(onlyEnabled, limit, offset)
}

// UpsertModelPrice 新增或更新参考单价，并写入审计日志。
func (s *BillingService) UpsertModelPrice(ctx context.Context, id int64, provider, modelName string, inputPer1K, outputPer1K float64, currency string, enabled bool, operatorID int64) (int64, error) {
	if modelName == "" {
		return 0, fmt.Errorf("模型名不能为空")
	}
	if currency == "" {
		currency = defaultCurrency
	}
	var priceID int64
	if id > 0 {
		existing, ok, err := s.dao.GetModelPriceByID(id)
		if err != nil {
			return 0, fmt.Errorf("查询参考单价失败: %w", err)
		}
		if !ok {
			return 0, fmt.Errorf("参考单价[%d]不存在", id)
		}
		existing.Provider = provider
		existing.Model = modelName
		existing.InputPer1K = inputPer1K
		existing.OutputPer1K = outputPer1K
		existing.Currency = currency
		existing.Enabled = enabled
		if err = s.dao.UpdateModelPrice(&existing); err != nil {
			return 0, fmt.Errorf("更新参考单价失败: %w", err)
		}
		priceID = existing.ID
	} else {
		price := &model.ModelPrice{
			ID:          s.snowNode.Generate(),
			Provider:    provider,
			Model:       modelName,
			InputPer1K:  inputPer1K,
			OutputPer1K: outputPer1K,
			Currency:    currency,
			Enabled:     enabled,
		}
		if err := s.dao.CreateModelPrice(price); err != nil {
			return 0, fmt.Errorf("新增参考单价失败: %w", err)
		}
		priceID = price.ID
	}
	s.recordAudit(ctx, "upsert", priceID, operatorID, fmt.Sprintf("provider=%s model=%s in=%.4f out=%.4f currency=%s enabled=%v", provider, modelName, inputPer1K, outputPer1K, currency, enabled))
	return priceID, nil
}

func (s *BillingService) DeleteModelPrice(ctx context.Context, id, operatorID int64) error {
	if id <= 0 {
		return fmt.Errorf("参考单价 id 不合法")
	}
	if err := s.dao.DeleteModelPrice(id); err != nil {
		return fmt.Errorf("删除参考单价失败: %w", err)
	}
	s.recordAudit(ctx, "delete", id, operatorID, "")
	return nil
}

// recordAudit 记录参考单价维护动作，失败只告警不阻断主流程。
func (s *BillingService) recordAudit(ctx context.Context, action string, priceID, operatorID int64, detail string) {
	if s.recorder == nil {
		return
	}
	if err := s.recorder.Record(ctx, audit.Event{
		EventType:  audit.EventModelPrice,
		ActorType:  audit.ActorAdmin,
		ActorID:    operatorID,
		Action:     action,
		TargetType: "model_price",
		TargetID:   fmt.Sprintf("%d", priceID),
		Detail:     detail,
		Result:     audit.ResultSuccess,
	}); err != nil {
		klog.CtxWarnf(ctx, "写入参考单价审计日志失败: %v", err)
	}
}

// ===== 用量配额（预留）=====

func (s *BillingService) ListUsageQuotas(ctx context.Context, ownerType int16, ownerID int64) ([]model.UsageQuota, error) {
	return s.dao.ListUsageQuotas(ownerType, ownerID)
}

func (s *BillingService) SetUsageQuota(ctx context.Context, ownerType int16, ownerID int64, period int16, tokenLimit int64, enabled bool) error {
	if ownerID <= 0 {
		return fmt.Errorf("配额归属 id 不合法")
	}
	quota := &model.UsageQuota{
		ID:         s.snowNode.Generate(),
		OwnerType:  ownerType,
		OwnerID:    ownerID,
		Period:     period,
		TokenLimit: tokenLimit,
		Enabled:    enabled,
	}
	if err := s.dao.UpsertUsageQuota(quota); err != nil {
		return fmt.Errorf("写入用量配额失败: %w", err)
	}
	return nil
}
