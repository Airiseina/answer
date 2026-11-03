namespace go bot

struct CommonRes {
    1: bool success
}

struct CreateBotReq {
    1: i64 creator_id
    2: string name
    3: string system_prompt
    4: string api_key
    5: string model
    6: optional string base_url
    7: optional i64 tenant_id
    8: optional i16 origin   // 0=用户自建 1=市场安装 2=OpenAPI 接入（2 不计入用量统计）
    9: optional string app_id // OpenAPI 接入时绑定的应用 ID
}

struct CreateBotRes {
    1: bool success
    2: i64 bot_id
}

struct BotInfo {
    1: i64 bot_id
    2: i64 creator_id
    3: string name
    4: string avatar_url
    5: string system_prompt
    6: string model
    7: bool is_system
    8: i64 created_at
    9: optional string base_url
    10: optional i16 origin   // 0=用户自建 1=市场安装 2=OpenAPI 接入
    11: optional string app_id // origin=2 时绑定的开放平台应用 ID
}

struct GetBotReq {
    1: i64 bot_id
    2: optional i64 tenant_id
}

struct GetBotRes {
    1: bool success
    2: optional BotInfo bot_info
}



struct GetSystemBotRes {
    1: bool success
    2: i64 bot_id
}

struct GetUserBotsReq {
    1: i64 creator_id
    2: optional i64 tenant_id
}

struct GetUserBotsRes {
    1: bool success
    2: list<BotInfo> bots
}

struct UpdateBotReq {
    1: i64 bot_id
    2: i64 operator_id
    3: optional string name
    4: optional string avatar_url
    5: optional string system_prompt
    6: optional string api_key
    7: optional string model
    8: optional string base_url
    9: optional i64 tenant_id
}

struct DeleteBotReq {
    1: i64 bot_id
    2: i64 operator_id
    3: optional i64 tenant_id
}

struct IsBotReq {
    1: i64 user_id
}

struct IsBotRes {
    1: bool is_bot
    2: optional i64 bot_id
}

struct GetBotConfigReq {
    1: i64 bot_id
    2: optional i64 tenant_id
}

struct GetBotConfigRes {
    1: bool success
    2: optional string api_key
    3: optional string model
    4: optional string system_prompt
    5: optional i64 user_id
    6: optional string base_url
    7: optional i16 origin   // 供用量统计判断是否为 OpenAPI 接入
}

struct AddBotToConversationReq {
    1: i64 operator_id
    2: i64 bot_id
    3: i64 conversation_id
    4: i16 conversation_type
    5: optional i64 tenant_id
}

struct AddBotToConversationRes {
    1: bool success
    2: optional i64 conversation_id
}

struct McpServerInfo {
    1: i64 id
    2: i64 bot_id
    3: string name
    4: string description
    5: string transport
    6: string url
    7: string auth_type
    8: optional string auth_token
    9: bool enabled
    10: i64 created_at
}

struct CreateMcpServerReq {
    1: i64 operator_id
    2: i64 bot_id
    3: string name
    4: string url
    5: optional string description
    6: optional string transport
    7: optional string auth_type
    8: optional string auth_token
    9: optional i64 tenant_id
}

struct CreateMcpServerRes {
    1: bool success
    2: i64 id
}

struct GetBotMcpServersReq {
    1: i64 bot_id
    2: optional i64 tenant_id
}

struct GetBotMcpServersRes {
    1: bool success
    2: list<McpServerInfo> servers
}

struct UpdateMcpServerReq {
    1: i64 id
    2: i64 operator_id
    3: optional string name
    4: optional string description
    5: optional string transport
    6: optional string url
    7: optional string auth_type
    8: optional string auth_token
    9: optional bool enabled
    10: optional i64 tenant_id
}

struct DeleteMcpServerReq {
    1: i64 id
    2: i64 operator_id
    3: optional i64 tenant_id
}

// ==================== Bot 市场（需求 3） ====================
//
// 市场只分发「Bot 定义」（名称/描述/系统提示词/推荐模型），
// 刻意不包含知识库、MCP 等创作者私有资产，安装者需自带 api_key/model/base_url。

// 市场 Bot 审核状态机：0=草稿 1=待审核 2=已发布 3=已驳回 4=已下架
struct MarketBotInfo {
    1: i64 market_bot_id
    2: i64 publisher_id
    3: string publisher_name
    4: string name
    5: string description
    6: string avatar_url
    7: string system_prompt
    8: string model
    9: optional string base_url
    10: string category
    11: string tags
    12: i16 status
    13: string review_note
    14: i64 install_count
    15: double rating
    16: i32 rating_count
    17: i32 version
    18: i64 created_at
    19: i64 updated_at
}

