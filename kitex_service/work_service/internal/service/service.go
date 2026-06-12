package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/agent"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/config"
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
	mcpTimeout           = 60 * time.Second
	llmTimeout           = 120 * time.Second
	agentTimeout         = 180 * time.Second // Agent需要多轮ChatModel+工具调用, 给更充裕的时间
	memorySaveTimeout    = 60 * time.Second
	imageDownloadTimeout = 30 * time.Second
)

// messageContent 消息内容的基础结构

// 安全提示词：运行时动态加载，不存入数据库
var (
	userBotSafetyPrompt string
	safetyPromptOnce    sync.Once
)

// loadUserBotSafetyPrompt 加载用户Bot安全提示词模板
func loadUserBotSafetyPrompt() string {
	safetyPromptOnce.Do(func() {
		promptFile := config.V.GetString("ai.user_bot.safety_prompt_file")
		if promptFile == "" {
			klog.Warn("未配置ai.user_bot.safety_prompt_file，用户Bot安全提示词为空")
			return
		}
		data, err := os.ReadFile(promptFile)
		if err != nil {
			klog.Warnf("读取用户Bot安全提示词文件[%s]失败: %v", promptFile, err)
			return
		}
		userBotSafetyPrompt = strings.TrimSpace(string(data))
		klog.Infof("用户Bot安全提示词加载成功，长度=%d", len(userBotSafetyPrompt))
	})
	return userBotSafetyPrompt
}

// buildSafeSystemPrompt 将用户prompt与安全提示词拼接，安全提示词始终追加在末尾（最高优先级）
// 兼容旧数据：如果prompt中已包含安全词，先剥离再重新拼接
func buildSafeSystemPrompt(userPrompt string) string {
	safety := loadUserBotSafetyPrompt()
	if safety == "" {
		return userPrompt
	}
	// 剥离旧数据中可能已拼接的安全词，避免重复
	cleaned := stripSafetyPrompt(userPrompt, safety)
	if cleaned == "" {
		return safety
	}
	return cleaned + "\n\n" + safety
}

// stripSafetyPrompt 从prompt中剥离已拼接的安全提示词
func stripSafetyPrompt(prompt string, safety string) string {
	if safety == "" || prompt == "" {
		return prompt
	}
	suffix := "\n\n" + safety
	if strings.HasSuffix(prompt, suffix) {
		return strings.TrimSuffix(prompt, suffix)
	}
	if strings.HasSuffix(prompt, safety) {
		return strings.TrimSuffix(prompt, safety)
	}
	return prompt
}

type messageContent struct {
	Type string `json:"type"`
}

// imageContent 图片消息内容
type imageContent struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	Text string `json:"text,omitempty"` // 用户附带的文字描述
}

// parseImageContent 解析消息内容，如果是图片消息则返回图片信息
func parseImageContent(content string) (img *imageContent, isImage bool) {
	var base messageContent
	if err := json.Unmarshal([]byte(content), &base); err != nil {
		return nil, false
	}
	if base.Type != "image" {
		return nil, false
	}
	var imgContent imageContent
	if err := json.Unmarshal([]byte(content), &imgContent); err != nil {
		return nil, false
	}
	if imgContent.URL == "" {
		return nil, false
	}
	return &imgContent, true
}

