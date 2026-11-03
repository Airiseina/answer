package main

import (
	"context"

	"github.com/Airiseina/answer/kitex_service/billing_service/internal/dal/mysql"
	"github.com/Airiseina/answer/kitex_service/billing_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/billing_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/billing_service/kitex_gen/billing"

	"github.com/cloudwego/kitex/pkg/klog"
)

// BillingServiceImpl 把 thrift 请求翻译成 service 层调用，并回填响应。
type BillingServiceImpl struct {
	billingService *service.BillingService
}

// RecordUsage 记录一次 Bot 回复的 token 用量（按 msg_id 幂等）。
func (s *BillingServiceImpl) RecordUsage(ctx context.Context, req *billing.RecordUsageReq) (*billing.CommonRes, error) {
	recorded, err := s.billingService.RecordUsage(ctx, service.RecordUsageInput{
		UserID:           req.UserId,
		TenantID:         req.TenantId,
		BotID:            req.BotId,
		Origin:           req.Origin,
		ConversationID:   req.ConversationId,
		MsgID:            req.MsgId,
		Model:            req.Model,
		PromptTokens:     req.PromptTokens,
		CompletionTokens: req.CompletionTokens,
	})
	if err != nil {
		klog.CtxErrorf(ctx, "记录用量失败, bot_id=%d, msg_id=%d, err=%v", req.BotId, req.MsgId, err)
		return &billing.CommonRes{Success: false}, err
	}
	if !recorded {
		klog.CtxInfof(ctx, "用量未写入（跳过或命中幂等去重）, bot_id=%d, msg_id=%d", req.BotId, req.MsgId)
	}
	return &billing.CommonRes{Success: true}, nil
}

// GetUsageDetail 返回本人用量明细。
func (s *BillingServiceImpl) GetUsageDetail(ctx context.Context, req *billing.GetUsageDetailReq) (*billing.GetUsageDetailRes, error) {
	items, total, err := s.billingService.GetUsageDetail(ctx, req.UserId,
		req.GetStartDate(), req.GetEndDate(), req.GetModel(), req.GetBotId(),
		int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		klog.CtxErrorf(ctx, "查询用量明细失败, user_id=%d, err=%v", req.UserId, err)
		return nil, err
	}
	list := make([]*billing.TokenUsageInfo, 0, len(items))
	for _, u := range items {
		list = append(list, toTokenUsageInfo(u))
	}
	return &billing.GetUsageDetailRes{Success: true, Items: list, Total: total}, nil
}

// GetUsageSummary 汇总本人区间用量（合计、趋势、占比、缺失单价数）。
func (s *BillingServiceImpl) GetUsageSummary(ctx context.Context, req *billing.GetUsageSummaryReq) (*billing.GetUsageSummaryRes, error) {
	summary, err := s.billingService.GetUsageSummary(ctx, req.UserId, req.GetStartDate(), req.GetEndDate())
	if err != nil {
		klog.CtxErrorf(ctx, "查询用量汇总失败, user_id=%d, err=%v", req.UserId, err)
		return nil, err
	}
	trend := make([]*billing.UsageTrendPoint, 0, len(summary.Trend))
	for _, p := range summary.Trend {
		trend = append(trend, &billing.UsageTrendPoint{
			BizDate:          p.BizDate,
			PromptTokens:     p.PromptTokens,
			CompletionTokens: p.CompletionTokens,
			Cost:             p.Cost,
			RequestCount:     p.RequestCount,
		})
	}
	return &billing.GetUsageSummaryRes{
		Success:           true,
		PromptTokens:      summary.PromptTokens,
		CompletionTokens:  summary.CompletionTokens,
		Cost:              summary.Cost,
		RequestCount:      summary.RequestCount,
		Trend:             trend,
		ByBot:             toGroupStats(summary.ByBot),
		ByModel:           toGroupStats(summary.ByModel),
		MissingPriceCount: summary.MissingPrice,
	}, nil
}

