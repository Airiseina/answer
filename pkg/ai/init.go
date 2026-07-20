package ai

import (
	"context"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/spf13/viper"

	"github.com/Airiseina/answer/pkg/observability/logger"
	"go.uber.org/zap"
)

// 向量化调用超时
const embeddingTimeout = 30 * time.Second

// 全局 eino Embedder 实例，由 AiInit 初始化
// 替代原 volcengine SDK 直调，统一接入 eino callback 体系
var embedder embedding.Embedder

// AiInit 初始化 AI 客户端（Embedder + ChatModel）
// 配置项：
//   - embedding.api_key:  火山方舟 API Key
//   - embedding.base_url: OpenAI 兼容端点（默认豆包 api/v3）
//   - embedding.model:    向量化模型 ID
//   - llm.api_key:        火山方舟 API Key（可与 embedding 复用）
//   - llm.base_url:       OpenAI 兼容端点
//   - llm.model:          对话模型 ID（用于实体抽取等 LLM 任务）
//
// LLM 配置缺失时仅记录告警，不影响 Embedder 初始化；
// 调用方通过 ChatModelReady() 判断 LLM 可用性
func AiInit(v *viper.Viper) {
	apiKey := v.GetString("embedding.api_key")
	baseURL := v.GetString("embedding.base_url")
	model := v.GetString("embedding.model")

	// 基于 eino-ext OpenAI 适配器创建 Embedder
	// 豆包 api/v3 标准端点 OpenAI 兼容，支持单次批量向量化多条文本
	emb, err := openai.NewEmbedder(context.Background(), &openai.EmbeddingConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		Timeout: embeddingTimeout,
	})
	if err != nil {
		logger.Error("初始化 eino Embedder 失败", zap.Error(err))
	} else {
		embedder = emb
		logger.Info("eino Embedder 初始化完成",
			zap.String("model", model),
			zap.String("base_url", baseURL))
	}

	// 初始化 ChatModel（实体抽取等 LLM 任务使用）
	// 配置缺失时仅记录告警，不阻断 Embedder 可用性
	initChatModel(v)
}
