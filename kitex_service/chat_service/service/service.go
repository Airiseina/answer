package service

import (
	"answer/kitex_service/chat_service/dal/mysql"
)

type ChatService struct {
	dao mysql.ChatDao
}

func NewChatService(dao mysql.ChatDao) *ChatService {
	return &ChatService{dao}
}
