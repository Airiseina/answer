package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/work_service/rpc"
	"github.com/Airiseina/answer/pkg/observability/meter"
	"github.com/Airiseina/answer/pkg/tenant"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	// summaryMessageLimit 单次总结最多拉取的消息条数（chat_service 上限 1000）。
	summaryMessageLimit int16 = 800
	// summaryTimelineMaxRunes 拼给 LLM 的时间线字符上限（按 rune 计），超出即截断。
	summaryTimelineMaxRunes = 12000
	// summaryListDefaultLimit / summaryListMaxLimit 总结列表默认与最大分页。
	summaryListDefaultLimit int32 = 20
	summaryListMaxLimit     int32 = 100
	// summarySendTimeout 投递总结消息的超时。
	summarySendTimeout = 10 * time.Second
)

// 空档占位文案：当天/当周没有消息时也必须发一条，保持固定节奏提醒。
const (
	emptyDailySummary  = "今日无话可说"
	emptyWeeklySummary = "本周无话可说"
)

// 提示词文件缺失时的兜底提示词，保证功能可用。
const (
	defaultDailySummaryPrompt = "你是一个贴心的个人助理，负责为用户生成「今日总结」。请根据提供的一天聊天时间线，" +
		"输出 300 字以内的中文总结，包含「今日概览 / 关键事项 / 明日提醒」三块；只依据消息内容总结，不要编造。"
	defaultWeeklySummaryPrompt = "你是一个贴心的个人助理，负责为用户生成「本周总结」。请根据提供的一周聊天时间线，" +
		"输出 500 字以内的中文总结，包含「本周概览 / 重点进展 / 下周关注」三块；只依据消息内容总结，不要编造。"
)

// SummaryWindow 表示一次总结的统计窗口（毫秒时间戳，含首含尾）。
type SummaryWindow struct {
	Start int64
	End   int64
}

// 提示词运行时加载一次并缓存，与用户 Bot 安全提示词的加载方式保持一致。
var (
	summaryPromptOnce sync.Once
	summaryPrompts    map[string]string
)

// summaryLocation 返回总结调度使用的时区；容器缺少 tzdata 时退回固定东八区。
func summaryLocation() *time.Location {
	if loc, err := time.LoadLocation(model.DefaultSummaryTimezone); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*60*60)
}

// SummaryWindowFor 按类型推导 now 所在的自然日 / 自然周（周一为周首）窗口。
func SummaryWindowFor(reportType string, now time.Time) (SummaryWindow, error) {
	loc := summaryLocation()
	local := now.In(loc)
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	switch reportType {
	case model.SummaryTypeDaily:
		return SummaryWindow{Start: dayStart.UnixMilli(), End: dayStart.AddDate(0, 0, 1).UnixMilli() - 1}, nil
	case model.SummaryTypeWeekly:
		offset := (int(local.Weekday()) + 6) % 7 // 周一=0
		weekStart := dayStart.AddDate(0, 0, -offset)
		return SummaryWindow{Start: weekStart.UnixMilli(), End: weekStart.AddDate(0, 0, 7).UnixMilli() - 1}, nil
	default:
		return SummaryWindow{}, fmt.Errorf("不支持的总结类型: %s", reportType)
	}
}

// resolveSummaryWindow 优先使用调用方给定的窗口，缺省时按 now 推导自然窗口。
func resolveSummaryWindow(reportType string, periodStart, periodEnd int64) (SummaryWindow, error) {
	if periodStart > 0 && periodEnd > 0 {
		if periodStart > periodEnd {
			return SummaryWindow{}, fmt.Errorf("总结时间窗口非法")
		}
		return SummaryWindow{Start: periodStart, End: periodEnd}, nil
	}
	return SummaryWindowFor(reportType, time.Now())
}

