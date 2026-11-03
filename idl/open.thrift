namespace go open

struct CommonRes {
    1: bool success
}

// ===== 应用（AppID + AppSecret）管理：面向站内登录用户 =====

struct CreateAppReq {
    1: i64 owner_id
    2: i64 tenant_id
    3: string name
    4: string scopes
    5: optional i32 qps_limit
}

struct CreateAppRes {
    1: bool success
    2: string app_id
    3: string app_secret  // 仅在创建/重置时明文返回一次
}

struct AppInfo {
    1: string app_id
    2: string name
    3: string scopes
    4: i16 status
    5: i32 qps_limit
    6: i64 owner_id
    7: i64 tenant_id
    8: i64 created_at
}

struct ListAppsReq {
    1: i64 owner_id
    2: i64 tenant_id
}

struct ListAppsRes {
    1: bool success
    2: list<AppInfo> apps
}

struct GetAppReq {
    1: string app_id
}

struct GetAppRes {
    1: bool success
    2: optional AppInfo app
    3: optional string secret_hash
}

struct VerifyAppReq {
    1: string app_id
    2: string app_secret
}

struct VerifyAppRes {
    1: bool success
    2: optional AppInfo app
}

// AuthenticateApp 供 api_gateway 的 open_auth 中间件调用：
// 校验 AppID+HMAC 签名、时间戳窗口、nonce 防重放，返回应用信息与失败原因。
struct AuthenticateAppReq {
    1: string app_id
    2: string timestamp      // 客户端请求头 X-Timestamp（Unix 秒）
    3: string nonce          // 客户端请求头 X-Nonce（随机串）
    4: string method         // HTTP 方法，如 POST
    5: string path           // 请求路径，如 /open/v1/bots
    6: string body_sha256    // 请求体 SHA-256 十六进制（空体为 sha256("")）
    7: string signature      // HMAC-SHA256(app_secret, stringToSign) 十六进制
}

struct AuthenticateAppRes {
    1: bool success
    2: optional AppInfo app
    3: string err_code       // ok / bad_signature / timestamp_expired / replay / forbidden
}

struct ResetAppSecretReq {
    1: string app_id
    2: i64 operator_id
}

struct ResetAppSecretRes {
    1: bool success
    2: string app_secret
}

struct UpdateAppStatusReq {
    1: string app_id
    2: i64 operator_id
    3: i16 status
}

// ===== 开放平台 Bot：等价站内 CreateBot，但归属到 App =====

struct CreateOpenBotReq {
    1: string app_id
    2: i64 tenant_id
    3: i64 creator_id
    4: string name
    5: string system_prompt
    6: string api_key
    7: string model
    8: optional string base_url
}

struct CreateOpenBotRes {
    1: bool success
    2: i64 bot_id
}

struct OpenBotInfo {
    1: i64 bot_id
    2: string app_id
    3: i64 creator_id
    4: string name
    5: string system_prompt
    6: string model
    7: string base_url
    8: i64 created_at
}

struct ListOpenBotsReq {
    1: string app_id
    2: i64 tenant_id
}

struct ListOpenBotsRes {
    1: bool success
    2: list<OpenBotInfo> bots
}

struct GetOpenBotReq {
    1: string app_id
    2: i64 bot_id
    3: i64 tenant_id
}

struct GetOpenBotRes {
    1: bool success
    2: optional OpenBotInfo bot
}

struct UpdateOpenBotReq {
    1: string app_id
    2: i64 bot_id
    3: i64 tenant_id
    4: optional string name
    5: optional string system_prompt
    6: optional string api_key
    7: optional string model
    8: optional string base_url
}

struct DeleteOpenBotReq {
    1: string app_id
    2: i64 bot_id
    3: i64 tenant_id
}

struct AddOpenBotConversationReq {
    1: string app_id
    2: i64 bot_id
    3: i64 operator_id
    4: i64 conversation_id
    5: i16 conversation_type
    6: i64 tenant_id
}

struct AddOpenBotConversationRes {
    1: bool success
    2: i64 conversation_id
}

// ===== 消息与会话 =====

struct OpenSendMessageReq {
    1: string app_id
    2: i64 bot_id
    3: i64 conversation_id
    4: i64 peer_id
    5: string content
    6: i64 tenant_id
}

struct OpenSendMessageRes {
    1: bool success
    2: i64 msg_id
    3: i64 conversation_id
}

struct OpenConversationInfo {
    1: i64 conversation_id
    2: i16 type
    3: string name
    4: list<i64> member_ids
    5: i64 tenant_id
}

