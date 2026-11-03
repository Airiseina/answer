package handle

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"

	bot "github.com/Airiseina/answer/kitex_service/bot_service/kitex_gen/bot"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// marketBotItem 是市场条目对外的统一视图。
type marketBotItem struct {
	MarketBotId   string  `json:"market_bot_id"`
	PublisherId   string  `json:"publisher_id"`
	PublisherName string  `json:"publisher_name"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	AvatarUrl     string  `json:"avatar_url"`
	SystemPrompt  string  `json:"system_prompt"`
	Model         string  `json:"model"`
	BaseUrl       *string `json:"base_url,omitempty"`
	Category      string  `json:"category"`
	Tags          string  `json:"tags"`
	Status        int16   `json:"status"`
	ReviewNote    string  `json:"review_note"`
	InstallCount  int64   `json:"install_count"`
	Rating        float64 `json:"rating"`
	RatingCount   int32   `json:"rating_count"`
	Version       int32   `json:"version"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

func toMarketBotItem(info *bot.MarketBotInfo) marketBotItem {
	return marketBotItem{
		MarketBotId:   fmt.Sprintf("%d", info.MarketBotId),
		PublisherId:   fmt.Sprintf("%d", info.PublisherId),
		PublisherName: info.PublisherName,
		Name:          info.Name,
		Description:   info.Description,
		AvatarUrl:     info.AvatarUrl,
		SystemPrompt:  info.SystemPrompt,
		Model:         info.Model,
		BaseUrl:       info.BaseUrl,
		Category:      info.Category,
		Tags:          info.Tags,
		Status:        info.Status,
		ReviewNote:    info.ReviewNote,
		InstallCount:  info.InstallCount,
		Rating:        info.Rating,
		RatingCount:   info.RatingCount,
		Version:       info.Version,
		CreatedAt:     fmt.Sprintf("%d", info.CreatedAt),
		UpdatedAt:     fmt.Sprintf("%d", info.UpdatedAt),
	}
}

func toMarketBotItems(infos []*bot.MarketBotInfo) []marketBotItem {
	items := make([]marketBotItem, 0, len(infos))
	for _, info := range infos {
		items = append(items, toMarketBotItem(info))
	}
	return items
}

type publishMarketBotReq struct {
	Name         string `json:"name" vd:"len($) > 0"`
	SystemPrompt string `json:"system_prompt" vd:"len($) > 0"`
	Description  string `json:"description"`
	AvatarUrl    string `json:"avatar_url"`
	Model        string `json:"model"`
	BaseUrl      string `json:"base_url"`
	Category     string `json:"category"`
	Tags         string `json:"tags"`
}

// PublishMarketBot 发布自己编写的 Bot 到市场，进入待审核。
func PublishMarketBot(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var req publishMarketBotReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	rpcReq := &bot.PublishMarketBotReq{
		TenantId:     identity.TenantId,
		PublisherId:  identity.Id,
		Name:         req.Name,
		SystemPrompt: req.SystemPrompt,
		Description:  req.Description,
	}
	if req.AvatarUrl != "" {
		rpcReq.AvatarUrl = &req.AvatarUrl
	}
	if req.Model != "" {
		rpcReq.Model = &req.Model
	}
	if req.BaseUrl != "" {
		rpcReq.BaseUrl = &req.BaseUrl
	}
	if req.Category != "" {
		rpcReq.Category = &req.Category
	}
	if req.Tags != "" {
		rpcReq.Tags = &req.Tags
	}
	resp, err := rpc.PublishMarketBot(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "发布市场Bot失败: %v", err)
		response.Error(c, "发布失败", err.Error())
		return
	}
	response.Success(c, map[string]interface{}{
		"market_bot_id": fmt.Sprintf("%d", resp.MarketBotId),
		"status":        resp.Status,
	})
}

