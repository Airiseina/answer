namespace go chat

struct CommonRes {
    1: bool success
}

struct SendMessageReq {
    1: i64 sender_id
    2: i64 conversation_id
    3: i64 peer_id
    4: string content
    5: i64 client_seq
}

struct SendMessageRes {
    1: bool success
    2: i64 msg_id
    3: i64 timestamp
    4: i64 conversation_id
    5: list<i64> member_ids
}

struct Message {
    1: i64 msg_id
    2: i64 client_seq
    3: i64 sender_id
    4: i64 conversation_id
    5: string content
    6: i64 timestamp
}

struct GetHistoryReq {
    1: i64 user_id            // 请求者用户ID，用于成员身份校验
    2: i64 conversation_id
    3: i64 before_msg_id
    4: i16 limit
}

struct GetHistoryRes {
    1: bool success
    2: list<Message> messages
}

struct CreateConversationReq {
    1: string name
    2: list<i64> member_ids
    3: optional i64 group_id  // 关联的群组ID，由 group_service 创建群组时传入
}

struct CreateConversationRes {
    1: bool success
    2: i64 conversation_id
}

struct GetConversationsReq {
    1: i64 user_id
}

struct ConversationInfo {
    1: i64 conversation_id
    2: i16 type
    3: string name
    4: list<i64> member_ids
    5: optional i64 group_id  // 群聊会话关联的群组ID，单聊时为空
}

struct GetConversationsRes {
    1: bool success
    2: list<ConversationInfo> conversations
}

struct SetOnlineReq {
    1: i64 user_id
    2: string gateway_addr
}

struct SetOfflineReq {
    1: i64 user_id
}

struct GetOnlineStatusReq {
    1: list<i64> user_ids
}

struct OnlineStatus {
    1: i64 user_id
    2: bool online
    3: string gateway_addr
}

struct GetOnlineStatusRes {
    1: list<OnlineStatus> statuses
}

// ===== 会话成员管理接口（供 group_service RPC 调用同步数据） =====

// 添加会话成员请求
// 由 group_service 在邀请成员入群后调用，同步 conversation_member 数据
struct AddConversationMembersReq {
    1: i64 conversation_id   // 目标会话ID
    2: list<i64> member_ids  // 待添加的用户ID列表
}

// 添加会话成员响应
struct AddConversationMembersRes {
    1: bool success
}

// 移除会话成员请求
// 由 group_service 在踢出成员后调用，同步 conversation_member 数据
struct RemoveConversationMembersReq {
    1: i64 conversation_id   // 目标会话ID
    2: list<i64> member_ids  // 待移除的用户ID列表
}

// 移除会话成员响应
struct RemoveConversationMembersRes {
    1: bool success
}

service ChatService {
    SendMessageRes SendMessage(1: SendMessageReq req)
    GetHistoryRes GetHistory(1: GetHistoryReq req)
    CreateConversationRes CreateConversation(1: CreateConversationReq req)
    GetConversationsRes GetConversations(1: GetConversationsReq req)
    CommonRes SetOnline(1: SetOnlineReq req)
    CommonRes SetOffline(1: SetOfflineReq req)
    GetOnlineStatusRes GetOnlineStatus(1: GetOnlineStatusReq req)
    // 会话成员管理：供 group_service 同步群组成员变更
    AddConversationMembersRes AddConversationMembers(1: AddConversationMembersReq req)
    RemoveConversationMembersRes RemoveConversationMembers(1: RemoveConversationMembersReq req)
}
