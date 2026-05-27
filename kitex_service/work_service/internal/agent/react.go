package agent

import (
	"context"
	"fmt"

	einomodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/kitex/pkg/klog"

	"github.com/Airiseina/answer/kitex_service/work_service/internal/mcp"
)

const maxReActSteps = 10

type Agent struct {
	mcpPool *mcp.Pool
}

func NewAgent(mcpPool *mcp.Pool) *Agent {
	return &Agent{mcpPool: mcpPool}
}

type AgentRunConfig struct {
	APIKey       string
	BaseURL      string
	Model        string
	SystemPrompt string
	McpServers   []mcp.ServerConfig
	History      []*schema.Message
	UserContent  string
}

func (a *Agent) Run(ctx context.Context, cfg AgentRunConfig) (string, error) {
	chatModel, err := einomodel.NewChatModel(ctx, &einomodel.ChatModelConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
	})
	if err != nil {
		return "", fmt.Errorf("创建ChatModel失败: %w", err)
	}

	toolsConfig := compose.ToolsNodeConfig{}
	if len(cfg.McpServers) > 0 {
		tools, toolsErr := a.mcpPool.GetAllTools(ctx, cfg.McpServers)
		if toolsErr != nil {
			klog.Errorf("获取MCP工具失败: %v", toolsErr)
		} else {
			toolsConfig.Tools = tools
		}
	}

	reactAgent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig:      toolsConfig,
		MaxStep:          maxReActSteps,
	})
	if err != nil {
		return "", fmt.Errorf("创建ReAct Agent失败: %w", err)
	}

	var messages []*schema.Message
	if cfg.SystemPrompt != "" {
		messages = append(messages, &schema.Message{
			Role:    schema.System,
			Content: cfg.SystemPrompt,
		})
	}
	messages = append(messages, cfg.History...)
	messages = append(messages, &schema.Message{
		Role:    schema.User,
		Content: cfg.UserContent,
	})

	result, err := reactAgent.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("ReAct Agent执行失败: %w", err)
	}

	return result.Content, nil
}
