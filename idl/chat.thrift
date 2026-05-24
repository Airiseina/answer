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
    6: optional string content
    7: optional i16 conversation_type
    8: optional i64 seq  // 会话内消息序号，用于未读数计算
}

struct Message {
    1: i64 msg_id
    2: i64 client_seq
    3: i64 sender_id
    4: i64 conversation_id
    5: string content
    6: i64 timestamp
    7: optional string sender_name
    8: optional i64 seq  // 会话内消息序号
    9: optional i16 status  // 消息状态：0=正常，1=已撤回
    10: optional bool is_edited  // 是否已编辑
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
    6: optional i64 max_seq       // 会话当前最大消息序号
    7: optional i64 max_read_seq  // 用户在该会话中的已读序号
    8: optional i64 unread_count  // 未读消息数
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

struct RenewOnlineReq {
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

struct DeleteConversationReq {
    1: i64 conversation_id
}

struct DeleteConversationRes {
    1: bool success
}

// 标记会话已读请求
// 客户端打开会话时调用，将用户在该会话中的已读位置更新到当前最大消息序号
struct MarkReadReq {
    1: i64 user_id           // 用户ID
    2: i64 conversation_id   // 会话ID
}

// 标记会话已读响应
struct MarkReadRes {
    1: bool success
    2: optional i64 max_read_seq  // 更新后的已读序号
}

// 查询会话成员请求
// 用于 typing 等场景下获取会话成员列表，以便向在线成员推送事件
struct GetConversationMembersReq {
    1: i64 conversation_id
}

// 查询会话成员响应
struct GetConversationMembersRes {
    1: bool success
    2: list<i64> member_ids
}

struct GetOrCreatePrivateConversationReq {
    1: i64 user_id_a
    2: i64 user_id_b
}

struct GetOrCreatePrivateConversationRes {
    1: bool success
    2: i64 conversation_id
}

// 撤回消息请求
// 发送者在 2 分钟内可撤回自己发送的消息
struct RecallMessageReq {
    1: i64 user_id           // 请求者用户ID，用于权限校验
    2: i64 msg_id            // 待撤回的消息ID
    3: i64 conversation_id   // 会话ID，用于推送通知
}

// 撤回消息响应
struct RecallMessageRes {
    1: bool success
    2: optional i64 conversation_id  // 消息所属会话ID
    3: optional list<i64> member_ids // 会话成员列表，用于推送
}

// 编辑消息请求
// 发送者可编辑自己发送的消息内容
struct EditMessageReq {
    1: i64 user_id           // 请求者用户ID，用于权限校验
    2: i64 msg_id            // 待编辑的消息ID
    3: i64 conversation_id   // 会话ID，用于推送通知
    4: string new_content    // 新的消息内容
}

// 编辑消息响应
struct EditMessageRes {
    1: bool success
    2: optional i64 conversation_id  // 消息所属会话ID
    3: optional list<i64> member_ids // 会话成员列表，用于推送
}

// 编辑历史记录
struct EditHistoryItem {
    1: i64 id
    2: i64 msg_id
    3: i32 version
    4: string old_content
    5: i64 editor_id
    6: i64 edited_at
}

// 查询编辑历史请求
struct GetEditHistoryReq {
    1: i64 user_id           // 请求者用户ID，用于权限校验
    2: i64 msg_id            // 消息ID
    3: i64 conversation_id   // 会话ID，用于权限校验
}

// 查询编辑历史响应
struct GetEditHistoryRes {
    1: bool success
    2: list<EditHistoryItem> histories
}

// ===== Phase 6: 云端漫游与上线同步 =====

// 会话序号对：客户端上报每个会话本地已同步的最大 seq
// 服务端据此拉取 seq > last_seq 的消息下推给客户端
struct ConvSeqPair {
    1: i64 conversation_id   // 会话ID
    2: i64 last_seq          // 客户端本地该会话的最大已同步seq
}

// 同步消息请求
// 客户端断线重连后，遍历本地所有会话，携带每个会话的本地最大 seq
// 服务端对每个会话拉取 seq > last_seq 的消息返回
struct SyncMessagesReq {
    1: i64 user_id                    // 请求者用户ID，用于成员身份校验
    2: list<ConvSeqPair> conv_seqs    // 各会话的本地最大seq
    3: i16 limit                      // 每个会话最多返回的消息条数，默认50，上限200
}

// 单个会话的同步消息结果
struct ConvMessages {
    1: i64 conversation_id            // 会话ID
    2: list<Message> messages         // seq > last_seq 的消息列表，按 seq 升序
}

// 同步消息响应
struct SyncMessagesRes {
    1: bool success
    2: list<ConvMessages> conv_messages  // 各会话的同步消息结果
}

service ChatService {
    SendMessageRes SendMessage(1: SendMessageReq req)
    GetHistoryRes GetHistory(1: GetHistoryReq req)
    CreateConversationRes CreateConversation(1: CreateConversationReq req)
    GetConversationsRes GetConversations(1: GetConversationsReq req)
    CommonRes SetOnline(1: SetOnlineReq req)
    CommonRes SetOffline(1: SetOfflineReq req)
    CommonRes RenewOnline(1: RenewOnlineReq req)
    GetOnlineStatusRes GetOnlineStatus(1: GetOnlineStatusReq req)
    // 会话成员管理：供 group_service 同步群组成员变更
    AddConversationMembersRes AddConversationMembers(1: AddConversationMembersReq req)
    RemoveConversationMembersRes RemoveConversationMembers(1: RemoveConversationMembersReq req)
    DeleteConversationRes DeleteConversation(1: DeleteConversationReq req)
    // 标记会话已读：更新用户的已读序号，用于未读数计算
    MarkReadRes MarkRead(1: MarkReadReq req)
    // 查询会话成员列表：用于 typing 等场景获取推送目标
    GetConversationMembersRes GetConversationMembers(1: GetConversationMembersReq req)
    GetOrCreatePrivateConversationRes GetOrCreatePrivateConversation(1: GetOrCreatePrivateConversationReq req)
    // 撤回消息：2 分钟内可撤回，更新 PG 状态并通过 WS 通知对方
    RecallMessageRes RecallMessage(1: RecallMessageReq req)
    // 编辑消息：更新 PG 内容并标记 is_edited，通过 WS 通知对方
    EditMessageRes EditMessage(1: EditMessageReq req)
    // 查询编辑历史：返回消息的所有历史版本
    GetEditHistoryRes GetEditHistory(1: GetEditHistoryReq req)
    // 同步消息：客户端断线重连后按会话维度拉取缺失消息
    SyncMessagesRes SyncMessages(1: SyncMessagesReq req)
}
