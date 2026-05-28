package llm

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type ChatMessage struct {
	Role    string
	Content string
}

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Chat(ctx context.Context, apiKey, baseURL, model, systemPrompt string, history []ChatMessage, userContent string) (string, error) {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)
	params := openai.ChatCompletionNewParams{
		Model:       model,
		Temperature: openai.Float(0.7),
	}
	if systemPrompt != "" {
		params.Messages = append(params.Messages, openai.SystemMessage(systemPrompt))
	}
	for _, h := range history {
		switch h.Role {
		case "user":
			params.Messages = append(params.Messages, openai.UserMessage(h.Content))
		case "assistant":
			params.Messages = append(params.Messages, openai.AssistantMessage(h.Content))
		default:
			params.Messages = append(params.Messages, openai.SystemMessage(h.Content))
		}
	}
	params.Messages = append(params.Messages, openai.UserMessage(userContent))
	resp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("调用OpenAI失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI返回空响应")
	}
	return resp.Choices[0].Message.Content, nil
}
