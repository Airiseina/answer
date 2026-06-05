package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
)

const knowledgeMcpTimeout = 30 * time.Second

func ensureKnowledgeConnected(ctx context.Context, pool *Pool) error {
	if _, ok := pool.GetConnection("knowledge"); ok {
		return nil
	}
	builtinServers := GetBuiltinServers()
	var knowledgeCfg ServerConfig
	found := false
	for _, s := range builtinServers {
		if s.Name == "knowledge" && s.URL != "" {
			knowledgeCfg = ServerConfig{
				Name:      s.Name,
				URL:       s.URL,
				Transport: s.Transport,
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("knowledge配置未找到或URL为空")
	}
	klog.Infof("知识库操作: knowledge未连接，正在自动连接 %s", knowledgeCfg.URL)
	_, err := pool.Connect(ctx, knowledgeCfg)
	if err != nil {
		return fmt.Errorf("自动连接knowledge失败: %w", err)
	}
	klog.Infof("知识库操作: knowledge自动连接成功")
	return nil
}

func SearchKnowledge(ctx context.Context, pool *Pool, query string, kbIDs string, topK int) string {
	if err := ensureKnowledgeConnected(ctx, pool); err != nil {
		klog.Errorf("确保knowledge连接失败: %v", err)
		return ""
	}
	args := map[string]any{
		"query":  query,
		"kb_ids": kbIDs,
		"top_k":  topK,
	}
	result, err := pool.CallToolWithFallback(ctx, "knowledge", "search_knowledge", args, knowledgeMcpTimeout, func(ctx context.Context, serverName, toolName string, args map[string]any, err error) (string, error) {
		klog.Warnf("知识库搜索降级: %v", err)
		return "", nil
	})
	if err != nil {
		klog.Errorf("搜索知识库失败: %v", err)
		return ""
	}
	if isMcpToolError(result) {
		klog.Errorf("搜索知识库返回错误: %s", result)
		return ""
	}
	return result
}

func GetBotKnowledgeBases(ctx context.Context, pool *Pool, botID string) string {
	if err := ensureKnowledgeConnected(ctx, pool); err != nil {
		klog.Errorf("确保knowledge连接失败: %v", err)
		return ""
	}
	args := map[string]any{
		"bot_id": botID,
	}
	result, err := pool.CallToolWithFallback(ctx, "knowledge", "get_bot_knowledge_bases", args, knowledgeMcpTimeout, func(ctx context.Context, serverName, toolName string, args map[string]any, err error) (string, error) {
		klog.Warnf("获取Bot知识库列表降级: %v", err)
		return "", nil
	})
	if err != nil {
		klog.Errorf("获取Bot知识库列表失败: %v", err)
		return ""
	}
	if isMcpToolError(result) {
		klog.Errorf("获取Bot知识库列表返回错误: %s", result)
		return ""
	}
	return result
}

func BuildKnowledgePrompt(knowledgeResult string) string {
	trimmed := strings.TrimSpace(knowledgeResult)
	if trimmed == "" {
		return ""
	}
	return fmt.Sprintf("\n\n[知识库检索结果]\n%s\n\n请基于以上知识库内容回答用户的问题。如果知识库中没有相关信息，请根据你的知识回答。", trimmed)
}