// downloadImageAsBase64 从SeaweedFS下载图片并转为base64
// 返回 base64编码数据、MIME类型、错误信息（如图片过大）
func downloadImageAsBase64(ctx context.Context, imageURL string) (*llm.ImageData, error) {
	v := config.V
	filerURL := v.GetString("seaweedfs.filer_url")
	publicURL := v.GetString("seaweedfs.public_url")
	maxSize := v.GetInt64("image.max_size")
	if maxSize <= 0 {
		maxSize = 5 * 1024 * 1024 // 默认5MB
	}

	// 将公开URL转换为SeaweedFS内部URL
	// 公开URL格式: /files/chat/data/ab/cd123.png
	// 内部URL格式: http://127.0.0.1:8888/chat/data/ab/cd123.png
	internalURL := imageURL
	if publicURL != "" && strings.HasPrefix(imageURL, publicURL) {
		internalURL = filerURL + strings.TrimPrefix(imageURL, publicURL)
	}

	downloadCtx, cancel := context.WithTimeout(ctx, imageDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, internalURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建图片下载请求失败: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载图片失败, status=%d", resp.StatusCode)
	}

	// 检查Content-Length（如果有的话）
	if resp.ContentLength > maxSize {
		return nil, fmt.Errorf("图片大小(%d字节)超过限制(%d字节)，无法识别", resp.ContentLength, maxSize)
	}

	// 限制读取大小，防止读取超大文件
	limitedReader := io.LimitReader(resp.Body, maxSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("读取图片数据失败: %w", err)
	}

	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("图片大小超过限制(%d字节)，无法识别", maxSize)
	}

	// 根据文件扩展名推断MIME类型
	ext := strings.ToLower(filepath.Ext(internalURL))
	mimeType := "image/png" // 默认
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	case ".bmp":
		mimeType = "image/bmp"
	}

	// 如果响应头有Content-Type，优先使用
	if ct := resp.Header.Get("Content-Type"); ct != "" && strings.HasPrefix(ct, "image/") {
		mimeType = ct
	}

	return &llm.ImageData{
		Base64Data: base64.StdEncoding.EncodeToString(data),
		MIMEType:   mimeType,
	}, nil
}

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
	result := svc.handleWithAgent(ctx, botCfg, agentMcpServers, conversationId, senderId, botId, content, isGroupChat, quoteMsgID)
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
	var kbIDs []string // 知识库ID列表，用于RAGAS评估
	searchRunID := ""
	if isGroupChat {
		searchRunID = runID
	}
	memoryCtx, memoryCancel := context.WithTimeout(ctx, mcpTimeout)
	memories := mcp.SearchMemories(memoryCtx, svc.mcpPool, content, userID, botID, searchRunID, 3)
	memoryCancel()
	enhancedPrompt := buildSafeSystemPrompt(botCfg.SystemPrompt) + mcp.BuildMemoryPrompt(memories)

	// Self-RAG架构：不再预检索知识库拼入system prompt
	// 而是将知识库检索作为MCP工具暴露给Eino Agent，让Agent自行决定：
	// 1. 是否需要检索（Retrieve决策）
	// 2. 检索结果是否相关（IS_REL判断）
	// 3. 是否需要重新检索（重新检索决策）
	// 4. 答案是否被知识库支撑（IS_SUP自检）
	knowledgeCtx, knowledgeCancel := context.WithTimeout(ctx, mcpTimeout)
	kbResult := mcp.GetBotKnowledgeBases(knowledgeCtx, svc.mcpPool, botID)
	knowledgeCancel()
	klog.Infof("MCP知识库: GetBotKnowledgeBases(botID=%s) 返回长度=%d, 内容前200字符=%q", botID, len(kbResult), truncateStr(kbResult, 200))
	if kbResult != "" {
		var kbList struct {
			KnowledgeBases []struct {
				Id   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"knowledge_bases"`
		}
		if err := json.Unmarshal([]byte(kbResult), &kbList); err != nil {
			klog.Warnf("MCP知识库: 解析kbResult失败: %v, 原始内容=%q", err, truncateStr(kbResult, 200))
		} else if len(kbList.KnowledgeBases) > 0 {
			var kbInfo []string
			for _, kb := range kbList.KnowledgeBases {
				kbInfo = append(kbInfo, fmt.Sprintf("知识库「%s」(ID: %d)", kb.Name, kb.Id))
			}
			for _, kb := range kbList.KnowledgeBases {
				kbIDs = append(kbIDs, fmt.Sprintf("%d", kb.Id))
			}
			enhancedPrompt += fmt.Sprintf(
				"\n\n[知识库信息]\n你可以访问以下知识库：%s\n知识库ID列表：%s\n\n"+
					"Self-RAG检索策略：\n"+
					"1. 当用户的问题可能涉及知识库中的内容时，请使用search_knowledge工具检索，不要凭记忆猜测\n"+
					"2. 如果检索结果与问题不相关，可以尝试换一个关键词重新检索\n"+
					"3. 如果知识库中没有相关信息，请根据你的知识回答，并说明信息来源\n"+
					"4. 如果检索结果足够回答问题，请基于检索结果回答，不要添加未在检索结果中出现的信息",
				strings.Join(kbInfo, "、"),
				strings.Join(kbIDs, ","),
			)
		}
	}
	if botCfg.Name != "" {
		enhancedPrompt += fmt.Sprintf("\n\n你的名字是「%s」，当用户用@提及你时（如@%s），他们就是在和你说话。请自然地回应。", botCfg.Name, botCfg.Name)
	}
	// 根据可用的MCP服务器添加工具使用引导，促使LLM主动调用工具
	if len(mcpServers) > 0 {
		var toolHints []string
		for _, s := range mcpServers {
			switch s.Name {
			case "searxng":
				toolHints = append(toolHints, "- 当你需要搜索互联网上的最新信息、新闻、实时数据或你不确定的事实时，必须使用web_search或news_search工具搜索，不要凭记忆猜测")
			case "weather":
				toolHints = append(toolHints, "- 当用户询问天气、空气质量等气象信息时，请使用weather相关工具查询")
			case "timeserver":
				toolHints = append(toolHints, "- 当用户询问当前时间、日期等时间信息时，请使用timeserver相关工具查询")
			default:
				toolHints = append(toolHints, fmt.Sprintf("- 当用户的问题可能需要%s工具提供的信息时，请主动调用该工具", s.Name))
			}
		}
		enhancedPrompt += "\n\n你可以使用以下工具来获取实时信息，请在需要时主动调用，不要凭记忆回答不确定的内容：\n" + strings.Join(toolHints, "\n")
	}
	userContent := content
	var imageData *llm.ImageData // 图片base64数据，用于多模态消息

	// 解析图片消息：如果是图片则下载转base64，否则当纯文本处理
	if imgInfo, isImage := parseImageContent(content); isImage {
		imgData, imgErr := downloadImageAsBase64(ctx, imgInfo.URL)
		if imgErr != nil {
			klog.CtxWarnf(ctx, "Bot[%d]下载图片失败: %v，将作为文本处理", botId, imgErr)
			userContent = fmt.Sprintf("[图片加载失败: %s]", imgErr.Error())
		} else {
			imageData = imgData
			// 使用用户附带的文字描述，没有则用默认提示
			if imgInfo.Text != "" {
				userContent = imgInfo.Text
			} else {
				userContent = "用户发送了一张图片，请根据图片内容进行回复。"
			}
		}
	}

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
				// 解析引用消息：如果是图片则下载转base64，否则当纯文本
				quoteText := quoteMsg.Content
				if quoteImgInfo, isQuoteImage := parseImageContent(quoteMsg.Content); isQuoteImage {
					quoteImgData, quoteImgErr := downloadImageAsBase64(ctx, quoteImgInfo.URL)
					if quoteImgErr != nil {
						quoteText = fmt.Sprintf("[图片加载失败: %s]", quoteImgErr.Error())
					} else {
						// 引用的图片优先作为主图片数据传给LLM（如果当前消息没有图片）
						if imageData == nil {
							imageData = quoteImgData
						}
						quoteText = "[图片]"
					}
				}
				userContent = fmt.Sprintf("[引用 %s 的消息]: %s\n\n%s", quoteSenderName, quoteText, userContent)
			}
		}
	}
	agentCtx, agentCancel := context.WithTimeout(ctx, agentTimeout)
	defer agentCancel()
	agentResult, err := svc.agent.Run(agentCtx, agent.AgentRunConfig{
		APIKey:       botCfg.ApiKey,
		BaseURL:      botCfg.BaseUrl,
		Model:        botCfg.Model,
		SystemPrompt: enhancedPrompt,
		McpServers:   mcpServers,
		UserContent:  userContent,
		ImageData:    imageData,
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
		llmResult, llmErr := svc.llmClient.Chat(llmCtx, botCfg.ApiKey, botCfg.BaseUrl, botCfg.Model, enhancedPrompt, chatHistory, userContent, imageData)
		if llmErr != nil {
			klog.CtxErrorf(ctx, "处理消息出错：%v", llmErr)
			return "抱歉，在月球这边接收地球的信息偶尔会有延迟呢~😭。能再重复一遍吗(*/ω＼*)?"
		}
		go func() {
			saveCtx, cancel := context.WithTimeout(context.Background(), memorySaveTimeout)
			defer cancel()
			memoryContent := fmt.Sprintf("用户: %s | 助手: %s", userContent, llmResult)
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
		memoryContent := fmt.Sprintf("用户: %s | 助手: %s", userContent, agentResult.Content)
		saveRunID := ""
		if isGroupChat {
			saveRunID = runID
		}
		mcp.SaveMemory(saveCtx, svc.mcpPool, memoryContent, userID, botID, saveRunID)
	}()

	// 异步提交RAGAS评估：当Agent使用了知识库检索时，自动评估回答质量
	ragasURL := config.V.GetString("ragas_eval.url")
	klog.Infof("RAGAS评估检查: Contexts数量=%d, ragas_eval.url=%s", len(agentResult.Contexts), ragasURL)
	if len(agentResult.Contexts) > 0 && ragasURL != "" {
		go func() {
			evalCtx, evalCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer evalCancel()
			SubmitRAGASEvaluation(evalCtx, ragasURL, userContent, agentResult.Content, agentResult.Contexts, kbIDs)
		}()
	}

	return agentResult.Content
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
		fmt.Fprintf(&sb, "[%d| %s] %s: %s\n", i+1, t, senderName, m.Content)
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
	result, llmErr := svc.llmClient.Chat(llmCtx, apiKey, baseURL, model, systemPrompt, nil, userContent, nil)
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
	if len(userNames) > 0 && userNames[0].Name != "" {
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
	result, llmErr := svc.llmClient.Chat(llmCtx, apiKey, baseURL, model, systemPrompt, nil, userContent, nil)
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
	result, llmErr := svc.llmClient.Chat(llmCtx, apiKey, baseURL, model, systemPrompt, nil, content, nil)
	if llmErr != nil {
		return "", fmt.Errorf("翻译失败: %w", llmErr)
	}
	return result, nil
}

// SubmitRAGASEvaluation 异步提交RAGAS评估请求
// 当Agent使用了知识库检索时，自动将(question, answer, contexts)提交给RAGAS评估服务
// 提交后自动轮询评估结果并打印到日志
func SubmitRAGASEvaluation(ctx context.Context, ragasURL, question, answer string, contexts []string, kbIDs []string) {
	// 构建评估请求体
	reqBody := map[string]interface{}{
		"dataset": map[string]interface{}{
			"samples": []map[string]interface{}{
				{
					"question":     question,
					"answer":       answer,
					"contexts":     contexts,
					"ground_truth": "", // 自动评估时无ground_truth，仅评估faithfulness
				},
			},
			"kb_ids": strings.Join(kbIDs, ","),
		},
		"metrics": []string{"faithfulness"}, // 自动评估仅用faithfulness（不需要ground_truth）
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		klog.Warnf("RAGAS评估: 序列化请求体失败: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", ragasURL+"/evaluate", strings.NewReader(string(bodyBytes)))
	if err != nil {
		klog.Warnf("RAGAS评估: 创建请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		klog.Warnf("RAGAS评估: 提交评估请求失败: %v", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		klog.Warnf("RAGAS评估: 提交失败, status=%d, response=%s", resp.StatusCode, string(respBody))
		return
	}

	// 解析eval_id
	var evalResp struct {
		EvalID string `json:"eval_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &evalResp); err != nil || evalResp.EvalID == "" {
		klog.Warnf("RAGAS评估: 解析响应失败: %v, body=%s", err, string(respBody))
		return
	}
	klog.Infof("RAGAS评估: 提交成功, eval_id=%s, question=%q", evalResp.EvalID, truncateStr(question, 50))

	// 轮询评估结果（最多等待120秒）
	for i := 0; i < 24; i++ {
		time.Sleep(5 * time.Second)
		resultReq, _ := http.NewRequestWithContext(context.Background(), "GET", ragasURL+"/evaluate/"+evalResp.EvalID, nil)
		resultResp, err := http.DefaultClient.Do(resultReq)
		if err != nil {
			klog.Warnf("RAGAS评估: 查询结果失败: %v", err)
			continue
		}
		resultBody, _ := io.ReadAll(resultResp.Body)
		resultResp.Body.Close()

		var result struct {
			EvalID  string             `json:"eval_id"`
			Status  string             `json:"status"`
			Metrics map[string]float64 `json:"metrics"`
			Error   string             `json:"error"`
		}
		if err := json.Unmarshal(resultBody, &result); err != nil {
			klog.Warnf("RAGAS评估: 解析结果失败: %v", err)
			continue
		}

		if result.Status == "completed" {
			klog.Infof("RAGAS评估: 评估完成, eval_id=%s, metrics=%v", result.EvalID, result.Metrics)
			return
		} else if result.Status == "failed" {
			klog.Warnf("RAGAS评估: 评估失败, eval_id=%s, error=%s", result.EvalID, result.Error)
			return
		}
		// status == "running", 继续轮询
	}
	klog.Warnf("RAGAS评估: 轮询超时, eval_id=%s", evalResp.EvalID)
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
