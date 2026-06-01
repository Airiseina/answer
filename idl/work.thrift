namespace go work

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

struct TranslateMessageReq {
    1: string content
    2: string target_lang
}

struct TranslateMessageRes {
    1: bool success
    2: string translated_content
}

service WorkService {
    SummarizeConversationRes SummarizeConversation(1: SummarizeConversationReq req)
    SuggestRepliesRes SuggestReplies(1: SuggestRepliesReq req)
    TranslateMessageRes TranslateMessage(1: TranslateMessageReq req)
}
