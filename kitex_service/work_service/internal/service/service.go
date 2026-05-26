package service

import (
	"context"
	"fmt"
	"time"

	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/llm"
	"github.com/Airiseina/answer/kitex_service/work_service/rpc"
)

type WorkService struct {
	llmClient *llm.Client
}

func NewWorkService(llmClient *llm.Client) *WorkService {
	return &WorkService{
		llmClient: llmClient,
	}
}

func (svc *WorkService) HandleMessage(ctx context.Context, botId, conversationId, senderId int64, content string, history []string) (bool, error) {
	botCfg, err := rpc.GetBotConfig(ctx, botId)
	if err != nil {
		return false, fmt.Errorf("获取Bot[%d]配置失败: %w", botId, err)
	}
	if botCfg.UserID > 0 {
		members, memberErr := rpc.GetConversationMembers(ctx, conversationId)
		if memberErr != nil {
			return false, fmt.Errorf("查询会话[%d]成员失败: %w", conversationId, memberErr)
		}
		inConv := false
		for _, m := range members {
			if m == botCfg.UserID {
				inConv = true
				break
			}
		}
		if !inConv {
			return false, fmt.Errorf("Bot[%d]不在会话[%d]中，请先将Bot拉入会话", botId, conversationId)
		}
	}
	var chatHistory []llm.ChatMessage
	for i, h := range history {
		role := "assistant"
		if i%2 == 0 {
			role = "user"
		}
		chatHistory = append(chatHistory, llm.ChatMessage{Role: role, Content: h})
	}
	result, llmErr := svc.llmClient.Chat(ctx, botCfg.ApiKey, botCfg.BaseUrl, botCfg.Model, botCfg.SystemPrompt, chatHistory, content)
	if llmErr != nil {
		return false, fmt.Errorf("Bot[%d]调用LLM失败: %w", botId, llmErr)
	}
	sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, sendErr := rpc.SendMessage(sendCtx, &chat.SendMessageReq{
		SenderId:       botCfg.UserID,
		ConversationId: conversationId,
		PeerId:         0,
		Content:        result,
	})
	if sendErr != nil {
		return false, fmt.Errorf("bot[%d]消息入库失败: %w", botId, sendErr)
	}
	return true, nil
}
