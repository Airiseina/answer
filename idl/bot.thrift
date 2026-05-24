namespace go bot

struct CommonRes {
    1: bool success
}

struct CreateBotReq {
    1: i64 creator_id
    2: string name
    3: string avatar_url
    4: string system_prompt
    5: string api_key
    6: string model
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
}
