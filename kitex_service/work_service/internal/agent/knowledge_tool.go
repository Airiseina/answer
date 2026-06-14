package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/kitex/pkg/klog"

	"github.com/Airiseina/answer/kitex_service/work_service/internal/mcp"
)

// searchKnowledgeTool 知识库检索工具（通过RPC调用knowledge_service）
// 替代原MCP knowledge工具，消除Python中间层
type searchKnowledgeTool struct {
	info *schema.ToolInfo
}

func newSearchKnowledgeTool() tool.InvokableTool {
	return &searchKnowledgeTool{
		info: &schema.ToolInfo{
			Name: "search_knowledge",
			Desc: "在知识库中检索与查询相关的内容。当用户的问题可能涉及知识库中的内容时，使用此工具检索，不要凭记忆猜测。如果检索结果与问题不相关，可以尝试换一个关键词重新检索。",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"query": {
					Type:     schema.String,
					Desc:     "检索查询文本",
					Required: true,
				},
				"kb_ids": {
					Type:     schema.Array,
					Desc:     "要检索的知识库ID列表",
					ElemInfo: &schema.ParameterInfo{Type: schema.Integer},
					Required: true,
				},
				"top_k": {
					Type: schema.Integer,
					Desc: "返回的最大结果数量，默认5",
				},
			}),
		},
	}
}

func (t *searchKnowledgeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *searchKnowledgeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Query string  `json:"query"`
		KbIDs []int64 `json:"kb_ids"`
		TopK  int     `json:"top_k"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	if args.Query == "" {
		return "", fmt.Errorf("query不能为空")
	}
	if len(args.KbIDs) == 0 {
		return "", fmt.Errorf("kb_ids不能为空")
	}
	if args.TopK <= 0 {
		args.TopK = 5
	}

	klog.Infof("知识库检索工具: query=%q, kb_ids=%v, top_k=%d", args.Query, args.KbIDs, args.TopK)
	result := mcp.SearchKnowledgeViaRPC(ctx, args.Query, args.KbIDs, args.TopK)
	if result == "" {
		return "未找到相关内容", nil
	}
	return result, nil
}

// getBotKnowledgeBasesTool 获取Bot关联知识库列表工具（通过RPC调用knowledge_service）
type getBotKnowledgeBasesTool struct {
	info *schema.ToolInfo
}

func newGetBotKnowledgeBasesTool() tool.InvokableTool {
	return &getBotKnowledgeBasesTool{
		info: &schema.ToolInfo{
			Name: "get_bot_knowledge_bases",
			Desc: "获取当前Bot关联的所有知识库列表，包括知识库名称、ID、文档数量等信息。在检索知识库之前，可先调用此工具了解可用的知识库。",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"bot_id": {
					Type:     schema.Integer,
					Desc:     "Bot的ID",
					Required: true,
				},
			}),
		},
	}
}

func (t *getBotKnowledgeBasesTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *getBotKnowledgeBasesTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		BotID int64 `json:"bot_id"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	if args.BotID == 0 {
		return "", fmt.Errorf("bot_id不能为空")
	}

	klog.Infof("获取Bot知识库列表工具: bot_id=%d", args.BotID)
	result := mcp.GetBotKnowledgeBasesViaRPC(ctx, args.BotID)
	if result == "" {
		return "当前Bot未关联知识库", nil
	}
	return result, nil
}

// parseKBIDs 将字符串数组解析为int64数组
func parseKBIDs(kbIDStrs []string) ([]int64, error) {
	kbIDs := make([]int64, 0, len(kbIDStrs))
	for _, s := range kbIDStrs {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("无效的知识库ID: %s", s)
		}
		kbIDs = append(kbIDs, id)
	}
	return kbIDs, nil
}
