package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/agent"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/llm"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/mcp"
	"github.com/Airiseina/answer/kitex_service/work_service/rpc"
	"github.com/Airiseina/answer/pkg/observability/meter"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/segmentio/kafka-go"
)

const botReplyTopic = "bot-reply-topic"

const (
	mcpTimeout        = 60 * time.Second
	llmTimeout        = 120 * time.Second
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

func (svc *WorkService) HandleMessage(ctx context.Context, botId, conversationId, senderId int64, content string, quoteMsgID int64) (bool, error) {
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
	agentMcpServers := mcp.FilterAgentServers(append(mcp.GetBuiltinServerConfigs(), mcpServers...))
	var result string
	result = svc.handleWithAgent(ctx, botCfg, agentMcpServers, conversationId, senderId, botId, content, isGroupChat, quoteMsgID)
	if ctx.Err() != nil {
		return false, fmt.Errorf("bot[%d]处理消息超时，跳过发送", botId)
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

func (svc *WorkService) handleWithAgent(ctx context.Context, botCfg *rpc.BotConfig, mcpServers []mcp.ServerConfig, conversationId, senderId, botId int64, content string, isGroupChat bool, quoteMsgID int64) string {
	userID := fmt.Sprintf("%d", senderId)
	botID := fmt.Sprintf("%d", botId)
	runID := fmt.Sprintf("%d", conversationId)
	searchRunID := ""
	if isGroupChat {
		searchRunID = runID
	}
	memoryCtx, memoryCancel := context.WithTimeout(ctx, mcpTimeout)
	memories := mcp.SearchMemories(memoryCtx, svc.mcpPool, content, userID, botID, searchRunID, 3)
	memoryCancel()
	enhancedPrompt := botCfg.SystemPrompt + mcp.BuildMemoryPrompt(memories)
	knowledgeCtx, knowledgeCancel := context.WithTimeout(ctx, mcpTimeout)
	kbResult := mcp.GetBotKnowledgeBases(knowledgeCtx, svc.mcpPool, botID)
	knowledgeCancel()
	if kbResult != "" {
		var kbList struct {
			KnowledgeBases []struct {
				Id int64 `json:"id"`
			} `json:"knowledge_bases"`
		}
		if err := json.Unmarshal([]byte(kbResult), &kbList); err == nil && len(kbList.KnowledgeBases) > 0 {
			var kbIDs []string
			for _, kb := range kbList.KnowledgeBases {
				kbIDs = append(kbIDs, fmt.Sprintf("%d", kb.Id))
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
	userContent := content
	if quoteMsgID > 0 {
		quoteMsgs, quoteErr := rpc.GetHistory(ctx, botCfg.UserID, conversationId, 50)
		if quoteErr != nil {
			klog.CtxWarnf(ctx, "获取引用消息[%d]失败: %v", quoteMsgID, quoteErr)
		} else {
			var quoteMsg *chat.Message
			for _, m := range quoteMsgs {
				if m.MsgId == quoteMsgID {
					quoteMsg = m
					break
				}
			}
			if quoteMsg != nil {
				quoteSenderName := fmt.Sprintf("用户%d", quoteMsg.SenderId)
				if quoteMsg.SenderId == botCfg.UserID {
					quoteSenderName = botCfg.Name
				} else {
					names, nameErr := rpc.GetUserNames(ctx, []int64{quoteMsg.SenderId})
					if nameErr == nil && names != nil && len(names) > 0 && names[0].Name != "" {
						quoteSenderName = names[0].Name
					}
				}
				userContent = fmt.Sprintf("[引用 %s 的消息]: %s\n\n%s", quoteSenderName, quoteMsg.Content, content)
			}
		}
	}
	agentCtx, agentCancel := context.WithTimeout(ctx, llmTimeout)
	defer agentCancel()
	agentResult, err := svc.agent.Run(agentCtx, agent.AgentRunConfig{
		APIKey:       botCfg.ApiKey,
		BaseURL:      botCfg.BaseUrl,
		Model:        botCfg.Model,
		SystemPrompt: enhancedPrompt,
		McpServers:   mcpServers,
		UserContent:  userContent,
	})
	if err != nil {
		klog.Errorf("Agent执行失败，降级为普通LLM调用: %v", err)
		llmCtx, llmCancel := context.WithTimeout(ctx, llmTimeout)
		defer llmCancel()
		var chatHistory []llm.ChatMessage
		historyMsgs, histErr := rpc.GetHistory(ctx, botCfg.UserID, conversationId, 10)
		if histErr != nil {
			klog.CtxWarnf(ctx, "降级LLM获取会话[%d]历史消息失败: %v", conversationId, histErr)
		} else {
			senderIDs := make([]int64, 0, len(historyMsgs))
			for _, m := range historyMsgs {
				senderIDs = append(senderIDs, m.SenderId)
			}
			senderNameMap := make(map[int64]string)
			if len(senderIDs) > 0 {
				nameResp, nameErr := rpc.GetUserNames(ctx, senderIDs)
				if nameErr == nil && nameResp != nil {
					for _, u := range nameResp {
						senderNameMap[u.Id] = u.Name
					}
				}
			}
			for _, m := range historyMsgs {
				msgContent := m.Content
				if isGroupChat {
					senderName := senderNameMap[m.SenderId]
					if senderName == "" {
						senderName = fmt.Sprintf("用户%d", m.SenderId)
					}
					if m.SenderId == botCfg.UserID {
						msgContent = fmt.Sprintf("%s: %s", botCfg.Name, msgContent)
					} else {
						msgContent = fmt.Sprintf("%s: %s", senderName, msgContent)
					}
				}
				role := "user"
				if m.SenderId == botCfg.UserID {
					role = "assistant"
				}
				chatHistory = append(chatHistory, llm.ChatMessage{Role: role, Content: msgContent})
			}
		}
		llmResult, llmErr := svc.llmClient.Chat(llmCtx, botCfg.ApiKey, botCfg.BaseUrl, botCfg.Model, enhancedPrompt, chatHistory, userContent)
		if llmErr != nil {
			klog.CtxErrorf(ctx, "处理消息出错：%v", llmErr)
			return fmt.Sprint("抱歉，在月球这边接收地球的信息偶尔会有延迟呢~😭。能再重复一遍吗(*/ω＼*)?")
		}
		go func() {
			saveCtx, cancel := context.WithTimeout(context.Background(), memorySaveTimeout)
			defer cancel()
			memoryContent := fmt.Sprintf("用户: %s | 助手: %s", content, llmResult)
			saveRunID := ""
			if isGroupChat {
				saveRunID = runID
			}
			mcp.SaveMemory(saveCtx, svc.mcpPool, memoryContent, userID, botID, saveRunID)
		}()
		return "抱歉，我这边信号有点不好~我只能凭记忆给你回答了😥\n\n" + llmResult
	}
	go func() {
		saveCtx, cancel := context.WithTimeout(context.Background(), memorySaveTimeout)
		defer cancel()
		memoryContent := fmt.Sprintf("用户: %s | 助手: %s", content, agentResult)
		saveRunID := ""
		if isGroupChat {
			saveRunID = runID
		}
		mcp.SaveMemory(saveCtx, svc.mcpPool, memoryContent, userID, botID, saveRunID)
	}()
	return agentResult
}

func (svc *WorkService) getSystemBotLLMConfig(ctx context.Context) (apiKey, baseURL, model string, err error) {
	botId, err := rpc.GetSystemBot(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("获取系统Bot失败: %w", err)
	}
	botCfg, err := rpc.GetBotConfig(ctx, botId)
	if err != nil {
		return "", "", "", fmt.Errorf("获取系统Bot[%d]配置失败: %w", botId, err)
	}
	if botCfg.ApiKey == "" {
		return "", "", "", fmt.Errorf("系统Bot未配置API Key")
	}
	return botCfg.ApiKey, botCfg.BaseUrl, botCfg.Model, nil
}

func (svc *WorkService) formatHistoryMessages(ctx context.Context, userId, conversationId int64, limit int16) (string, error) {
	historyMsgs, err := rpc.GetHistory(ctx, userId, conversationId, limit)
	if err != nil {
		return "", fmt.Errorf("获取会话[%d]历史消息失败: %w", conversationId, err)
	}
	if len(historyMsgs) == 0 {
		return "", nil
	}
	senderIDs := make([]int64, 0, len(historyMsgs))
	for _, m := range historyMsgs {
		senderIDs = append(senderIDs, m.SenderId)
	}
	senderNameMap := make(map[int64]string)
	if len(senderIDs) > 0 {
		nameResp, nameErr := rpc.GetUserNames(ctx, senderIDs)
		if nameErr == nil && nameResp != nil {
			for _, u := range nameResp {
				senderNameMap[u.Id] = u.Name
			}
		}
	}
	var sb strings.Builder
	for i, m := range historyMsgs {
		senderName := senderNameMap[m.SenderId]
		if senderName == "" {
			senderName = fmt.Sprintf("用户%d", m.SenderId)
		}
		t := time.Unix(m.Timestamp, 0).Format("15:04:05")
		sb.WriteString(fmt.Sprintf("[%d| %s] %s: %s\n", i+1, t, senderName, m.Content))
	}
	return sb.String(), nil
}

func (svc *WorkService) SummarizeConversation(ctx context.Context, conversationId, userId int64) (string, error) {
	apiKey, baseURL, model, err := svc.getSystemBotLLMConfig(ctx)
	if err != nil {
		return "", err
	}
	members, memberErr := rpc.GetConversationMembers(ctx, conversationId)
	if memberErr != nil {
		return "", fmt.Errorf("查询会话[%d]成员失败: %w", conversationId, memberErr)
	}
	isMember := false
	for _, m := range members {
		if m == userId {
			isMember = true
			break
		}
	}
	if !isMember {
		return "", fmt.Errorf("用户不在会话[%d]中，无权查看", conversationId)
	}
	formattedHistory, err := svc.formatHistoryMessages(ctx, userId, conversationId, 20)
	if err != nil {
		return "", err
	}
	if formattedHistory == "" {
		return "暂无消息记录", nil
	}
	systemPrompt := "你是一个聊天记录总结助手。请根据提供的聊天记录，生成一段简洁的中文总结，概括讨论的主要话题、关键信息和结论。总结应该条理清晰、重点突出，不超过200字。"
	userContent := fmt.Sprintf("请总结以下聊天记录：\n\n%s", formattedHistory)
	llmCtx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()
	result, llmErr := svc.llmClient.Chat(llmCtx, apiKey, baseURL, model, systemPrompt, nil, userContent)
	if llmErr != nil {
		return "", fmt.Errorf("生成总结失败: %w", llmErr)
	}
	return result, nil
}

func (svc *WorkService) SuggestReplies(ctx context.Context, conversationId, userId int64) ([]string, error) {
	apiKey, baseURL, model, err := svc.getSystemBotLLMConfig(ctx)
	if err != nil {
		return nil, err
	}
	members, memberErr := rpc.GetConversationMembers(ctx, conversationId)
	if memberErr != nil {
		return nil, fmt.Errorf("查询会话[%d]成员失败: %w", conversationId, memberErr)
	}
	isMember := false
	for _, m := range members {
		if m == userId {
			isMember = true
			break
		}
	}
	if !isMember {
		return nil, fmt.Errorf("用户不在会话[%d]中，无权查看", conversationId)
	}
	formattedHistory, err := svc.formatHistoryMessages(ctx, userId, conversationId, 5)
	if err != nil {
		return nil, err
	}
	if formattedHistory == "" {
		return []string{"你好！", "大家好！", "最近怎么样？"}, nil
	}
	userNames, _ := rpc.GetUserNames(ctx, []int64{userId})
	userName := fmt.Sprintf("用户%d", userId)
	if userNames != nil && len(userNames) > 0 && userNames[0].Name != "" {
		userName = userNames[0].Name
	}
	systemPrompt := fmt.Sprintf(`你是一个聊天助手。根据以下对话上下文，为「%s」生成3条可能的回复。
要求：
1. 回复必须从「%s」的视角出发，是「%s」要说的话
2. 优先回复最新（序号最大）的消息，结合上下文理解对话意图
3. 每条回复自然、简洁，符合对话语境
4. 回复风格多样化：一条正式、一条轻松、一条幽默
5. 只返回3条回复，每条一行，用数字编号（1. 2. 3.）
6. 不要添加任何其他解释或说明`, userName, userName, userName)
	userContent := fmt.Sprintf("对话上下文（按时间顺序排列，序号越大越新）：\n%s\n\n请为「%s」针对最新消息生成3条回复候选：", formattedHistory, userName)
	llmCtx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()
	result, llmErr := svc.llmClient.Chat(llmCtx, apiKey, baseURL, model, systemPrompt, nil, userContent)
	if llmErr != nil {
		return nil, fmt.Errorf("生成回复候选失败: %w", llmErr)
	}
	var replies []string
	for _, line := range strings.Split(result, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = regexp.MustCompile(`^\d+[\.、)]\s*`).ReplaceAllString(line, "")
		if line != "" {
			replies = append(replies, line)
		}
	}
	if len(replies) == 0 {
		replies = []string{"好的", "了解了", "谢谢"}
	}
	if len(replies) > 3 {
		replies = replies[:3]
	}
	return replies, nil
}

func (svc *WorkService) TranslateMessage(ctx context.Context, content, targetLang string) (string, error) {
	apiKey, baseURL, model, err := svc.getSystemBotLLMConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("获取翻译服务配置失败: %w", err)
	}
	systemPrompt := fmt.Sprintf(`你是一个专业翻译助手。请将用户提供的文本翻译为%s。
要求：
1. 只返回翻译结果，不要添加任何解释或说明
2. 保持原文的语气和风格
3. 如果原文已经是目标语言，直接返回原文
4. 如果原文包含多种语言，将所有内容统一翻译为目标语言`, targetLang)
	llmCtx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()
	result, llmErr := svc.llmClient.Chat(llmCtx, apiKey, baseURL, model, systemPrompt, nil, content)
	if llmErr != nil {
		return "", fmt.Errorf("翻译失败: %w", llmErr)
	}
	return result, nil
}