// loadSummaryPrompt 读取指定类型的提示词文件（只加载一次）。
func loadSummaryPrompt(reportType string) string {
	summaryPromptOnce.Do(func() {
		summaryPrompts = map[string]string{
			model.SummaryTypeDaily:  readSummaryPromptFile(config.V.GetString("ai.summary.daily_prompt_file"), "日"),
			model.SummaryTypeWeekly: readSummaryPromptFile(config.V.GetString("ai.summary.weekly_prompt_file"), "周"),
		}
	})
	return summaryPrompts[reportType]
}

func readSummaryPromptFile(path, label string) string {
	if path == "" {
		klog.Warnf("未配置%s总结提示词文件，使用内置兜底提示词", label)
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		klog.Warnf("读取%s总结提示词文件[%s]失败: %v", label, path, err)
		return ""
	}
	return strings.TrimSpace(string(data))
}

func summarySystemPrompt(reportType string) string {
	if prompt := loadSummaryPrompt(reportType); prompt != "" {
		return prompt
	}
	if reportType == model.SummaryTypeWeekly {
		return defaultWeeklySummaryPrompt
	}
	return defaultDailySummaryPrompt
}

func summaryTypeLabel(reportType string) string {
	if reportType == model.SummaryTypeWeekly {
		return "周"
	}
	return "日"
}

// GenerateSummary 生成一份日/周总结：聚合用户全部会话的时间窗消息，
// 由系统 Bot 的 LLM 生成正文，落库后再以系统 Bot 私聊投递给用户。
//
// 幂等：同一 (user, type, period_start) 已生成成功则直接返回旧报告，
// 保证调度重试不会重复生成、重复推送。
func (svc *WorkService) GenerateSummary(ctx context.Context, userID int64, reportType string, periodStart, periodEnd int64) (model.SummaryReport, error) {
	if userID <= 0 {
		return model.SummaryReport{}, fmt.Errorf("用户ID不能为空")
	}
	if reportType == "" {
		reportType = model.SummaryTypeDaily
	}
	if svc.summaryDao == nil {
		return model.SummaryReport{}, fmt.Errorf("总结服务未初始化")
	}
	window, err := resolveSummaryWindow(reportType, periodStart, periodEnd)
	if err != nil {
		return model.SummaryReport{}, err
	}
	existing, err := svc.summaryDao.GetReportByPeriod(userID, reportType, window.Start)
	if err != nil {
		return model.SummaryReport{}, err
	}
	if existing.ID != 0 && existing.Status == model.SummaryStatusCompleted {
		return existing, nil
	}

	report := model.SummaryReport{
		TenantID:    rpc.GetPersonalTenantID(ctx, userID),
		UserID:      userID,
		Type:        reportType,
		PeriodStart: window.Start,
		PeriodEnd:   window.End,
		Status:      model.SummaryStatusRunning,
	}
	if existing.ID != 0 {
		// 复用失败/进行中的报告行，避免唯一索引冲突。
		report.ID = existing.ID
		if report.TenantID == 0 {
			report.TenantID = existing.TenantID
		}
		if err = svc.summaryDao.UpdateReport(existing.ID, map[string]interface{}{
			"period_end": window.End,
			"status":     model.SummaryStatusRunning,
			"error":      "",
		}); err != nil {
			return model.SummaryReport{}, err
		}
	} else {
		report.ID = svc.nextSummaryID()
		if err = svc.summaryDao.CreateReport(&report); err != nil {
			return model.SummaryReport{}, err
		}
	}

	content, modelName, genErr := svc.buildSummaryContent(ctx, userID, reportType, window)
	if genErr != nil {
		_ = svc.summaryDao.UpdateReport(report.ID, map[string]interface{}{
			"status": model.SummaryStatusFailed,
			"error":  truncateSummaryError(genErr.Error()),
		})
		meter.M.SummaryGenerateTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("type", reportType),
			attribute.String("result", "failed"),
		))
		return model.SummaryReport{}, genErr
	}

	if err = svc.summaryDao.UpdateReport(report.ID, map[string]interface{}{
		"content": content,
		"model":   modelName,
		"status":  model.SummaryStatusCompleted,
		"error":   "",
	}); err != nil {
		return model.SummaryReport{}, err
	}
	report.Content = content
	report.Model = modelName
	report.Status = model.SummaryStatusCompleted

	// 投递失败不回滚报告：正文已生成，用户仍可在列表里查看。
	if err = svc.deliverSummary(ctx, report.TenantID, userID, content); err != nil {
		klog.CtxErrorf(ctx, "总结[%d]投递失败: %v", report.ID, err)
	}
	meter.M.SummaryGenerateTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("type", reportType),
		attribute.String("result", "completed"),
	))
	return report, nil
}

