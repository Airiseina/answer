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
}

struct GetBotReq {
    1: i64 bot_id
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
}

struct DeleteBotReq {
    1: i64 bot_id
    2: i64 operator_id
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
}

struct GetBotConfigRes {
    1: bool success
    2: optional string api_key
    3: optional string model
    4: optional string system_prompt
    5: optional i64 user_id
    6: optional string base_url
}

struct AddBotToConversationReq {
    1: i64 operator_id
    2: i64 bot_id
    3: i64 conversation_id
    4: i16 conversation_type
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
}

struct CreateMcpServerRes {
    1: bool success
    2: i64 id
}

struct GetBotMcpServersReq {
    1: i64 bot_id
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
}

struct DeleteMcpServerReq {
    1: i64 id
    2: i64 operator_id
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
}
