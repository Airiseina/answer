namespace go chat

struct ChatReq{
1:i16 id,
2:i16 sessionId,
3:string content,
4:string role,
}

struct ChatHistory{
1:i16 id,
2:i16 sessionId,
}

struct ChatRecord{
1:i16 id,
2:i16 sessionId,
3:string question
4:string answer,
5:i64 time,
}

struct ChatHistoryRes{
1: list<ChatRecord> records,
2: string role_setting,
3: string title,
}

struct ChatRes{
1:i16 id,
2:i16 sessionId,
3:string content,
}


service Chat{
ChatRes Chat(1:ChatReq req)
ChatHistoryRes ChatHis(1:ChatHistory req)
}