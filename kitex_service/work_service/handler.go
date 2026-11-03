package main

import (
	"context"

	"github.com/Airiseina/answer/kitex_service/work_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/service"
	work "github.com/Airiseina/answer/kitex_service/work_service/kitex_gen/work"

	"github.com/cloudwego/kitex/pkg/klog"
)

type WorkServiceImpl struct {
	workService *service.WorkService
}

// SummarizeConversation implements the WorkServiceImpl interface.
func (s *WorkServiceImpl) SummarizeConversation(ctx context.Context, req *work.SummarizeConversationReq) (resp *work.SummarizeConversationRes, err error) {
	summary, err := s.workService.SummarizeConversation(ctx, req.ConversationId, req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "总结会话[%d]失败: %v", req.ConversationId, err)
		return &work.SummarizeConversationRes{Success: false}, err
	}
	return &work.SummarizeConversationRes{Success: true, Summary: summary}, nil
}

func (s *WorkServiceImpl) SuggestReplies(ctx context.Context, req *work.SuggestRepliesReq) (resp *work.SuggestRepliesRes, err error) {
	replies, err := s.workService.SuggestReplies(ctx, req.ConversationId, req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "生成回复候选失败: %v", err)
		return &work.SuggestRepliesRes{Success: false}, err
	}
	return &work.SuggestRepliesRes{Success: true, Replies: replies}, nil
}

func (s *WorkServiceImpl) TranslateMessage(ctx context.Context, req *work.TranslateMessageReq) (resp *work.TranslateMessageRes, err error) {
	result, err := s.workService.TranslateMessage(ctx, req.Content, req.TargetLang)
	if err != nil {
		klog.CtxErrorf(ctx, "翻译消息失败: %v", err)
		return &work.TranslateMessageRes{Success: false}, err
	}
	return &work.TranslateMessageRes{Success: true, TranslatedContent: result}, nil
}

// GenerateSummary 手动触发一次日/周总结。
func (s *WorkServiceImpl) GenerateSummary(ctx context.Context, req *work.GenerateSummaryReq) (resp *work.GenerateSummaryRes, err error) {
	report, err := s.workService.GenerateSummary(ctx, req.UserId, req.Type, req.GetPeriodStart(), req.GetPeriodEnd())
	if err != nil {
		klog.CtxErrorf(ctx, "生成总结失败 user=%d type=%s: %v", req.UserId, req.Type, err)
		return &work.GenerateSummaryRes{Success: false}, err
	}
	return &work.GenerateSummaryRes{Success: true, Report: toSummaryReportInfo(report)}, nil
}

// GetSummaryReport 读取本人总结详情。
func (s *WorkServiceImpl) GetSummaryReport(ctx context.Context, req *work.GetSummaryReportReq) (resp *work.GetSummaryReportRes, err error) {
	report, err := s.workService.GetSummaryReport(ctx, req.UserId, req.ReportId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询总结报告失败 user=%d report=%d: %v", req.UserId, req.ReportId, err)
		return &work.GetSummaryReportRes{Success: false}, err
	}
	if report.ID == 0 {
		return &work.GetSummaryReportRes{Success: false}, nil
	}
	return &work.GetSummaryReportRes{Success: true, Report: toSummaryReportInfo(report)}, nil
}

// ListSummaryReports 返回本人总结列表。
func (s *WorkServiceImpl) ListSummaryReports(ctx context.Context, req *work.ListSummaryReportsReq) (resp *work.ListSummaryReportsRes, err error) {
	reports, err := s.workService.ListSummaryReports(ctx, req.UserId, req.GetType(), req.GetLimit(), req.GetOffset())
	if err != nil {
		klog.CtxErrorf(ctx, "查询总结列表失败 user=%d: %v", req.UserId, err)
		return &work.ListSummaryReportsRes{}, err
	}
	infos := make([]*work.SummaryReportInfo, 0, len(reports))
	for _, report := range reports {
		infos = append(infos, toSummaryReportInfo(report))
	}
	return &work.ListSummaryReportsRes{Reports: infos}, nil
}

// GetSummarySchedule 读取用户的总结订阅开关。
func (s *WorkServiceImpl) GetSummarySchedule(ctx context.Context, req *work.GetSummaryScheduleReq) (resp *work.GetSummaryScheduleRes, err error) {
	schedule, err := s.workService.GetSummarySchedule(ctx, req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询总结订阅失败 user=%d: %v", req.UserId, err)
		return &work.GetSummaryScheduleRes{Success: false}, err
	}
	return &work.GetSummaryScheduleRes{
		Success: true,
		Schedule: &work.SummaryScheduleInfo{
			DailyEnabled:  schedule.DailyEnabled,
			WeeklyEnabled: schedule.WeeklyEnabled,
		},
	}, nil
}

// SetSummarySchedule 写入用户的总结订阅开关。
func (s *WorkServiceImpl) SetSummarySchedule(ctx context.Context, req *work.SetSummaryScheduleReq) (resp *work.SetSummaryScheduleRes, err error) {
	if _, err = s.workService.SetSummarySchedule(ctx, req.UserId, req.DailyEnabled, req.WeeklyEnabled); err != nil {
		klog.CtxErrorf(ctx, "更新总结订阅失败 user=%d: %v", req.UserId, err)
		return &work.SetSummaryScheduleRes{Success: false}, err
	}
	return &work.SetSummaryScheduleRes{Success: true}, nil
}

// toSummaryReportInfo 把领域模型转换为 IDL 结构，created_at 统一转毫秒时间戳。
func toSummaryReportInfo(report model.SummaryReport) *work.SummaryReportInfo {
	return &work.SummaryReportInfo{
		Id:          report.ID,
		UserId:      report.UserID,
		Type:        report.Type,
		PeriodStart: report.PeriodStart,
		PeriodEnd:   report.PeriodEnd,
		Content:     report.Content,
		Model:       report.Model,
		Status:      report.Status,
		Error:       report.Error,
		CreatedAt:   report.CreatedAt.UnixMilli(),
	}
}