// buildSummaryContent 生成总结正文；时间线为空时返回占位文案且不调用 LLM。
func (svc *WorkService) buildSummaryContent(ctx context.Context, userID int64, reportType string, window SummaryWindow) (string, string, error) {
	timeline, err := svc.buildSummaryTimeline(ctx, userID, window)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(timeline) == "" {
		if reportType == model.SummaryTypeWeekly {
			return emptyWeeklySummary, "", nil
		}
		return emptyDailySummary, "", nil
	}
	apiKey, baseURL, modelName, err := svc.getSystemBotLLMConfig(ctx)
	if err != nil {
		return "", "", err
	}
	llmCtx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()
	result, llmErr := svc.llmClient.Chat(llmCtx, apiKey, baseURL, modelName, summarySystemPrompt(reportType), nil, timeline, nil)
	if llmErr != nil {
		return "", "", fmt.Errorf("生成%s总结失败: %w", summaryTypeLabel(reportType), llmErr)
	}
	if strings.TrimSpace(result) == "" {
		return "", "", fmt.Errorf("生成%s总结失败: LLM返回空内容", summaryTypeLabel(reportType))
	}
	return strings.TrimSpace(result), modelName, nil
}

// buildSummaryTimeline 把用户在时间窗内跨会话的消息拼成带上下文的文本时间线。
// tenant_id 传 0：总结聚合用户在所有租户下的会话。
func (svc *WorkService) buildSummaryTimeline(ctx context.Context, userID int64, window SummaryWindow) (string, error) {
	msgs, truncated, err := rpc.GetMessagesByTimeRange(ctx, 0, userID, window.Start, window.End, summaryMessageLimit)
	if err != nil {
		return "", fmt.Errorf("拉取总结消息失败: %w", err)
	}
	if len(msgs) == 0 {
		return "", nil
	}
	briefs, briefErr := rpc.GetConversationBriefs(ctx, userID)
	if briefErr != nil {
		klog.CtxWarnf(ctx, "拉取会话列表失败，总结时间线将不带会话名: %v", briefErr)
		briefs = map[int64]rpc.ConversationBrief{}
	}
	senderIDs := make([]int64, 0, len(msgs))
	for _, m := range msgs {
		senderIDs = append(senderIDs, m.SenderId)
	}
	nameMap := make(map[int64]string)
	if names, nameErr := rpc.GetUserNames(ctx, senderIDs); nameErr == nil {
		for _, u := range names {
			nameMap[u.Id] = u.Name
		}
	}
	var sb strings.Builder
	total := 0
	for _, m := range msgs {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		sender := nameMap[m.SenderId]
		if sender == "" {
			sender = fmt.Sprintf("用户%d", m.SenderId)
		}
		stamp := time.UnixMilli(m.Timestamp).In(summaryLocation()).Format("01-02 15:04")
		conv := summaryConversationLabel(briefs[m.ConversationId], userID, nameMap)
		line := fmt.Sprintf("[%s][%s] %s: %s\n", stamp, conv, sender, content)
		if total+len([]rune(line)) > summaryTimelineMaxRunes {
			sb.WriteString("...(时间线过长，后续消息已省略)\n")
			break
		}
		sb.WriteString(line)
		total += len([]rune(line))
	}
	if truncated {
		sb.WriteString("...(消息条数达到上限，已截断)\n")
	}
	return sb.String(), nil
}