struct PublishMarketBotReq {
    1: i64 tenant_id
    2: i64 publisher_id
    3: string name
    4: string system_prompt
    5: string description
    6: optional string avatar_url
    7: optional string model
    8: optional string base_url
    9: optional string category
    10: optional string tags
}

struct PublishMarketBotRes {
    1: bool success
    2: i64 market_bot_id
    3: i16 status
}

struct UpdateMarketBotReq {
    1: i64 tenant_id
    2: i64 market_bot_id
    3: i64 operator_id
    4: optional string name
    5: optional string description
    6: optional string avatar_url
    7: optional string system_prompt
    8: optional string model
    9: optional string base_url
    10: optional string category
    11: optional string tags
}

struct GetMarketBotReq {
    1: i64 market_bot_id
    2: optional i64 viewer_id // 发布者本人可查看自己未发布的条目
    3: optional i64 tenant_id
}

struct GetMarketBotRes {
    1: bool success
    2: optional MarketBotInfo bot
}

struct ListMarketBotsReq {
    1: optional string keyword
    2: optional string category
    3: optional i32 page
    4: optional i32 page_size
}

struct ListMarketBotsRes {
    1: bool success
    2: list<MarketBotInfo> bots
    3: i32 total
}

struct ListMyMarketBotsReq {
    1: i64 tenant_id
    2: i64 publisher_id
    3: optional i16 status // 未设置返回全部
    4: optional i32 page
    5: optional i32 page_size
}

struct ListMyMarketBotsRes {
    1: bool success
    2: list<MarketBotInfo> bots
    3: i32 total
}

struct InstallMarketBotReq {
    1: i64 tenant_id
    2: i64 user_id
    3: i64 market_bot_id
    4: string name
    5: string api_key
    6: string model
    7: optional string base_url
}

struct InstallMarketBotRes {
    1: bool success
    2: i64 bot_id
    3: i32 version
}

struct RateMarketBotReq {
    1: i64 tenant_id
    2: i64 user_id
    3: i64 market_bot_id
    4: i32 score
}

struct OfflineMarketBotReq {
    1: i64 tenant_id
    2: i64 market_bot_id
    3: i64 operator_id
}

struct ReviewMarketBotReq {
    1: i64 admin_id
    2: i64 market_bot_id
    3: bool approve
    4: optional string note
}

struct ListMarketReviewsReq {
    1: optional i16 status // 未设置默认待审核（1）
    2: optional i32 page
    3: optional i32 page_size
}

struct ListMarketReviewsRes {
    1: bool success
    2: list<MarketBotInfo> bots
    3: i32 total
}

// ==================== 多 Agent 协作编排（需求 4） ====================
//
// 编排图由前端画布产出，序列化为 JSON（nodes/edges）保存在 bot_service；
// work_service 执行时按 DAG 分层调度，每个 agent 节点复用单 Agent 的 ReAct 执行器。
// status 状态机：0=草稿 1=已发布 2=已停用。

struct AgentGraphInfo {
    1: i64 graph_id
    2: i64 bot_id
    3: string name
    4: string graph_json
    5: i16 status
    6: i32 version
    7: i64 created_at
    8: i64 updated_at
}

struct AgentGraphHistoryInfo {
    1: i64 id
    2: i64 graph_id
    3: i32 version
    4: string graph_json
    5: i64 operator_id
    6: i64 created_at
}

struct SaveAgentGraphReq {
    1: i64 tenant_id
    2: i64 bot_id
    3: i64 operator_id
    4: optional i64 graph_id // 传已有图 ID 表示更新；否则新建
    5: string name
    6: string graph_json
}

struct SaveAgentGraphRes {
    1: bool success
    2: i64 graph_id
    3: i32 version
    4: i16 status
    5: list<string> errors // 校验失败时返回，供前端提示
}

struct GetAgentGraphReq {
    1: i64 tenant_id
    2: i64 bot_id
}

struct GetAgentGraphRes {
    1: bool success
    2: optional AgentGraphInfo graph
}

struct ValidateAgentGraphReq {
    1: i64 bot_id
    2: string graph_json
}

struct ValidateAgentGraphRes {
    1: bool valid
    2: list<string> errors
}

struct PublishAgentGraphReq {
    1: i64 tenant_id
    2: i64 bot_id
    3: i64 operator_id
}

struct ListAgentGraphHistoryReq {
    1: i64 tenant_id
    2: i64 bot_id
    3: optional i32 limit
}

struct ListAgentGraphHistoryRes {
    1: bool success
    2: list<AgentGraphHistoryInfo> items
}

// ==================== 个人 Skill（需求 6） ====================
//
// 本轮只支持「资料型 Skill」：一个 zip 包或单个 markdown 文件，运行期作为知识库被检索，
// 不执行任何脚本。每个 Skill 独占一个知识库，绑定到 Bot 后并入既有 search_knowledge 检索链路。
// status 状态机：0=待解析 1=可用 2=解析失败 3=已停用。

