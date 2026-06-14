package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Airiseina/answer/kitex_service/work_service/rpc"
	"github.com/cloudwego/kitex/pkg/klog"
)

const knowledgeRPCTimeout = 30 * time.Second

// SearchKnowledgeViaRPC 通过RPC调用knowledge_service进行知识库检索
// 返回JSON字符串，格式与原MCP工具保持一致，便于Agent解析
func SearchKnowledgeViaRPC(ctx context.Context, query string, kbIDs []int64, topK int) string {
	searchCtx, cancel := context.WithTimeout(ctx, knowledgeRPCTimeout)
	defer cancel()

	results, err := rpc.SearchKnowledge(searchCtx, kbIDs, query, int32(topK))
	if err != nil {
		klog.Errorf("知识库检索RPC失败: %v", err)
		return ""
	}
	if len(results) == 0 {
		return ""
	}

	// 转换为与原MCP工具一致的JSON格式
	type searchResultItem struct {
		Content    string  `json:"content"`
		Source     string  `json:"source"`
		DocID      int64   `json:"doc_id"`
		KBID       int64   `json:"kb_id"`
		ChunkIndex int     `json:"chunk_index"`
		Score      float64 `json:"score"`
		PageNumber *int    `json:"page_number,omitempty"`
	}
	items := make([]searchResultItem, 0, len(results))
	for _, r := range results {
		items = append(items, searchResultItem{
			Content:    r.Content,
			Source:     r.Source,
			DocID:      r.DocID,
			KBID:       r.KBID,
			ChunkIndex: r.ChunkIndex,
			Score:      r.Score,
			PageNumber: r.PageNumber,
		})
	}
	data, _ := json.Marshal(items)
	return string(data)
}

// GetBotKnowledgeBasesViaRPC 通过RPC获取Bot关联的知识库列表
// 返回JSON字符串，格式与原MCP工具保持一致
func GetBotKnowledgeBasesViaRPC(ctx context.Context, botID int64) string {
	kbCtx, cancel := context.WithTimeout(ctx, knowledgeRPCTimeout)
	defer cancel()

	kbs, err := rpc.GetBotKnowledgeBases(kbCtx, botID)
	if err != nil {
		klog.Errorf("获取Bot知识库列表RPC失败: %v", err)
		return ""
	}
	if len(kbs) == 0 {
		return ""
	}

	// 转换为与原MCP工具一致的JSON格式
	type kbItem struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		DocCount    int32  `json:"doc_count"`
		ChunkCount  int32  `json:"chunk_count"`
	}
	type kbResponse struct {
		KnowledgeBases []kbItem `json:"knowledge_bases"`
	}
	resp := kbResponse{
		KnowledgeBases: make([]kbItem, 0, len(kbs)),
	}
	for _, kb := range kbs {
		resp.KnowledgeBases = append(resp.KnowledgeBases, kbItem{
			ID:          kb.ID,
			Name:        kb.Name,
			Description: kb.Description,
			DocCount:    kb.DocCount,
			ChunkCount:  kb.ChunkCount,
		})
	}
	data, _ := json.Marshal(resp)
	return string(data)
}

// BuildKnowledgePrompt 构建知识库检索结果的提示词
func BuildKnowledgePrompt(knowledgeResult string) string {
	trimmed := strings.TrimSpace(knowledgeResult)
	if trimmed == "" {
		return ""
	}
	return fmt.Sprintf("\n\n[知识库检索结果]\n%s\n\n请基于以上知识库内容回答用户的问题。如果知识库中没有相关信息，请根据你的知识回答。", trimmed)
}