// summaryConversationLabel 给会话一个可读标签：群聊用群名，私聊用对方显示名。
func summaryConversationLabel(brief rpc.ConversationBrief, selfUserID int64, nameMap map[int64]string) string {
	if brief.Type == rpc.ConvTypeGroup {
		if brief.Name != "" {
			return "群聊·" + brief.Name
		}
		return fmt.Sprintf("群聊%d", brief.ID)
	}
	for _, member := range brief.MemberIDs {
		if member == selfUserID {
			continue
		}
		name := nameMap[member]
		if name == "" {
			name = fmt.Sprintf("用户%d", member)
		}
		return "私聊·" + name
	}
	return "私聊"
}

// deliverSummary 以系统 Bot 身份，把总结发送到「用户 ⇄ 系统 Bot」的私聊会话。
// 会话在用户的个人租户下创建，避免总结落到平台租户。
func (svc *WorkService) deliverSummary(ctx context.Context, tenantID, userID int64, content string) error {
	systemBotID, err := rpc.GetSystemBot(ctx)
	if err != nil {
		return fmt.Errorf("获取系统Bot失败: %w", err)
	}
	botCfg, err := rpc.GetBotConfig(ctx, tenant.PlatformTenantID, systemBotID)
	if err != nil {
		return fmt.Errorf("获取系统Bot[%d]配置失败: %w", systemBotID, err)
	}
	if botCfg.UserID <= 0 {
		return fmt.Errorf("系统Bot[%d]缺少用户身份，无法投递总结", systemBotID)
	}
	if tenantID <= 0 {
		tenantID = rpc.GetPersonalTenantID(ctx, userID)
	}
	if tenantID <= 0 {
		return fmt.Errorf("用户[%d]缺少个人租户，无法投递总结", userID)
	}
	sendCtx, cancel := context.WithTimeout(tenant.WithTenant(context.Background(), tenantID), summarySendTimeout)
	defer cancel()
	convID, err := rpc.GetOrCreatePrivateConversation(sendCtx, botCfg.UserID, userID)
	if err != nil {
		return err
	}
	sendResp, err := rpc.SendMessage(sendCtx, &chat.SendMessageReq{
		SenderId:       botCfg.UserID,
		ConversationId: convID,
		Content:        content,
		TenantId:       &tenantID,
	})
	if err != nil {
		return fmt.Errorf("总结消息入库失败: %w", err)
	}
	if svc.kafkaWriter == nil {
		return nil
	}
	reply := map[string]interface{}{
		"msg_id":            sendResp.GetMsgId(),
		"seq":               sendResp.GetSeq(),
		"conversation_id":   sendResp.GetConversationId(),
		"conversation_type": sendResp.GetConversationType(),
		"sender_id":         botCfg.UserID,
		"content":           content,
		"timestamp":         sendResp.GetTimestamp(),
		"member_ids":        sendResp.GetMemberIds(),
	}
	replyJSON, marshalErr := json.Marshal(reply)
	if marshalErr != nil {
		klog.Errorf("总结[%d]推送消息序列化失败: %v", sendResp.GetMsgId(), marshalErr)
		return nil
	}
	if writeErr := svc.kafkaWriter.WriteMessages(context.Background(), kafka.Message{
		Topic: botReplyTopic,
		Key:   []byte(fmt.Sprintf("%d", convID)),
		Value: replyJSON,
	}); writeErr != nil {
		klog.Errorf("总结[%d]推送消息写入Kafka失败: %v", sendResp.GetMsgId(), writeErr)
	}
	return nil
}

// GetSummaryReport 读取本人总结报告；DAO 层强制 user_id 过滤，杜绝越权读取。
func (svc *WorkService) GetSummaryReport(ctx context.Context, userID, reportID int64) (model.SummaryReport, error) {
	if userID <= 0 || reportID <= 0 {
		return model.SummaryReport{}, fmt.Errorf("参数不完整")
	}
	if svc.summaryDao == nil {
		return model.SummaryReport{}, fmt.Errorf("总结服务未初始化")
	}
	return svc.summaryDao.GetReport(userID, reportID)
}