struct UserSkillInfo {
    1: i64 skill_id
    2: i64 owner_id
    3: string name
    4: string description
    5: string version
    6: i16 status
    7: i64 kb_id
    8: i64 size_bytes
    9: i32 file_count
    10: i64 created_at
    11: i64 updated_at
}

struct UserSkillFileInfo {
    1: i64 file_id
    2: i64 skill_id
    3: string rel_path
    4: string object_url
    5: i64 size_bytes
}

struct UploadUserSkillReq {
    1: i64 tenant_id
    2: i64 owner_id
    3: string file_name
    4: binary data
}

struct UploadUserSkillRes {
    1: bool success
    2: i64 skill_id
    3: i32 file_count
    4: i64 size_bytes
    5: list<string> warnings // 部分文件被跳过时返回原因，不影响整体成功
}

struct ListUserSkillsReq {
    1: i64 tenant_id
    2: i64 owner_id
}

struct ListUserSkillsRes {
    1: bool success
    2: list<UserSkillInfo> skills
}

struct GetUserSkillReq {
    1: i64 tenant_id
    2: i64 skill_id
}

struct GetUserSkillRes {
    1: bool success
    2: optional UserSkillInfo skill
    3: list<UserSkillFileInfo> files
}

struct DeleteUserSkillReq {
    1: i64 tenant_id
    2: i64 skill_id
    3: i64 operator_id
}

struct BindUserSkillReq {
    1: i64 tenant_id
    2: i64 owner_id
    3: i64 bot_id
    4: i64 skill_id
    5: bool enabled // false=解绑
}

struct BotSkillInfo {
    1: i64 skill_id
    2: string name
    3: string description
    4: string version
    5: bool enabled
}

struct ListBotSkillsReq {
    1: i64 tenant_id
    2: i64 bot_id
    3: i64 operator_id
}

struct ListBotSkillsRes {
    1: bool success
    2: list<BotSkillInfo> skills
}

service BotService {
    CreateBotRes CreateBot(1: CreateBotReq req)
    GetBotRes GetBot(1: GetBotReq req)
    GetSystemBotRes GetSystemBot()
    GetUserBotsRes GetUserBots(1: GetUserBotsReq req)
    CommonRes UpdateBot(1: UpdateBotReq req)
    CommonRes DeleteBot(1: DeleteBotReq req)
    IsBotRes IsBot(1: IsBotReq req)
    GetBotConfigRes GetBotConfig(1: GetBotConfigReq req)
    AddBotToConversationRes AddBotToConversation(1: AddBotToConversationReq req)

    CreateMcpServerRes CreateMcpServer(1: CreateMcpServerReq req)
    GetBotMcpServersRes GetBotMcpServers(1: GetBotMcpServersReq req)
    CommonRes UpdateMcpServer(1: UpdateMcpServerReq req)
    CommonRes DeleteMcpServer(1: DeleteMcpServerReq req)

    // Bot 市场
    PublishMarketBotRes PublishMarketBot(1: PublishMarketBotReq req)
    CommonRes UpdateMarketBot(1: UpdateMarketBotReq req)
    GetMarketBotRes GetMarketBot(1: GetMarketBotReq req)
    ListMarketBotsRes ListMarketBots(1: ListMarketBotsReq req)
    ListMyMarketBotsRes ListMyMarketBots(1: ListMyMarketBotsReq req)
    InstallMarketBotRes InstallMarketBot(1: InstallMarketBotReq req)
    CommonRes RateMarketBot(1: RateMarketBotReq req)
    CommonRes OfflineMarketBot(1: OfflineMarketBotReq req)
    ListMarketReviewsRes ListMarketReviews(1: ListMarketReviewsReq req)
    CommonRes ReviewMarketBot(1: ReviewMarketBotReq req)

    // 个人 Skill（资料型）
    UploadUserSkillRes UploadUserSkill(1: UploadUserSkillReq req)
    ListUserSkillsRes ListUserSkills(1: ListUserSkillsReq req)
    GetUserSkillRes GetUserSkill(1: GetUserSkillReq req)
    CommonRes DeleteUserSkill(1: DeleteUserSkillReq req)
    CommonRes BindUserSkill(1: BindUserSkillReq req)
    ListBotSkillsRes ListBotSkills(1: ListBotSkillsReq req)

    // 多 Agent 协作编排
    SaveAgentGraphRes SaveAgentGraph(1: SaveAgentGraphReq req)
    GetAgentGraphRes GetAgentGraph(1: GetAgentGraphReq req)
    ValidateAgentGraphRes ValidateAgentGraph(1: ValidateAgentGraphReq req)
    CommonRes PublishAgentGraph(1: PublishAgentGraphReq req)
    ListAgentGraphHistoryRes ListAgentGraphHistory(1: ListAgentGraphHistoryReq req)
}
