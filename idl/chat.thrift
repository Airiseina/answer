namespace go chat

struct CommonRes {
    1: bool success
}

struct SendMessageReq {
    1: i64 sender_id
    2: i64 receiver_id
    3: string content
    4: i64 client_seq
}

struct SendMessageRes {
    1: bool success
    2: string reason
    3: i64 msg_id
    4: i64 timestamp
}

struct Message {
    1: i64 msg_id
    2: i64 client_seq
    3: i64 sender_id
    4: i64 receiver_id
    5: string content
    6: i64 timestamp
}

struct GetHistoryReq {
    1: i64 user_id
    2: i64 peer_id
    3: i64 before_msg_id
    4: i16 limit
}

struct GetHistoryRes {
    1: bool success
    2: string reason
    3: list<Message> messages
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
    3: string gateway_addr//这是什么？
}

struct GetOnlineStatusRes {
    1: list<OnlineStatus> statuses
}

service ChatService {
    SendMessageRes SendMessage(1: SendMessageReq req)
    GetHistoryRes GetHistory(1: GetHistoryReq req)
    CommonRes SetOnline(1: SetOnlineReq req)
    CommonRes SetOffline(1: SetOfflineReq req)
    GetOnlineStatusRes GetOnlineStatus(1: GetOnlineStatusReq req)
}
