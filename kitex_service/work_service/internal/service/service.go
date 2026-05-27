package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/agent"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/llm"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/mcp"
	"github.com/Airiseina/answer/kitex_service/work_service/rpc"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/segmentio/kafka-go"
)

const botReplyTopic = "bot-reply-topic"

type WorkService struct {
	llmClient   *llm.Client
	kafkaWriter *kafka.Writer
	mcpPool     *mcp.Pool
	agent       *agent.Agent
}

func NewWorkService(llmClient *llm.Client, kafkaWriter *kafka.Writer, mcpPool *mcp.Pool) *WorkService {
	return &WorkService{
		llmClient:   llmClient,
		kafkaWriter: kafkaWriter,
		mcpPool:     mcpPool,
		agent:       agent.NewAgent(mcpPool),
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

	mcpServers := mcp.GetMcpServersForBot(ctx, botId, rpc.GetBotMcpServers)
	botMcpServers := mcpServers
	allMcpServers := append(mcp.GetBuiltinServerConfigs(), botMcpServers...)

	var result string
	if len(allMcpServers) > 0 {
		result = svc.handleWithAgent(ctx, botCfg, allMcpServers, conversationId, senderId, content, history)
	} else {
		var chatHistory []llm.ChatMessage
		for i, h := range history {
			role := "assistant"
			if i%2 == 0 {
				role = "user"
			}
			chatHistory = append(chatHistory, llm.ChatMessage{Role: role, Content: h})
		}
		llmResult, llmErr := svc.llmClient.Chat(ctx, botCfg.ApiKey, botCfg.BaseUrl, botCfg.Model, botCfg.SystemPrompt, chatHistory, content)
		if llmErr != nil {
			return false, fmt.Errorf("Bot[%d]调用LLM失败: %w", botId, llmErr)
		}
		result = llmResult
	}

	sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sendResp, sendErr := rpc.SendMessage(sendCtx, &chat.SendMessageReq{
		SenderId:       botCfg.UserID,
		ConversationId: conversationId,
		PeerId:         0,
		Content:        result,
	})
	if sendErr != nil {
		return false, fmt.Errorf("bot[%d]消息入库失败: %w", botId, sendErr)
	}
	if svc.kafkaWriter != nil {
		reply := map[string]interface{}{
			"msg_id":            sendResp.GetMsgId(),
			"seq":               sendResp.GetSeq(),
			"conversation_id":   sendResp.GetConversationId(),
			"conversation_type": sendResp.GetConversationType(),
			"sender_id":         botCfg.UserID,
			"content":           result,
			"timestamp":         sendResp.GetTimestamp(),
			"member_ids":        sendResp.GetMemberIds(),
		}
		replyJSON, _ := json.Marshal(reply)
		if writeErr := svc.kafkaWriter.WriteMessages(context.Background(), kafka.Message{
			Topic: botReplyTopic,
			Key:   []byte(fmt.Sprintf("%d", sendResp.GetConversationId())),
			Value: replyJSON,
		}); writeErr != nil {
			klog.Errorf("Bot[%d]回复推送消息写入Kafka失败: %v", botId, writeErr)
		}
	}
	return true, nil
}

func (svc *WorkService) handleWithAgent(ctx context.Context, botCfg *rpc.BotConfig, mcpServers []mcp.ServerConfig, conversationId, senderId int64, content string, history []string) string {
	userID := fmt.Sprintf("%d", senderId)
	runID := fmt.Sprintf("%d", conversationId)

	memories := mcp.SearchMemories(ctx, svc.mcpPool, content, userID, 3)
	enhancedPrompt := botCfg.SystemPrompt + mcp.BuildMemoryPrompt(memories)

	var einoHistory []*schema.Message
	for i, h := range history {
		role := schema.Assistant
		if i%2 == 0 {
			role = schema.User
		}
		einoHistory = append(einoHistory, &schema.Message{Role: role, Content: h})
	}

	agentResult, err := svc.agent.Run(ctx, agent.AgentRunConfig{
		APIKey:       botCfg.ApiKey,
		BaseURL:      botCfg.BaseUrl,
		Model:        botCfg.Model,
		SystemPrompt: enhancedPrompt,
		McpServers:   mcpServers,
		History:      einoHistory,
		UserContent:  content,
	})
	if err != nil {
		klog.Errorf("Agent执行失败，降级为普通LLM调用: %v", err)
		var chatHistory []llm.ChatMessage
		for i, h := range history {
			role := "assistant"
			if i%2 == 0 {
				role = "user"
			}
			chatHistory = append(chatHistory, llm.ChatMessage{Role: role, Content: h})
		}
		llmResult, llmErr := svc.llmClient.Chat(ctx, botCfg.ApiKey, botCfg.BaseUrl, botCfg.Model, botCfg.SystemPrompt, chatHistory, content)
		if llmErr != nil {
			return fmt.Sprintf("抱歉，处理消息时出错: %v", err)
		}
		return llmResult
	}

	go func() {
		saveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		memoryContent := fmt.Sprintf("用户: %s | 助手: %s", content, agentResult)
		mcp.SaveMemory(saveCtx, svc.mcpPool, memoryContent, userID, runID)
	}()

	return agentResult
}
