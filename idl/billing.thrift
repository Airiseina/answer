namespace go billing

struct CommonRes {
    1: bool success
}

// TokenUsageInfo 是一条 LLM token 用量明细。
// origin=2（OpenAPI 接入）的 Bot 不写入；cost 为按后台参考单价折算的预估成本。
struct TokenUsageInfo {
    1: i64 id
    2: string biz_date          // yyyy-mm-dd（Asia/Shanghai）
    3: i64 user_id
    4: i64 tenant_id
    5: i64 bot_id
    6: i16 origin               // 0=用户自建 1=市场安装
    7: i64 conversation_id
    8: i64 msg_id               // 幂等去重键
    9: string model
    10: i64 prompt_tokens
    11: i64 completion_tokens
    12: double cost
    13: i64 created_at          // 毫秒时间戳
}

// RecordUsageReq 由 work_service 在 Bot 回复结束后上报（按 msg_id 幂等）。
struct RecordUsageReq {
    1: i64 user_id
    2: i64 tenant_id
    3: i64 bot_id
    4: i16 origin
    5: i64 conversation_id
    6: i64 msg_id
    7: string model
    8: i64 prompt_tokens
    9: i64 completion_tokens
}

struct GetUsageDetailReq {
    1: i64 user_id              // 必填：用量仅本人可见
    2: optional string start_date
    3: optional string end_date
    4: optional string model
    5: optional i64 bot_id
    6: optional i32 limit
    7: optional i32 offset
}

struct GetUsageDetailRes {
    1: bool success
    2: list<TokenUsageInfo> items
    3: i64 total
}

// UsageTrendPoint 是按天的用量趋势点。
struct UsageTrendPoint {
    1: string biz_date
    2: i64 prompt_tokens
    3: i64 completion_tokens
    4: double cost
    5: i64 request_count
}

// UsageGroupStat 是按 Bot 或模型聚合的占比统计。
struct UsageGroupStat {
    1: string key
    2: i64 prompt_tokens
    3: i64 completion_tokens
    4: double cost
    5: i64 request_count
}

struct GetUsageSummaryReq {
    1: i64 user_id              // 必填：用量仅本人可见
    2: optional string start_date
    3: optional string end_date
}

struct GetUsageSummaryRes {
    1: bool success
    2: i64 prompt_tokens
    3: i64 completion_tokens
    4: double cost
    5: i64 request_count
    6: list<UsageTrendPoint> trend
    7: list<UsageGroupStat> by_bot
    8: list<UsageGroupStat> by_model
    9: i64 missing_price_count  // 未配置参考单价、成本记 0 的记录数
}

// ModelPriceInfo 是后台手工维护的参考单价。
struct ModelPriceInfo {
    1: i64 id
    2: string provider
    3: string model
    4: double input_per_1k
    5: double output_per_1k
    6: string currency
    7: bool enabled
    8: i64 created_at
    9: i64 updated_at
}

struct ListModelPricesReq {
    1: optional bool only_enabled
    2: optional i32 limit
    3: optional i32 offset
}

struct ListModelPricesRes {
    1: bool success
    2: list<ModelPriceInfo> prices
    3: i64 total
}

struct UpsertModelPriceReq {
    1: optional i64 id          // 传 0 或省略为新增，否则更新
    2: string provider
    3: string model
    4: double input_per_1k
    5: double output_per_1k
    6: optional string currency
    7: bool enabled
    8: i64 operator_id
}

struct DeleteModelPriceReq {
    1: i64 id
    2: i64 operator_id
}

// UsageQuotaInfo 用量上限（预留：本轮仅用于记录期阈值告警，不做扣费）。
struct UsageQuotaInfo {
    1: i64 id
    2: i16 owner_type           // 0=用户 1=租户
    3: i64 owner_id
    4: i16 period               // 0=日 1=月
    5: i64 token_limit
    6: bool enabled
}

struct SetUsageQuotaReq {
    1: i16 owner_type
    2: i64 owner_id
    3: i16 period
    4: i64 token_limit
    5: bool enabled
    6: i64 operator_id
}

struct ListUsageQuotasReq {
    1: optional i16 owner_type
    2: optional i64 owner_id
}

struct ListUsageQuotasRes {
    1: bool success
    2: list<UsageQuotaInfo> quotas
}

service BillingService {
    CommonRes RecordUsage(1: RecordUsageReq req)
    GetUsageDetailRes GetUsageDetail(1: GetUsageDetailReq req)
    GetUsageSummaryRes GetUsageSummary(1: GetUsageSummaryReq req)
    ListModelPricesRes ListModelPrices(1: ListModelPricesReq req)
    CommonRes UpsertModelPrice(1: UpsertModelPriceReq req)
    CommonRes DeleteModelPrice(1: DeleteModelPriceReq req)
    ListUsageQuotasRes ListUsageQuotas(1: ListUsageQuotasReq req)
    CommonRes SetUsageQuota(1: SetUsageQuotaReq req)
}