type updateMarketBotReq struct {
	MarketBotId  int64  `json:"market_bot_id,string" vd:"$ > 0"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	AvatarUrl    string `json:"avatar_url"`
	SystemPrompt string `json:"system_prompt"`
	Model        string `json:"model"`
	BaseUrl      string `json:"base_url"`
	Category     string `json:"category"`
	Tags         string `json:"tags"`
}

// UpdateMarketBot 发布者修改自己的市场条目；改动提示词会重新进入待审核。
func UpdateMarketBot(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var req updateMarketBotReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	rpcReq := &bot.UpdateMarketBotReq{
		TenantId:    identity.TenantId,
		MarketBotId: req.MarketBotId,
		OperatorId:  identity.Id,
	}
	if req.Name != "" {
		rpcReq.Name = &req.Name
	}
	if req.Description != "" {
		rpcReq.Description = &req.Description
	}
	if req.AvatarUrl != "" {
		rpcReq.AvatarUrl = &req.AvatarUrl
	}
	if req.SystemPrompt != "" {
		rpcReq.SystemPrompt = &req.SystemPrompt
	}
	if req.Model != "" {
		rpcReq.Model = &req.Model
	}
	if req.BaseUrl != "" {
		rpcReq.BaseUrl = &req.BaseUrl
	}
	if req.Category != "" {
		rpcReq.Category = &req.Category
	}
	if req.Tags != "" {
		rpcReq.Tags = &req.Tags
	}
	resp, err := rpc.UpdateMarketBot(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "更新市场Bot失败: %v", err)
		response.Error(c, "更新失败", err.Error())
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

// GetMarketBot 查看市场条目详情；未发布内容仅发布者本人可见。
func GetMarketBot(ctx context.Context, c *app.RequestContext) {
	identity, _ := middleware.CurrentUser(c)
	marketBotId, err := strconv.ParseInt(c.Param("market_bot_id"), 10, 64)
	if err != nil || marketBotId <= 0 {
		response.Error(c, "参数错误", "市场Bot ID 非法")
		return
	}
	rpcReq := &bot.GetMarketBotReq{MarketBotId: marketBotId}
	if identity != nil {
		rpcReq.ViewerId = &identity.Id
	}
	resp, err := rpc.GetMarketBot(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "查询市场Bot[%d]失败: %v", marketBotId, err)
		response.Error(c, "查询失败", nil)
		return
	}
	if resp.Bot == nil {
		response.Error(c, "条目不存在", "该 Bot 不存在或不可见")
		return
	}
	response.Success(c, map[string]interface{}{"bot": toMarketBotItem(resp.Bot)})
}

// ListMarketBots 浏览已发布的市场条目，支持关键词/分类/分页。
func ListMarketBots(ctx context.Context, c *app.RequestContext) {
	page, _ := strconv.ParseInt(c.Query("page"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.Query("page_size"), 10, 32)
	rpcReq := &bot.ListMarketBotsReq{}
	if keyword := c.Query("keyword"); keyword != "" {
		rpcReq.Keyword = &keyword
	}
	if category := c.Query("category"); category != "" {
		rpcReq.Category = &category
	}
	if page > 0 {
		p := int32(page)
		rpcReq.Page = &p
	}
	if pageSize > 0 {
		ps := int32(pageSize)
		rpcReq.PageSize = &ps
	}
	resp, err := rpc.ListMarketBots(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "浏览市场Bot列表失败: %v", err)
		response.Error(c, "查询失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{
		"bots":  toMarketBotItems(resp.Bots),
		"total": resp.Total,
	})
}

// ListMyMarketBots 查看自己发布的条目（含草稿/待审/驳回/下架）。
func ListMyMarketBots(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	page, _ := strconv.ParseInt(c.Query("page"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.Query("page_size"), 10, 32)
	rpcReq := &bot.ListMyMarketBotsReq{TenantId: identity.TenantId, PublisherId: identity.Id}
	if statusStr := c.Query("status"); statusStr != "" {
		if status, err := strconv.ParseInt(statusStr, 10, 16); err == nil {
			s := int16(status)
			rpcReq.Status = &s
		}
	}
	if page > 0 {
		p := int32(page)
		rpcReq.Page = &p
	}
	if pageSize > 0 {
		ps := int32(pageSize)
		rpcReq.PageSize = &ps
	}
	resp, err := rpc.ListMyMarketBots(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "查询我的发布失败: %v", err)
		response.Error(c, "查询失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{
		"bots":  toMarketBotItems(resp.Bots),
		"total": resp.Total,
	})
}

type installMarketBotReq struct {
	MarketBotId int64  `json:"market_bot_id,string" vd:"$ > 0"`
	Name         string `json:"name"`
	ApiKey       string `json:"api_key" vd:"len($) > 0"`
	Model        string `json:"model" vd:"len($) > 0"`
	BaseUrl      string `json:"base_url"`
}

// InstallMarketBot 用自己的 apikey 把市场 Bot 安装为自己的实例。
func InstallMarketBot(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var req installMarketBotReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	rpcReq := &bot.InstallMarketBotReq{
		TenantId:    identity.TenantId,
		UserId:      identity.Id,
		MarketBotId: req.MarketBotId,
		Name:        req.Name,
		ApiKey:      req.ApiKey,
		Model:       req.Model,
	}
	if req.BaseUrl != "" {
		rpcReq.BaseUrl = &req.BaseUrl
	}
	resp, err := rpc.InstallMarketBot(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "安装市场Bot失败: %v", err)
		response.Error(c, "安装失败", err.Error())
		return
	}
	response.Success(c, map[string]interface{}{
		"bot_id":  fmt.Sprintf("%d", resp.BotId),
		"version": resp.Version,
	})
}

type rateMarketBotReq struct {
	MarketBotId int64 `json:"market_bot_id,string" vd:"$ > 0"`
	Score        int32 `json:"score" vd:"$ >= 1 && $ <= 5"`
}

// RateMarketBot 对已发布的市场 Bot 评分（1-5），重复评分视为改分。
func RateMarketBot(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var req rateMarketBotReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.RateMarketBot(ctx, &bot.RateMarketBotReq{
		TenantId:    identity.TenantId,
		UserId:      identity.Id,
		MarketBotId: req.MarketBotId,
		Score:       req.Score,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "评价市场Bot失败: %v", err)
		response.Error(c, "评价失败", err.Error())
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

type offlineMarketBotReq struct {
	MarketBotId int64 `json:"market_bot_id,string" vd:"$ > 0"`
}

// OfflineMarketBot 发布者下架自己的市场条目。
func OfflineMarketBot(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var req offlineMarketBotReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.OfflineMarketBot(ctx, &bot.OfflineMarketBotReq{
		TenantId:    identity.TenantId,
		MarketBotId: req.MarketBotId,
		OperatorId:  identity.Id,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "下架市场Bot失败: %v", err)
		response.Error(c, "下架失败", err.Error())
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

// AdminListMarketReviews 管理后台审核列表；未传状态时默认查看待审核队列。
func AdminListMarketReviews(ctx context.Context, c *app.RequestContext) {
	if _, ok := middleware.CurrentAdmin(c); !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	page, _ := strconv.ParseInt(c.Query("page"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.Query("page_size"), 10, 32)
	rpcReq := &bot.ListMarketReviewsReq{}
	if statusStr := c.Query("status"); statusStr != "" {
		if status, err := strconv.ParseInt(statusStr, 10, 16); err == nil {
			s := int16(status)
			rpcReq.Status = &s
		}
	}
	if page > 0 {
		p := int32(page)
		rpcReq.Page = &p
	}
	if pageSize > 0 {
		ps := int32(pageSize)
		rpcReq.PageSize = &ps
	}
	resp, err := rpc.ListMarketReviews(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "查询市场审核列表失败: %v", err)
		response.Error(c, "查询失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{
		"bots":  toMarketBotItems(resp.Bots),
		"total": resp.Total,
	})
}

type reviewMarketBotReq struct {
	Note string `json:"note"`
}

// AdminApproveMarketBot 管理员审核通过，上架市场条目。
func AdminApproveMarketBot(ctx context.Context, c *app.RequestContext) {
	adminReview(ctx, c, true)
}

// AdminRejectMarketBot 管理员驳回市场条目，并记录审核意见。
func AdminRejectMarketBot(ctx context.Context, c *app.RequestContext) {
	adminReview(ctx, c, false)
}

func adminReview(ctx context.Context, c *app.RequestContext, approve bool) {
	identity, ok := middleware.CurrentAdmin(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	marketBotId, err := strconv.ParseInt(c.Param("market_bot_id"), 10, 64)
	if err != nil || marketBotId <= 0 {
		response.Error(c, "参数错误", "市场Bot ID 非法")
		return
	}
	var req reviewMarketBotReq
	_ = c.BindJSON(&req)
	rpcReq := &bot.ReviewMarketBotReq{
		AdminId:     identity.Id,
		MarketBotId: marketBotId,
		Approve:     approve,
	}
	if req.Note != "" {
		rpcReq.Note = &req.Note
	}
	resp, err := rpc.ReviewMarketBot(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "审核市场Bot[%d]失败: %v", marketBotId, err)
		response.Error(c, "审核失败", err.Error())
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}
