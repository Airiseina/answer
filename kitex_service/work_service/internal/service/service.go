package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/agent"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/llm"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/mcp"
	"github.com/Airiseina/answer/kitex_service/work_service/rpc"
	"github.com/Airiseina/answer/pkg/meter"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/segmentio/kafka-go"
)

const botReplyTopic = "bot-reply-topic"

const (
	mcpTimeout        = 60 * time.Second
	llmTimeout        = 180 * time.Second
	memorySaveTimeout = 60 * time.Second
)

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
	start := time.Now()
	botCfg, err := rpc.GetBotConfig(ctx, botId)
	if err != nil {
		return false, fmt.Errorf("获取Bot[%d]配置失败: %w", botId, err)
	}
	meter.M.BotRequestTotal.Add(ctx, 1, metric.WithAttributes(attribute.Int64("bot_id", botId)))
	var isGroupChat bool
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
		convType, convErr := rpc.GetConversationType(ctx, conversationId, senderId)
		if convErr != nil {
			klog.CtxWarnf(ctx, "获取会话[%d]类型失败，按群聊处理: %v", conversationId, convErr)
			isGroupChat = true
		} else {
			isGroupChat = convType != rpc.ConvTypePrivate || len(members) > 2
		}
	}
	if botCfg.UserID > 0 && botCfg.Name == "" {
		botCfg.Name = rpc.GetUserName(ctx, botCfg.UserID)
	}
	mcpServers := mcp.GetMcpServersForBot(ctx, botId, rpc.GetBotMcpServers)
	botMcpServers := mcpServers
	allMcpServers := append(mcp.GetBuiltinServerConfigs(), botMcpServers...)
	var result string
	if len(allMcpServers) > 0 {
		result = svc.handleWithAgent(ctx, botCfg, allMcpServers, conversationId, senderId, content, history, isGroupChat)
	} else {
		llmCtx, llmCancel := context.WithTimeout(ctx, llmTimeout)
		defer llmCancel()
		var chatHistory []llm.ChatMessage
		for i, h := range history {
			role := "assistant"
			if i%2 == 0 {
				role = "user"
			}
			chatHistory = append(chatHistory, llm.ChatMessage{Role: role, Content: h})
		}
		llmResult, llmErr := svc.llmClient.Chat(llmCtx, botCfg.ApiKey, botCfg.BaseUrl, botCfg.Model, botCfg.SystemPrompt, chatHistory, content)
		if llmErr != nil {
			return false, fmt.Errorf("Bot[%d]调用LLM失败: %w", botId, llmErr)
		}
		result = llmResult
	}
	if ctx.Err() != nil {
		return false, fmt.Errorf("Bot[%d]处理消息超时，跳过发送", botId)
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
	meter.M.BotResponseLatency.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.Int64("bot_id", botId)))
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

func (svc *WorkService) handleWithAgent(ctx context.Context, botCfg *rpc.BotConfig, mcpServers []mcp.ServerConfig, conversationId, senderId int64, content string, history []string, isGroupChat bool) string {
	userID := fmt.Sprintf("%d", senderId)
	runID := fmt.Sprintf("%d", conversationId)
	searchRunID := ""
	if isGroupChat {
		searchRunID = runID
	}
	memoryCtx, memoryCancel := context.WithTimeout(ctx, mcpTimeout)
	memories := mcp.SearchMemories(memoryCtx, svc.mcpPool, content, userID, searchRunID, 3)
	memoryCancel()
	enhancedPrompt := botCfg.SystemPrompt + mcp.BuildMemoryPrompt(memories)
	knowledgeCtx, knowledgeCancel := context.WithTimeout(ctx, mcpTimeout)
	kbResult := mcp.GetBotKnowledgeBases(knowledgeCtx, svc.mcpPool, fmt.Sprintf("%d", botCfg.UserID))
	knowledgeCancel()
	if kbResult != "" {
		var kbList struct {
			Success        bool `json:"success"`
			KnowledgeBases []struct {
				KbId int64 `json:"kb_id"`
			} `json:"knowledge_bases"`
		}
		if err := json.Unmarshal([]byte(kbResult), &kbList); err == nil && kbList.Success && len(kbList.KnowledgeBases) > 0 {
			var kbIDs []string
			for _, kb := range kbList.KnowledgeBases {
				kbIDs = append(kbIDs, fmt.Sprintf("%d", kb.KbId))
			}
			searchCtx, searchCancel := context.WithTimeout(ctx, mcpTimeout)
			knowledgeResult := mcp.SearchKnowledge(searchCtx, svc.mcpPool, content, strings.Join(kbIDs, ","), 5)
			searchCancel()
			enhancedPrompt += mcp.BuildKnowledgePrompt(knowledgeResult)
		}
	}
	if botCfg.Name != "" {
		enhancedPrompt += fmt.Sprintf("\n\n你的名字是「%s」，当用户用@提及你时（如@%s），他们就是在和你说话。请自然地回应。", botCfg.Name, botCfg.Name)
	}
	var einoHistory []*schema.Message
	for i, h := range history {
		role := schema.Assistant
		if i%2 == 0 {
			role = schema.User
		}
		einoHistory = append(einoHistory, &schema.Message{Role: role, Content: h})
	}
	agentCtx, agentCancel := context.WithTimeout(ctx, llmTimeout)
	defer agentCancel()
	agentResult, err := svc.agent.Run(agentCtx, agent.AgentRunConfig{
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
		llmCtx, llmCancel := context.WithTimeout(ctx, llmTimeout)
		defer llmCancel()
		var chatHistory []llm.ChatMessage
		for i, h := range history {
			role := "assistant"
			if i%2 == 0 {
				role = "user"
			}
			chatHistory = append(chatHistory, llm.ChatMessage{Role: role, Content: h})
		}
		llmResult, llmErr := svc.llmClient.Chat(llmCtx, botCfg.ApiKey, botCfg.BaseUrl, botCfg.Model, botCfg.SystemPrompt, chatHistory, content)
		if llmErr != nil {
			klog.CtxErrorf(ctx, "处理消息出错：%v", llmErr)
			return fmt.Sprint("啊？你说啥😶?抱歉我没有听清,能再重复一遍吗(*/ω＼*)?")
		}
		return llmResult
	}
	go func() {
		saveCtx, cancel := context.WithTimeout(context.Background(), memorySaveTimeout)
		defer cancel()
		memoryContent := fmt.Sprintf("用户: %s | 助手: %s", content, agentResult)
		saveRunID := ""
		if isGroupChat {
			saveRunID = runID
		}
		mcp.SaveMemory(saveCtx, svc.mcpPool, memoryContent, userID, saveRunID)
	}()
	return agentResult
}
