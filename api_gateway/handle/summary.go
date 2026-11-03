package handle

import (
	"context"
	"strconv"

	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/middleware/ratelimit"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"
	work "github.com/Airiseina/answer/kitex_service/work_service/kitex_gen/work"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

const (
	summaryTypeDaily  = "daily"
	summaryTypeWeekly = "weekly"
)

// summaryReportItem 是总结报告对外的统一视图。
type summaryReportItem struct {
	Id          string `json:"id"`
	UserId      string `json:"user_id"`
	Type        string `json:"type"`
	PeriodStart int64  `json:"period_start"`
	PeriodEnd   int64  `json:"period_end"`
	Content     string `json:"content"`
	Model       string `json:"model"`
	Status      int16  `json:"status"`
	Error       string `json:"error"`
	CreatedAt   int64  `json:"created_at"`
}

func toSummaryReportItem(info *work.SummaryReportInfo) summaryReportItem {
	return summaryReportItem{
		Id:          strconv.FormatInt(info.Id, 10),
		UserId:      strconv.FormatInt(info.UserId, 10),
		Type:        info.Type,
		PeriodStart: info.PeriodStart,
		PeriodEnd:   info.PeriodEnd,
		Content:     info.Content,
		Model:       info.Model,
		Status:      info.Status,
		Error:       info.Error,
		CreatedAt:   info.CreatedAt,
	}
}

// currentSummaryUserID 读取当前登录用户 ID；未登录返回 0。
func currentSummaryUserID(c *app.RequestContext) int64 {
	identity, ok := c.Get(middleware.IdentityKey)
	if !ok {
		return 0
	}
	userInfo, ok := identity.(*middleware.Resp)
	if !ok || userInfo == nil {
		return 0
	}
	return userInfo.Id
}

// ListSummaryReports 返回当前用户全部总结（可按 type 过滤）。
func ListSummaryReports(ctx context.Context, c *app.RequestContext) {
	listSummaries(ctx, c, "")
}

// ListDailySummaries 返回当前用户的日总结列表。
func ListDailySummaries(ctx context.Context, c *app.RequestContext) {
	listSummaries(ctx, c, summaryTypeDaily)
}

// ListWeeklySummaries 返回当前用户的周总结列表。
func ListWeeklySummaries(ctx context.Context, c *app.RequestContext) {
	listSummaries(ctx, c, summaryTypeWeekly)
}

// listSummaries 统一处理列表查询；forcedType 非空时强制按该类型过滤。
func listSummaries(ctx context.Context, c *app.RequestContext, forcedType string) {
	userID := currentSummaryUserID(c)
	if userID <= 0 {
		response.Error(c, "未登录", "登录态失效")
		return
	}
	filterType := forcedType
	if filterType == "" {
		filterType = c.Query("type")
	}
	if filterType != "" && filterType != summaryTypeDaily && filterType != summaryTypeWeekly {
		response.Error(c, "参数错误", "type只能为daily或weekly")
		return
	}
	var limit *int32
	if raw := c.Query("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			l := int32(v)
			limit = &l
		}
	}
	var offset *int32
	if raw := c.Query("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			o := int32(v)
			offset = &o
		}
	}
	req := &work.ListSummaryReportsReq{UserId: userID, Limit: limit, Offset: offset}
	if filterType != "" {
		req.Type = &filterType
	}
	resp, err := rpc.ListSummaryReports(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC ListSummaryReports失败: %v", err)
		response.Error(c, "系统繁忙", "查询总结列表失败")
		return
	}
	items := make([]summaryReportItem, 0, len(resp.Reports))
	for _, info := range resp.Reports {
		items = append(items, toSummaryReportItem(info))
	}
	response.Success(c, map[string]interface{}{
		"reports": items,
	})
}