// ListSummaryReports 返回本人总结列表，支持按类型过滤。
func (svc *WorkService) ListSummaryReports(ctx context.Context, userID int64, reportType string, limit, offset int32) ([]model.SummaryReport, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户ID不能为空")
	}
	if reportType != "" && reportType != model.SummaryTypeDaily && reportType != model.SummaryTypeWeekly {
		return nil, fmt.Errorf("不支持的总结类型: %s", reportType)
	}
	if svc.summaryDao == nil {
		return nil, fmt.Errorf("总结服务未初始化")
	}
	if limit <= 0 {
		limit = summaryListDefaultLimit
	}
	if limit > summaryListMaxLimit {
		limit = summaryListMaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return svc.summaryDao.ListReports(userID, reportType, int(offset), int(limit))
}

// GetSummarySchedule 读取用户的总结订阅；未设置过时返回「日/周均开启」的默认值。
func (svc *WorkService) GetSummarySchedule(ctx context.Context, userID int64) (model.SummarySchedule, error) {
	if userID <= 0 {
		return model.SummarySchedule{}, fmt.Errorf("用户ID不能为空")
	}
	if svc.summaryDao == nil {
		return model.SummarySchedule{}, fmt.Errorf("总结服务未初始化")
	}
	schedule, err := svc.summaryDao.GetSchedule(userID)
	if err != nil {
		return model.SummarySchedule{}, err
	}
	if schedule.ID == 0 {
		return model.SummarySchedule{
			UserID:        userID,
			DailyEnabled:  true,
			WeeklyEnabled: true,
			Timezone:      model.DefaultSummaryTimezone,
		}, nil
	}
	if schedule.Timezone == "" {
		schedule.Timezone = model.DefaultSummaryTimezone
	}
	return schedule, nil
}

// SetSummarySchedule 写入用户的日/周总结订阅开关。
func (svc *WorkService) SetSummarySchedule(ctx context.Context, userID int64, dailyEnabled, weeklyEnabled bool) (model.SummarySchedule, error) {
	if userID <= 0 {
		return model.SummarySchedule{}, fmt.Errorf("用户ID不能为空")
	}
	if svc.summaryDao == nil {
		return model.SummarySchedule{}, fmt.Errorf("总结服务未初始化")
	}
	schedule := model.SummarySchedule{
		UserID:        userID,
		DailyEnabled:  dailyEnabled,
		WeeklyEnabled: weeklyEnabled,
		Timezone:      model.DefaultSummaryTimezone,
	}
	if err := svc.summaryDao.SaveSchedule(&schedule); err != nil {
		return model.SummarySchedule{}, err
	}
	return schedule, nil
}

// SummaryEnabled 读取用户订阅开关；未设置过视为开启，读取失败时按开启处理避免漏发。
func (svc *WorkService) SummaryEnabled(ctx context.Context, userID int64, reportType string) bool {
	if svc.summaryDao == nil {
		return false
	}
	schedule, err := svc.summaryDao.GetSchedule(userID)
	if err != nil {
		klog.CtxWarnf(ctx, "读取用户[%d]总结订阅失败，按开启处理: %v", userID, err)
		return true
	}
	if schedule.ID == 0 {
		return true
	}
	if reportType == model.SummaryTypeWeekly {
		return schedule.WeeklyEnabled
	}
	return schedule.DailyEnabled
}

func (svc *WorkService) nextSummaryID() int64 {
	if svc.snowNode != nil {
		return svc.snowNode.Generate()
	}
	return time.Now().UnixNano()
}

// truncateSummaryError 截断错误信息，避免超过 error 列的 varchar(255)。
func truncateSummaryError(msg string) string {
	runes := []rune(msg)
	if len(runes) <= 200 {
		return msg
	}
	return string(runes[:200])
}
