package agent

import (
	"context"
	"fmt"
	"time"

	einomodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	einoagent "github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	template "github.com/cloudwego/eino/utils/callbacks"
	"github.com/cloudwego/kitex/pkg/klog"

	"github.com/Airiseina/answer/kitex_service/work_service/internal/llm"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/mcp"
)

// MaxStep=8: 一次循环=ChatModel+Tools=2步, 8步最多3个循环(3次工具调用), 最后一步ChatModel返回结果
// 对于聊天场景, 3次工具调用足够(搜索+查询+总结), 避免Agent陷入无限循环
const maxReActSteps = 8

const (
	llmTimeout = 120 * time.Second
	mcpTimeout = 20 * time.Second
)

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
	UserContent  string
	ImageData    *llm.ImageData // 图片数据，非nil时构造多模态消息
}

func (a *Agent) Run(ctx context.Context, cfg AgentRunConfig) (string, error) {
	chatModel, err := einomodel.NewChatModel(ctx, &einomodel.ChatModelConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		Timeout: llmTimeout,
	})
	if err != nil {
		return "", fmt.Errorf("创建ChatModel失败: %w", err)
	}

	toolsConfig := compose.ToolsNodeConfig{}

	if len(cfg.McpServers) > 0 {
		mcpCtx, mcpCancel := context.WithTimeout(ctx, mcpTimeout)
		mcpTools, toolsErr := a.mcpPool.GetAllTools(mcpCtx, cfg.McpServers)
		mcpCancel()
		if toolsErr != nil {
			klog.Errorf("获取MCP工具失败: %v", toolsErr)
		} else {
			toolsConfig.Tools = append(toolsConfig.Tools, mcpTools...)
		}
	}

	// 注意: 不使用 ToolReturnDirectly, 因为工具可能返回错误信息(如 {"error": "HTTP 500"}),
	// 直接返回给用户体验很差, 应让LLM解释和格式化结果

	// 构建回调, 监控Agent执行步数和工具调用
	stepCounter := &agentStepCounter{}
	cb := react.BuildAgentCallback(
		&template.ModelCallbackHandler{
			OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
				stepCounter.modelCalls++
				toolCallCount := 0
				if output != nil && output.Message != nil {
					toolCallCount = len(output.Message.ToolCalls)
				}
				klog.Infof("Agent步骤[%d]: ChatModel完成, tool_calls=%d", stepCounter.modelCalls, toolCallCount)
				return ctx
			},
			OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
				stepCounter.modelErrors++
				klog.Errorf("Agent步骤[%d]: ChatModel错误: %v", stepCounter.modelCalls, err)
				return ctx
			},
		},
		&template.ToolCallbackHandler{
			OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *tool.CallbackOutput) context.Context {
				stepCounter.toolCalls++
				toolName := info.Name
				outputLen := 0
				if output != nil {
					outputLen = len(output.Response)
				}
				klog.Infof("Agent步骤[%d]: 工具[%s]执行完成, 输出长度=%d", stepCounter.toolCalls, toolName, outputLen)
				return ctx
			},
			OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
				stepCounter.toolErrors++
				klog.Errorf("Agent步骤[%d]: 工具[%s]错误: %v", stepCounter.toolCalls, info.Name, err)
				return ctx
			},
		},
	)

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
	// 构造用户消息：有图片时使用多模态格式
	if cfg.ImageData != nil {
		userMsg := &schema.Message{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: cfg.UserContent,
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: &cfg.ImageData.Base64Data,
							MIMEType:   cfg.ImageData.MIMEType,
						},
					},
				},
			},
		}
		messages = append(messages, userMsg)
	} else {
		messages = append(messages, &schema.Message{
			Role:    schema.User,
			Content: cfg.UserContent,
		})
	}

	start := time.Now()
	result, err := reactAgent.Generate(ctx, messages, einoagent.WithComposeOptions(compose.WithCallbacks(cb)))
	if err != nil {
		klog.Errorf("Agent执行失败(耗时%v, 模型调用%d次(错误%d次), 工具调用%d次(错误%d次)): %v",
			time.Since(start), stepCounter.modelCalls, stepCounter.modelErrors,
			stepCounter.toolCalls, stepCounter.toolErrors, err)
		return "", fmt.Errorf("ReAct Agent执行失败: %w", err)
	}

	klog.Infof("Agent执行完成(耗时%v, 模型调用%d次, 工具调用%d次)",
		time.Since(start), stepCounter.modelCalls, stepCounter.toolCalls)
	return result.Content, nil
}

type agentStepCounter struct {
	modelCalls  int
	modelErrors int
	toolCalls   int
	toolErrors  int
}
