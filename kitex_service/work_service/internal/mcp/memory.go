package mcp

import (
	"context"
	"fmt"
	"strings"

	bot "github.com/Airiseina/answer/kitex_service/bot_service/kitex_gen/bot"
	"github.com/cloudwego/kitex/pkg/klog"
)

func GetMcpServersForBot(ctx context.Context, botId int64, rpcGetBotMcpServers func(ctx context.Context, botId int64) (*bot.GetBotMcpServersRes, error)) []ServerConfig {
	resp, err := rpcGetBotMcpServers(ctx, botId)
	if err != nil {
		klog.Errorf("获取Bot[%d]的MCP Server列表失败: %v", botId, err)
		return nil
	}
	var configs []ServerConfig
	for _, s := range resp.Servers {
		if !s.Enabled {
			continue
		}
		configs = append(configs, ServerConfig{
			Name:      s.Name,
			URL:       s.Url,
			Transport: s.Transport,
			AuthType:  s.AuthType,
			AuthToken: s.GetAuthToken(),
		})
	}
	return configs
}

func ensureMem0Connected(ctx context.Context, pool *Pool) error {
	if _, ok := pool.GetConnection("mem0"); ok {
		return nil
	}
	builtinServers := GetBuiltinServers()
	var mem0Cfg ServerConfig
	found := false
	for _, s := range builtinServers {
		if s.Name == "mem0" && s.URL != "" {
			mem0Cfg = ServerConfig{
				Name:      s.Name,
				URL:       s.URL,
				Transport: s.Transport,
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("mem0配置未找到或URL为空")
	}
	klog.Infof("记忆操作: mem0未连接，正在自动连接 %s", mem0Cfg.URL)
	_, err := pool.Connect(ctx, mem0Cfg)
	if err != nil {
		return fmt.Errorf("自动连接mem0失败: %w", err)
	}
	klog.Infof("记忆操作: mem0自动连接成功")
	return nil
}

func isMcpToolError(result string) bool {
	return strings.HasPrefix(result, "Error executing tool") || strings.Contains(result, `"error"`)
}

func SearchMemories(ctx context.Context, pool *Pool, query string, userID string, runID string, limit int) string {
	if err := ensureMem0Connected(ctx, pool); err != nil {
		klog.Errorf("确保mem0连接失败: %v", err)
		return ""
	}
	args := map[string]any{
		"query":   query,
		"user_id": userID,
		"limit":   limit,
	}
	if runID != "" {
		args["run_id"] = runID
	}
	result, err := pool.CallToolWithTimeout(ctx, "mem0", "search_memories", args, memoryMcpTimeout)
	if err != nil {
		klog.Errorf("搜索记忆失败: %v", err)
		return ""
	}
	if isMcpToolError(result) {
		klog.Errorf("搜索记忆返回错误: %s", result)
		return ""
	}
	return result
}

func SaveMemory(ctx context.Context, pool *Pool, content string, userID string, runID string) {
	if err := ensureMem0Connected(ctx, pool); err != nil {
		klog.Errorf("确保mem0连接失败: %v", err)
		return
	}
	result, err := pool.CallToolWithTimeout(ctx, "mem0", "add_memory", map[string]any{
		"content": content,
		"user_id": userID,
		"run_id":  runID,
	}, memoryMcpTimeout)
	if err != nil {
		klog.Errorf("保存记忆失败: %v", err)
		return
	}
	if isMcpToolError(result) {
		klog.Errorf("保存记忆返回错误: %s", result)
	}
}

func BuildMemoryPrompt(memories string) string {
	trimmed := strings.TrimSpace(memories)
	if trimmed == "" {
		return ""
	}
	return fmt.Sprintf("\n\n[用户相关记忆]\n%s", trimmed)
}
