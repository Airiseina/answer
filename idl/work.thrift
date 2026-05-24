namespace go work

struct HandleMessageReq {
    1: i64 bot_id
    2: i64 conversation_id
    3: i64 sender_id
    4: string content
    5: list<string> history
}

struct HandleMessageRes {
    1: bool success
    2: string msg_id
}

service WorkService {
    HandleMessageRes HandleMessage(1: HandleMessageReq req)
}
