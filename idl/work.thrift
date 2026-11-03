namespace go work

struct SummarizeConversationReq {
    1: i64 conversation_id
    2: i64 user_id
}

struct SummarizeConversationRes {
    1: bool success
    2: string summary
}

struct SuggestRepliesReq {
    1: i64 conversation_id
    2: i64 user_id
}

struct SuggestRepliesRes {
    1: bool success
    2: list<string> replies
}

struct TranslateMessageReq {
    1: string content
    2: string target_lang
}

struct TranslateMessageRes {
    1: bool success
    2: string translated_content
}

// ===== 系统 Agent 扩展：日总结 / 周总结（Phase 4）=====

// SummaryReportInfo 一份已生成的总结报告
struct SummaryReportInfo {
    1: i64 id
    2: i64 user_id
    3: string type          // daily / weekly
    4: i64 period_start     // 统计窗口起始（毫秒时间戳）
    5: i64 period_end       // 统计窗口结束（毫秒时间戳）
    6: string content       // 总结正文
    7: string model         // 生成所用模型
    8: i16 status           // 0 待生成 1 生成中 2 完成 3 失败
    9: string error         // 失败原因
    10: i64 created_at
}

// GenerateSummaryReq 手动触发一次总结生成
// period_start / period_end 为空时由服务端按 type 推导自然日/自然周窗口
struct GenerateSummaryReq {
    1: i64 user_id
    2: string type          // daily / weekly
    3: optional i64 period_start
    4: optional i64 period_end
}

struct GenerateSummaryRes {
    1: bool success
    2: SummaryReportInfo report
}

struct GetSummaryReportReq {
    1: i64 user_id
    2: i64 report_id
}

struct GetSummaryReportRes {
    1: bool success
    2: SummaryReportInfo report
}

// ListSummaryReportsReq 查询本人总结列表（仅返回请求者自己的数据）
struct ListSummaryReportsReq {
    1: i64 user_id
    2: optional string type     // 为空表示不过滤
    3: optional i32 limit       // 默认20，上限100
    4: optional i32 offset
}

struct ListSummaryReportsRes {
    1: list<SummaryReportInfo> reports
}

// SummaryScheduleInfo 用户的总结订阅开关
struct SummaryScheduleInfo {
    1: bool daily_enabled
    2: bool weekly_enabled
}

struct GetSummaryScheduleReq {
    1: i64 user_id
}

struct GetSummaryScheduleRes {
    1: bool success
    2: SummaryScheduleInfo schedule
}

struct SetSummaryScheduleReq {
    1: i64 user_id
    2: bool daily_enabled
    3: bool weekly_enabled
}

struct SetSummaryScheduleRes {
    1: bool success
}

service WorkService {
    SummarizeConversationRes SummarizeConversation(1: SummarizeConversationReq req)
    SuggestRepliesRes SuggestReplies(1: SuggestRepliesReq req)
    TranslateMessageRes TranslateMessage(1: TranslateMessageReq req)
    // 日/周总结：手动触发、查询、订阅设置
    GenerateSummaryRes GenerateSummary(1: GenerateSummaryReq req)
    GetSummaryReportRes GetSummaryReport(1: GetSummaryReportReq req)
    ListSummaryReportsRes ListSummaryReports(1: ListSummaryReportsReq req)
    GetSummaryScheduleRes GetSummarySchedule(1: GetSummaryScheduleReq req)
    SetSummaryScheduleRes SetSummarySchedule(1: SetSummaryScheduleReq req)
}
