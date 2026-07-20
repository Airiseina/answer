package ai

import (
	"context"
	"errors"
	"time"

	einomodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"

	"github.com/Airiseina/answer/pkg/observability/logger"
	"go.uber.org/zap"
)

// LLM 调用超时，与 work_service/internal/llm/client.go 的 defaultChatTimeout 保持一致
const defaultChatTimeout = 120 * time.Second

// 结构化抽取场景使用较低温度，提升 JSON 输出稳定性
const defaultExtractionTemperature = 0.3

// errChatModelNotReady 在 ChatModel 未初始化时返回
// 与 errEmbedderNotReady 命名风格对齐
var errChatModelNotReady = errors.New("LLM 客户端未初始化，请先调用 AiInit")

// 全局 eino ChatModel 实例，由 AiInit 初始化
// 替代各服务自建 LLM Client，统一接入 eino callback 体系
var (
	chatModel     model.ChatModel
	chatModelName string
)

// initChatModel 从 viper 配置初始化 eino ChatModel
// 配置项：
//   - llm.api_key:  火山方舟 API Key
//   - llm.base_url: OpenAI 兼容端点（默认豆包 api/v3）
//   - llm.model:    对话模型 ID
//
// 配置缺失时不报错，仅记录告警；调用方通过 ChatModelReady() 判断可用性
func initChatModel(v *viper.Viper) {
	apiKey := v.GetString("llm.api_key")
	baseURL := v.GetString("llm.base_url")
	modelName := v.GetString("llm.model")
	if apiKey == "" || modelName == "" {
		logger.Warn("LLM 配置缺失，跳过初始化（实体抽取功能将降级）",
			zap.Bool("api_key_set", apiKey != ""),
			zap.Bool("model_set", modelName != ""))
		return
	}
	// eino-ext OpenAI 适配器构造 ChatModel
	// 参考: https://github.com/cloudwego/eino-ext/tree/main/components/model/openai
	temp := float32(defaultExtractionTemperature)
	cm, err := einomodel.NewChatModel(context.Background(), &einomodel.ChatModelConfig{
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       modelName,
		Temperature: &temp,
		Timeout:     defaultChatTimeout,
	})
	if err != nil {
		logger.Error("初始化 eino ChatModel 失败", zap.Error(err))
		return
	}
	chatModel = cm
	chatModelName = modelName
	logger.Info("eino ChatModel 初始化完成",
		zap.String("model", modelName),
		zap.String("base_url", baseURL))
}

// Chat 调用 LLM 生成回复
// 参数:
//   - systemPrompt: 系统提示词，为空则不附加
//   - userContent:  用户输入文本
//
// 与 work_service/internal/llm/client.go 的 Chat 行为对齐，但仅支持文本
// （实体抽取场景无需多模态）
func Chat(ctx context.Context, systemPrompt, userContent string) (string, error) {
	if chatModel == nil {
		return "", errChatModelNotReady
	}
	// 构造消息列表：System + User
	messages := make([]*schema.Message, 0, 2)
	if systemPrompt != "" {
		messages = append(messages, &schema.Message{
			Role:    schema.System,
			Content: systemPrompt,
		})
	}
	messages = append(messages, &schema.Message{
		Role:    schema.User,
		Content: userContent,
	})

	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		logger.Error("调用 eino ChatModel 失败", zap.Error(err))
		return "", err
	}
	if resp == nil || resp.Content == "" {
		return "", errors.New("ChatModel 返回空响应")
	}
	return resp.Content, nil
}

// ChatModelReady 返回 LLM 客户端是否就绪
// 调用方（如 knowledge_service 实体抽取）据此判断是否可走 LLM 路径
func ChatModelReady() bool {
	return chatModel != nil
}

// ChatModelName 返回当前 LLM 模型名，未初始化时返回空串
// 用于日志与可观测性
func ChatModelName() string {
	return chatModelName
}