// ListModelPrices 分页返回参考单价（管理后台）。
func (s *BillingServiceImpl) ListModelPrices(ctx context.Context, req *billing.ListModelPricesReq) (*billing.ListModelPricesRes, error) {
	prices, total, err := s.billingService.ListModelPrices(ctx, req.GetOnlyEnabled(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		klog.CtxErrorf(ctx, "查询参考单价失败: %v", err)
		return nil, err
	}
	list := make([]*billing.ModelPriceInfo, 0, len(prices))
	for _, p := range prices {
		list = append(list, toModelPriceInfo(p))
	}
	return &billing.ListModelPricesRes{Success: true, Prices: list, Total: total}, nil
}

// UpsertModelPrice 新增或更新参考单价（管理后台）。
func (s *BillingServiceImpl) UpsertModelPrice(ctx context.Context, req *billing.UpsertModelPriceReq) (*billing.CommonRes, error) {
	priceID, err := s.billingService.UpsertModelPrice(ctx, req.GetId(),
		req.Provider, req.Model, req.GetInputPer_1k(), req.GetOutputPer_1k(),
		req.GetCurrency(), req.Enabled, req.OperatorId)
	if err != nil {
		klog.CtxErrorf(ctx, "维护参考单价失败, model=%s, err=%v", req.Model, err)
		return &billing.CommonRes{Success: false}, err
	}
	klog.CtxInfof(ctx, "参考单价已保存, id=%d, model=%s", priceID, req.Model)
	return &billing.CommonRes{Success: true}, nil
}

// DeleteModelPrice 删除参考单价（管理后台）。
func (s *BillingServiceImpl) DeleteModelPrice(ctx context.Context, req *billing.DeleteModelPriceReq) (*billing.CommonRes, error) {
	if err := s.billingService.DeleteModelPrice(ctx, req.Id, req.OperatorId); err != nil {
		klog.CtxErrorf(ctx, "删除参考单价[%d]失败: %v", req.Id, err)
		return &billing.CommonRes{Success: false}, err
	}
	return &billing.CommonRes{Success: true}, nil
}

// ListUsageQuotas 查询用量配额（预留）。
func (s *BillingServiceImpl) ListUsageQuotas(ctx context.Context, req *billing.ListUsageQuotasReq) (*billing.ListUsageQuotasRes, error) {
	quotas, err := s.billingService.ListUsageQuotas(ctx, req.GetOwnerType(), req.GetOwnerId())
	if err != nil {
		klog.CtxErrorf(ctx, "查询用量配额失败: %v", err)
		return nil, err
	}
	list := make([]*billing.UsageQuotaInfo, 0, len(quotas))
	for _, q := range quotas {
		list = append(list, &billing.UsageQuotaInfo{
			Id:         q.ID,
			OwnerType:  q.OwnerType,
			OwnerId:    q.OwnerID,
			Period:     q.Period,
			TokenLimit: q.TokenLimit,
			Enabled:    q.Enabled,
		})
	}
	return &billing.ListUsageQuotasRes{Success: true, Quotas: list}, nil
}

// SetUsageQuota 写入用量配额（预留）。
func (s *BillingServiceImpl) SetUsageQuota(ctx context.Context, req *billing.SetUsageQuotaReq) (*billing.CommonRes, error) {
	if err := s.billingService.SetUsageQuota(ctx, req.OwnerType, req.OwnerId, req.Period, req.TokenLimit, req.Enabled); err != nil {
		klog.CtxErrorf(ctx, "写入用量配额失败: %v", err)
		return &billing.CommonRes{Success: false}, err
	}
	return &billing.CommonRes{Success: true}, nil
}

func toTokenUsageInfo(u model.TokenUsage) *billing.TokenUsageInfo {
	return &billing.TokenUsageInfo{
		Id:               u.ID,
		BizDate:          u.BizDate,
		UserId:           u.UserID,
		TenantId:         u.TenantID,
		BotId:            u.BotID,
		Origin:           u.Origin,
		ConversationId:   u.ConversationID,
		MsgId:            u.MsgID,
		Model:            u.Model,
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		Cost:             u.Cost,
		CreatedAt:        u.CreatedAt.UnixMilli(),
	}
}

func toGroupStats(rows []mysql.GroupRow) []*billing.UsageGroupStat {
	list := make([]*billing.UsageGroupStat, 0, len(rows))
	for _, r := range rows {
		list = append(list, &billing.UsageGroupStat{
			Key:              r.Key,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
			Cost:             r.Cost,
			RequestCount:     r.RequestCount,
		})
	}
	return list
}

func toModelPriceInfo(p model.ModelPrice) *billing.ModelPriceInfo {
	return &billing.ModelPriceInfo{
		Id:           p.ID,
		Provider:     p.Provider,
		Model:        p.Model,
		InputPer_1k:  p.InputPer1K,
		OutputPer_1k: p.OutputPer1K,
		Currency:     p.Currency,
		Enabled:      p.Enabled,
		CreatedAt:    p.CreatedAt.UnixMilli(),
		UpdatedAt:    p.UpdatedAt.UnixMilli(),
	}
}