struct ListOpenConversationsReq {
    1: string app_id
    2: i64 bot_id
    3: i64 tenant_id
}

struct ListOpenConversationsRes {
    1: bool success
    2: list<OpenConversationInfo> conversations
}

struct OpenMessage {
    1: i64 msg_id
    2: i64 sender_id
    3: i64 conversation_id
    4: string content
    5: i64 timestamp
    6: optional string sender_name
}

struct ListOpenMessagesReq {
    1: string app_id
    2: i64 bot_id
    3: i64 conversation_id
    4: i64 before_msg_id
    5: i16 limit
    6: i64 tenant_id
}

struct ListOpenMessagesRes {
    1: bool success
    2: list<OpenMessage> messages
}

// ===== 事件：轮询与回调共用 =====

struct OpenEventInfo {
    1: i64 id
    2: string app_id
    3: i64 bot_id
    4: string event_type
    5: string payload
    6: i16 status
    7: i64 created_at
}

struct ListEventsReq {
    1: string app_id
    2: optional i64 since_id
    3: optional i32 limit
}

struct ListEventsRes {
    1: bool success
    2: list<OpenEventInfo> events
}

struct AckEventsReq {
    1: string app_id
    2: list<i64> event_ids
}

// RecordEvent 由 bot-reply 消费者调用，把 Bot 回复落为一条可投递事件。
struct RecordEventReq {
    1: i64 bot_id
    2: string event_type
    3: string payload
}

// ===== Webhook 回调 =====

struct RegisterWebhookReq {
    1: string app_id
    2: i64 bot_id
    3: string url
    4: string events
}

struct RegisterWebhookRes {
    1: bool success
    2: i64 id
    3: string secret
    4: string verify_challenge
}

struct WebhookInfo {
    1: i64 id
    2: string app_id
    3: i64 bot_id
    4: string url
    5: string events
    6: bool enabled
    7: string status
    8: i64 created_at
}

struct ListWebhooksReq {
    1: string app_id
    2: optional i64 bot_id
}

struct ListWebhooksRes {
    1: bool success
    2: list<WebhookInfo> webhooks
}

struct DeleteWebhookReq {
    1: string app_id
    2: i64 id
}

struct GetWebhookReq {
    1: string app_id
    2: i64 id
}

struct GetWebhookRes {
    1: bool success
    2: optional WebhookInfo webhook
    3: optional string secret
}

struct RecordDeliveryReq {
    1: i64 webhook_id
    2: i64 event_id
    3: i16 status
    4: i32 attempts
    5: string last_error
}

service OpenService {
    // 应用管理
    CreateAppRes CreateApp(1: CreateAppReq req)
    ListAppsRes ListApps(1: ListAppsReq req)
    GetAppRes GetApp(1: GetAppReq req)
    VerifyAppRes VerifyApp(1: VerifyAppReq req)
    AuthenticateAppRes AuthenticateApp(1: AuthenticateAppReq req)
    ResetAppSecretRes ResetAppSecret(1: ResetAppSecretReq req)
    CommonRes UpdateAppStatus(1: UpdateAppStatusReq req)

    // Bot 管理
    CreateOpenBotRes CreateBot(1: CreateOpenBotReq req)
    ListOpenBotsRes ListBots(1: ListOpenBotsReq req)
    GetOpenBotRes GetBot(1: GetOpenBotReq req)
    CommonRes UpdateBot(1: UpdateOpenBotReq req)
    CommonRes DeleteBot(1: DeleteOpenBotReq req)
    AddOpenBotConversationRes AddBotToConversation(1: AddOpenBotConversationReq req)

    // 消息与会话
    OpenSendMessageRes SendMessage(1: OpenSendMessageReq req)
    ListOpenMessagesRes ListMessages(1: ListOpenMessagesReq req)
    ListOpenConversationsRes ListConversations(1: ListOpenConversationsReq req)

    // 事件
    ListEventsRes ListEvents(1: ListEventsReq req)
    CommonRes AckEvents(1: AckEventsReq req)
    CommonRes RecordEvent(1: RecordEventReq req)

    // 回调
    RegisterWebhookRes RegisterWebhook(1: RegisterWebhookReq req)
    ListWebhooksRes ListWebhooks(1: ListWebhooksReq req)
    GetWebhookRes GetWebhook(1: GetWebhookReq req)
    CommonRes DeleteWebhook(1: DeleteWebhookReq req)
    CommonRes RecordDelivery(1: RecordDeliveryReq req)
}
