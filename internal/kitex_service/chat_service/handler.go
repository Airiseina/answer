package main

import (
	"answer/internal/kitex_service/chat_service/kitex_gen/chat"
	"context"
)

// ChatImpl implements the last service interface defined in the IDL.
type ChatImpl struct{}

// Chat implements the ChatImpl interface.
func (s *ChatImpl) Chat(ctx context.Context, req *chat.ChatReq) (resp *chat.ChatRes, err error) {
	// TODO: Your code here...
	return
}

// ChatHis implements the ChatImpl interface.
func (s *ChatImpl) ChatHis(ctx context.Context, req *chat.ChatHistory) (resp *chat.ChatHistoryRes, err error) {
	// TODO: Your code here...
	return
}
