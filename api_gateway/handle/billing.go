package handle

import (
	"context"
	"strconv"

	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"
	"github.com/Airiseina/answer/kitex_service/billing_service/kitex_gen/billing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// usageDetailItem 是一条用量明细对外的视图。
type usageDetailItem struct {
	Id               string  `json:"id"`
	BizDate          string  `json:"biz_date"`
	BotId            string  `json:"bot_id"`
	Origin           int16   `json:"origin"`
	ConversationId   string  `json:"conversation_id"`
	MsgId            string  `json:"msg_id"`
	Model            string  `json:"model"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
	CreatedAt        int64   `json:"created_at"`
}

func toUsageDetailItems(items []*billing.TokenUsageInfo) []usageDetailItem {
	list := make([]usageDetailItem, 0, len(items))
	for _, u := range items {
		list = append(list, usageDetailItem{
			Id:               strconv.FormatInt(u.Id, 10),
			BizDate:          u.BizDate,
			BotId:            strconv.FormatInt(u.BotId, 10),
			Origin:           u.Origin,
			ConversationId:   strconv.FormatInt(u.ConversationId, 10),
			MsgId:            strconv.FormatInt(u.MsgId, 10),
			Model:            u.Model,
			PromptTokens:     u.PromptTokens,
			CompletionTokens: u.CompletionTokens,
			Cost:             u.Cost,
			CreatedAt:        u.CreatedAt,
		})
	}
	return list
}

// currentBillingUserID 读取当前登录用户 ID；未登录返回 0。
// 用量看板仅本人可见，查询一律以该 ID 强制过滤，不接受前端传入的 user_id。
func currentBillingUserID(c *app.RequestContext) int64 {
	identity, ok := middleware.CurrentUser(c)
	if !ok || identity == nil {
		return 0
	}
	return identity.Id
}

// GetUsageDetail 返回本人用量明细（按日期/模型/Bot 过滤）。
func GetUsageDetail(ctx context.Context, c *app.RequestContext) {
	userID := currentBillingUserID(c)
	if userID <= 0 {
		response.Error(c, "未登录", "登录态失效")
		return
	}
	req := &billing.GetUsageDetailReq{UserId: userID}
	if v := c.Query("start_date"); v != "" {
		req.StartDate = &v
	}
	if v := c.Query("end_date"); v != "" {
		req.EndDate = &v
	}
	if v := c.Query("model"); v != "" {
		req.Model = &v
	}
	if raw := c.Query("bot_id"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil && v > 0 {
			req.BotId = &v
		}
	}
	if raw := c.Query("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			l := int32(v)
			req.Limit = &l
		}
	}
	if raw := c.Query("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			o := int32(v)
			req.Offset = &o
		}
	}
	resp, err := rpc.GetUsageDetail(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC GetUsageDetail失败: %v", err)
		response.Error(c, "系统繁忙", "查询用量明细失败")
		return
	}
	response.Success(c, map[string]interface{}{
		"items": toUsageDetailItems(resp.Items),
		"total": resp.Total,
	})
}

// GetUsageSummary 返回本人用量汇总（合计、按天趋势、按 Bot/模型占比、预估成本）。
func GetUsageSummary(ctx context.Context, c *app.RequestContext) {
	userID := currentBillingUserID(c)
	if userID <= 0 {
		response.Error(c, "未登录", "登录态失效")
		return
	}
	req := &billing.GetUsageSummaryReq{UserId: userID}
	if v := c.Query("start_date"); v != "" {
		req.StartDate = &v
	}
	if v := c.Query("end_date"); v != "" {
		req.EndDate = &v
	}
	resp, err := rpc.GetUsageSummary(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC GetUsageSummary失败: %v", err)
		response.Error(c, "系统繁忙", "查询用量汇总失败")
		return
	}
	trend := make([]map[string]interface{}, 0, len(resp.Trend))
	for _, p := range resp.Trend {
		trend = append(trend, map[string]interface{}{
			"biz_date":          p.BizDate,
			"prompt_tokens":     p.PromptTokens,
			"completion_tokens": p.CompletionTokens,
			"cost":              p.Cost,
			"request_count":     p.RequestCount,
		})
	}
	response.Success(c, map[string]interface{}{
		"prompt_tokens":       resp.PromptTokens,
		"completion_tokens":   resp.CompletionTokens,
		"cost":                resp.Cost,
		"request_count":       resp.RequestCount,
		"trend":               trend,
		"by_bot":              toUsageGroupStats(resp.ByBot),
		"by_model":            toUsageGroupStats(resp.ByModel),
		"missing_price_count": resp.MissingPriceCount,
	})
}

func toUsageGroupStats(stats []*billing.UsageGroupStat) []map[string]interface{} {
	list := make([]map[string]interface{}, 0, len(stats))
	for _, s := range stats {
		list = append(list, map[string]interface{}{
			"key":               s.Key,
			"prompt_tokens":     s.PromptTokens,
			"completion_tokens": s.CompletionTokens,
			"cost":              s.Cost,
			"request_count":     s.RequestCount,
		})
	}
	return list
}

// ===== 管理后台：参考单价维护 =====

// modelPriceItem 是一条参考单价的对外视图。
type modelPriceItem struct {
	Id          string  `json:"id"`
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	InputPer1K  float64 `json:"input_per_1k"`
	OutputPer1K float64 `json:"output_per_1k"`
	Currency    string  `json:"currency"`
	Enabled     bool    `json:"enabled"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
}

func toModelPriceItems(prices []*billing.ModelPriceInfo) []modelPriceItem {
	list := make([]modelPriceItem, 0, len(prices))
	for _, p := range prices {
		list = append(list, modelPriceItem{
			Id:          strconv.FormatInt(p.Id, 10),
			Provider:    p.Provider,
			Model:       p.Model,
			InputPer1K:  p.InputPer_1k,
			OutputPer1K: p.OutputPer_1k,
			Currency:    p.Currency,
			Enabled:     p.Enabled,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}
	return list
}

// AdminListModelPrices 管理后台查询参考单价列表（仅供管理员维护，不影响用户可见范围）。
func AdminListModelPrices(ctx context.Context, c *app.RequestContext) {
	if _, ok := middleware.CurrentAdmin(c); !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	req := &billing.ListModelPricesReq{}
	if v := c.Query("only_enabled"); v == "1" || v == "true" {
		onlyEnabled := true
		req.OnlyEnabled = &onlyEnabled
	}
	if raw := c.Query("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			l := int32(v)
			req.Limit = &l
		}
	}
	if raw := c.Query("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			o := int32(v)
			req.Offset = &o
		}
	}
	resp, err := rpc.ListModelPrices(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC ListModelPrices失败: %v", err)
		response.Error(c, "系统繁忙", "查询参考单价失败")
		return
	}
	response.Success(c, map[string]interface{}{
		"prices": toModelPriceItems(resp.Prices),
		"total":  resp.Total,
	})
}

// upsertModelPriceReq 是管理后台新增/更新参考单价的请求体。
type upsertModelPriceReq struct {
	Id          string  `json:"id"`
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	InputPer1K  float64 `json:"input_per_1k"`
	OutputPer1K float64 `json:"output_per_1k"`
	Currency    string  `json:"currency"`
	Enabled     *bool   `json:"enabled"`
}

// AdminUpsertModelPrice 管理后台新增或更新一条参考单价。
func AdminUpsertModelPrice(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var body upsertModelPriceReq
	if err := c.BindJSON(&body); err != nil {
		response.Error(c, "参数错误", "请求格式不正确")
		return
	}
	if body.Model == "" {
		response.Error(c, "参数错误", "模型名不能为空")
		return
	}
	req := &billing.UpsertModelPriceReq{
		Provider:    body.Provider,
		Model:       body.Model,
		InputPer_1k: body.InputPer1K,
		OutputPer_1k: body.OutputPer1K,
		Enabled:     true,
		OperatorId:  identity.Id,
	}
	if body.Id != "" {
		if id, err := strconv.ParseInt(body.Id, 10, 64); err == nil {
			req.Id = &id
		}
	}
	if body.Currency != "" {
		req.Currency = &body.Currency
	}
	if body.Enabled != nil {
		req.Enabled = *body.Enabled
	}
	if _, err := rpc.UpsertModelPrice(ctx, req); err != nil {
		hlog.CtxErrorf(ctx, "RPC UpsertModelPrice失败: %v", err)
		response.Error(c, "保存失败", "参考单价保存失败")
		return
	}
	response.Success(c, map[string]interface{}{"success": true})
}

// AdminDeleteModelPrice 管理后台删除一条参考单价。
func AdminDeleteModelPrice(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var body struct {
		Id string `json:"id"`
	}
	if err := c.BindJSON(&body); err != nil {
		response.Error(c, "参数错误", "请求格式不正确")
		return
	}
	id, err := strconv.ParseInt(body.Id, 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, "参数错误", "id格式不正确")
		return
	}
	if _, err := rpc.DeleteModelPrice(ctx, &billing.DeleteModelPriceReq{Id: id, OperatorId: identity.Id}); err != nil {
		hlog.CtxErrorf(ctx, "RPC DeleteModelPrice失败: %v", err)
		response.Error(c, "删除失败", "参考单价删除失败")
		return
	}
	response.Success(c, map[string]interface{}{"success": true})
}
