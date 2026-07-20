package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Airiseina/answer/pkg/observability/logger"
	"go.uber.org/zap"
)

// Entity LLM 抽取出的实体
// 与 knowledge_service/internal/graph.EntityData 字段对齐，便于调用方转换
type Entity struct {
	Name        string `json:"name"`
	Type        string `json:"type"`        // Person/Organization/Concept/Technology/Location/Product/Event 等
	Description string `json:"description"`
}

// Relation LLM 抽取出的实体间关系
// 与 knowledge_service/internal/graph.RelationData 字段对齐
type Relation struct {
	SourceEntity string `json:"source"`
	TargetEntity string `json:"target"`
	RelationType string `json:"relation_type"`
}

// ExtractResult LLM 实体抽取结果
type ExtractResult struct {
	Entities  []Entity   `json:"entities"`
	Relations []Relation `json:"relations"`
}

// entityExtractionSystemPrompt 实体抽取系统提示词
// 输出严格 JSON，便于程序解析
const entityExtractionSystemPrompt = `你是一个专业的信息抽取助手。从给定的文本中提取关键实体和实体间关系。

要求：
1. 实体类型包括：Person（人名）、Organization（组织/公司）、Concept（概念）、Technology（技术）、Location（地点）、Product（产品）、Event（事件）等
2. 仅提取文本中明确出现或有强暗示的实体，不要编造
3. 实体名称使用规范化的简称（如"字节跳动"而非"字节跳动公司"）
4. 关系类型使用大写下划线格式（如 FOUNDED_BY、LOCATED_IN、DEVELOPED_BY、PART_OF）
5. 只提取有明确文本依据的关系

输出格式：严格的 JSON，不要包含任何解释或 markdown 代码块标记。
格式如下：
{
  "entities": [
    {"name": "实体名", "type": "Person", "description": "简要描述"}
  ],
  "relations": [
    {"source": "源实体名", "target": "目标实体名", "relation_type": "FOUNDED_BY"}
  ]
}

如果文本中没有可识别的实体，返回 {"entities": [], "relations": []}`

// keywordExtractionSystemPrompt 关键词抽取系统提示词
// 用于图谱检索时从用户查询中提取关键词
const keywordExtractionSystemPrompt = `你是一个查询关键词提取助手。从用户的查询中提取用于知识图谱检索的关键实体词和概念词。

要求：
1. 提取 1-5 个最具检索价值的关键词
2. 优先提取命名实体（人名、组织、技术、产品等）
3. 过滤停用词和无意义词
4. 输出格式：严格的 JSON 字符串数组，如 ["关键词1", "关键词2"]
5. 不要包含任何解释或 markdown 代码块标记

如果查询无有效关键词，返回 []`

// ExtractEntities 从文本中用 LLM 抽取实体和关系
// 调用方应将长文本分块后逐块调用此函数
// LLM 未初始化时返回 errChatModelNotReady，调用方应据此降级
func ExtractEntities(ctx context.Context, text string) (*ExtractResult, error) {
	if !ChatModelReady() {
		return nil, errChatModelNotReady
	}
	if strings.TrimSpace(text) == "" {
		return &ExtractResult{}, nil
	}

	userContent := fmt.Sprintf("请从以下文本中抽取实体和关系：\n\n%s", text)
	resp, err := Chat(ctx, entityExtractionSystemPrompt, userContent)
	if err != nil {
		logger.Error("LLM 实体抽取调用失败", zap.Error(err))
		return nil, err
	}

	// 清理可能的 markdown 代码块标记（部分模型不严格遵守 prompt 约束）
	resp = cleanJSONResponse(resp)

	var result ExtractResult
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		// 解析失败不阻断流程，返回空结果；调用方继续处理其他 Chunk
		logger.Warn("LLM 实体抽取响应解析失败，返回空结果",
			zap.String("response_prefix", truncateForLog(resp, 200)),
			zap.Error(err))
		return &ExtractResult{}, nil
	}
	return &result, nil
}

// ExtractKeywords 用 LLM 从查询中抽取关键词
// 用于图谱检索时定位实体，替代原 extractKeywords 的简单分词方案
// LLM 未初始化时返回 errChatModelNotReady，调用方应据此降级
func ExtractKeywords(ctx context.Context, query string) ([]string, error) {
	if !ChatModelReady() {
		return nil, errChatModelNotReady
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	userContent := fmt.Sprintf("查询：%s", query)
	resp, err := Chat(ctx, keywordExtractionSystemPrompt, userContent)
	if err != nil {
		logger.Error("LLM 关键词抽取调用失败", zap.Error(err))
		return nil, err
	}

	resp = cleanJSONResponse(resp)
	var keywords []string
	if err := json.Unmarshal([]byte(resp), &keywords); err != nil {
		logger.Warn("LLM 关键词响应解析失败，返回空结果",
			zap.String("response_prefix", truncateForLog(resp, 200)),
			zap.Error(err))
		return nil, nil
	}
	return keywords, nil
}

// cleanJSONResponse 清理 LLM 响应中可能包含的 markdown 代码块标记
// 部分 OpenAI 兼容端点（含火山方舟）不严格遵守 response_format 约束
// 通过 prompt 工程要求纯 JSON，再做容错清理，提升跨模型兼容性
func cleanJSONResponse(s string) string {
	s = strings.TrimSpace(s)
	// 去除 ```json 或 ``` 开头
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	// 去除结尾 ```
	s = strings.TrimSuffix(s, "```")
	// 二次 Trim 兼容代码块内有换行的情况
	return strings.TrimSpace(s)
}

// truncateForLog 截断字符串用于日志输出，避免日志过长
// 使用 rune 计数以正确处理中文等多字节字符
func truncateForLog(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
