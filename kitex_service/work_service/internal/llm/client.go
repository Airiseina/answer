package llm

import (
	"context"
	"fmt"
	"time"

	einomodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

// ChatMessage 历史对话消息，用于构造 LLM 上下文
type ChatMessage struct {
	Role    string
	Content string
}

// ImageData 图片base64数据，用于多模态消息
type ImageData struct {
	Base64Data string
	MIMEType   string
}

// 默认 LLM 调用超时，与 service.go 中的 llmTimeout 保持一致
const defaultChatTimeout = 120 * time.Second

// 默认采样温度，与原 openai-go 实现保持一致
const defaultTemperature = 0.7

// Client LLM 客户端，统一基于 eino ChatModel 实现
// 替代原 openai-go 直接调用，与 agent/react.go 共用同一套 LLM 接入层
type Client struct{}

func NewClient() *Client {
	return &Client{}
}

// Chat 调用 LLM 生成回复
// 参数:
//   - apiKey/baseURL/modelName: OpenAI 兼容 API 配置
//   - systemPrompt: 系统提示词，为空则不附加
//   - history: 历史对话消息，按时间顺序排列
//   - userContent: 当前用户输入文本
//   - imageData: 图片数据，非 nil 时构造多模态消息
func (c *Client) Chat(ctx context.Context, apiKey, baseURL, modelName, systemPrompt string, history []ChatMessage, userContent string, imageData *ImageData) (string, error) {
	// 基于 eino-ext OpenAI 适配器创建 ChatModel
	// 与 agent/react.go 使用同一实现，统一可观测性与回调链路
	temp := float32(defaultTemperature)
	chatModel, err := einomodel.NewChatModel(ctx, &einomodel.ChatModelConfig{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       modelName,
		Temperature: &temp,
		Timeout:     defaultChatTimeout,
	})
	if err != nil {
		return "", fmt.Errorf("创建ChatModel失败: %w", err)
	}

	// 构造消息列表：System + History + User
	messages := make([]*schema.Message, 0, 2+len(history))
	if systemPrompt != "" {
		messages = append(messages, &schema.Message{
			Role:    schema.System,
			Content: systemPrompt,
		})
	}
	for _, h := range history {
		var role schema.RoleType
		switch h.Role {
		case "user":
			role = schema.User
		case "assistant":
			role = schema.Assistant
		default:
			// 兜底：未知角色按 System 处理，与原 openai-go 实现一致
			role = schema.System
		}
		messages = append(messages, &schema.Message{
			Role:    role,
			Content: h.Content,
		})
	}

	// 构造用户消息：有图片时使用多模态格式（与 agent/react.go 一致）
	if imageData != nil {
		messages = append(messages, &schema.Message{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: userContent,
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: &imageData.Base64Data,
							MIMEType:   imageData.MIMEType,
						},
					},
				},
			},
		})
	} else {
		messages = append(messages, &schema.Message{
			Role:    schema.User,
			Content: userContent,
		})
	}

	// 调用 ChatModel 生成回复
	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("调用ChatModel失败: %w", err)
	}
	if resp == nil || resp.Content == "" {
		return "", fmt.Errorf("ChatModel返回空响应")
	}
	return resp.Content, nil
}