// GetSummaryReport 读取本人某份总结详情。
func GetSummaryReport(ctx context.Context, c *app.RequestContext) {
	userID := currentSummaryUserID(c)
	if userID <= 0 {
		response.Error(c, "未登录", "登录态失效")
		return
	}
	reportID, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil || reportID <= 0 {
		response.Error(c, "参数错误", "id格式不正确")
		return
	}
	resp, err := rpc.GetSummaryReport(ctx, &work.GetSummaryReportReq{UserId: userID, ReportId: reportID})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC GetSummaryReport失败: %v", err)
		response.Error(c, "系统繁忙", "查询总结详情失败")
		return
	}
	if !resp.Success || resp.Report == nil {
		response.Error(c, "总结不存在", "未找到该总结或无权查看")
		return
	}
	response.Success(c, map[string]interface{}{
		"report": toSummaryReportItem(resp.Report),
	})
}

// GenerateSummary 手动触发一次日/周总结生成。
func GenerateSummary(ctx context.Context, c *app.RequestContext) {
	userID := currentSummaryUserID(c)
	if userID <= 0 {
		response.Error(c, "未登录", "登录态失效")
		return
	}
	if !ratelimit.CheckRateLimit(c, userID, "summary_generate", ratelimit.Default) {
		return
	}
	var reqBody struct {
		Type string `json:"type"`
	}
	if err := c.BindJSON(&reqBody); err != nil {
		response.Error(c, "参数错误", "请求格式不正确")
		return
	}
	if reqBody.Type == "" {
		reqBody.Type = summaryTypeDaily
	}
	if reqBody.Type != summaryTypeDaily && reqBody.Type != summaryTypeWeekly {
		response.Error(c, "参数错误", "type只能为daily或weekly")
		return
	}
	resp, err := rpc.GenerateSummary(ctx, &work.GenerateSummaryReq{UserId: userID, Type: reqBody.Type})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC GenerateSummary失败: %v", err)
		response.Error(c, "系统繁忙", "生成总结失败，请稍后重试")
		return
	}
	if !resp.Success || resp.Report == nil {
		response.Error(c, "生成失败", "无法生成该总结")
		return
	}
	response.Success(c, map[string]interface{}{
		"report": toSummaryReportItem(resp.Report),
	})
}

// GetSummarySchedule 读取当前用户的总结订阅开关。
func GetSummarySchedule(ctx context.Context, c *app.RequestContext) {
	userID := currentSummaryUserID(c)
	if userID <= 0 {
		response.Error(c, "未登录", "登录态失效")
		return
	}
	resp, err := rpc.GetSummarySchedule(ctx, &work.GetSummaryScheduleReq{UserId: userID})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC GetSummarySchedule失败: %v", err)
		response.Error(c, "系统繁忙", "查询总结订阅失败")
		return
	}
	schedule := resp.GetSchedule()
	response.Success(c, map[string]interface{}{
		"daily_enabled":  schedule.GetDailyEnabled(),
		"weekly_enabled": schedule.GetWeeklyEnabled(),
	})
}

// SetSummarySchedule 更新当前用户的总结订阅开关。
func SetSummarySchedule(ctx context.Context, c *app.RequestContext) {
	userID := currentSummaryUserID(c)
	if userID <= 0 {
		response.Error(c, "未登录", "登录态失效")
		return
	}
	var reqBody struct {
		DailyEnabled  *bool `json:"daily_enabled"`
		WeeklyEnabled *bool `json:"weekly_enabled"`
	}
	if err := c.BindJSON(&reqBody); err != nil {
		response.Error(c, "参数错误", "请求格式不正确")
		return
	}
	if reqBody.DailyEnabled == nil || reqBody.WeeklyEnabled == nil {
		response.Error(c, "参数错误", "daily_enabled与weekly_enabled必填")
		return
	}
	resp, err := rpc.SetSummarySchedule(ctx, &work.SetSummaryScheduleReq{
		UserId:        userID,
		DailyEnabled:  *reqBody.DailyEnabled,
		WeeklyEnabled: *reqBody.WeeklyEnabled,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC SetSummarySchedule失败: %v", err)
		response.Error(c, "系统繁忙", "更新总结订阅失败")
		return
	}
	if !resp.Success {
		response.Error(c, "更新失败", "无法更新总结订阅")
		return
	}
	response.Success(c, map[string]interface{}{
		"daily_enabled":  *reqBody.DailyEnabled,
		"weekly_enabled": *reqBody.WeeklyEnabled,
	})
}
