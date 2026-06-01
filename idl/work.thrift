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

service WorkService {
    HandleMessageRes HandleMessage(1: HandleMessageReq req)
    SummarizeConversationRes SummarizeConversation(1: SummarizeConversationReq req)
    SuggestRepliesRes SuggestReplies(1: SuggestRepliesReq req)
}
