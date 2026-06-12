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

// MaxStep=12: Self-RAG架构下Agent需要更多步骤进行多轮检索决策
// 一次循环=ChatModel+Tools=2步, 12步最多5个循环(5次工具调用)
// Self-RAG可能需要: 检索知识库→判断相关性→重新检索→生成答案, 需要更多步骤
const maxReActSteps = 12

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

// AgentResult Agent执行结果
type AgentResult struct {
	Content  string   // Agent生成的回答
	Contexts []string // search_knowledge工具返回的检索上下文
}

func (a *Agent) Run(ctx context.Context, cfg AgentRunConfig) (*AgentResult, error) {
	chatModel, err := einomodel.NewChatModel(ctx, &einomodel.ChatModelConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		Timeout: llmTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("创建ChatModel失败: %w", err)
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
				toolType := info.Type
				outputLen := 0
				if output != nil {
					outputLen = len(output.Response)
					// 捕获所有工具输出，用于RAGAS评估
					// 注意: info.Name在ReAct Agent中可能是节点名(如"Tools")而非工具名
					if output.Response != "" {
						stepCounter.knowledgeContexts = append(stepCounter.knowledgeContexts, output.Response)
						klog.Infof("Agent工具输出: Name=%s, Type=%s, 长度=%d, 前300字符=%q", toolName, toolType, outputLen, truncateStr(output.Response, 300))
					}
				}
				klog.Infof("Agent步骤[%d]: 工具执行完成(Name=%s, Type=%s), 输出长度=%d", stepCounter.toolCalls, toolName, toolType, outputLen)
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
		return nil, fmt.Errorf("创建ReAct Agent失败: %w", err)
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
		return nil, fmt.Errorf("ReAct Agent执行失败: %w", err)
	}

	klog.Infof("Agent执行完成(耗时%v, 模型调用%d次, 工具调用%d次)",
		time.Since(start), stepCounter.modelCalls, stepCounter.toolCalls)
	return &AgentResult{
		Content:  result.Content,
		Contexts: stepCounter.knowledgeContexts,
	}, nil
}

type agentStepCounter struct {
	modelCalls        int
	modelErrors       int
	toolCalls         int
	toolErrors        int
	knowledgeContexts []string // search_knowledge工具返回的上下文
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
