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

func SearchMemories(ctx context.Context, pool *Pool, query string, userID string, limit int) string {
	_, ok := pool.GetConnection("mem0")
	if !ok {
		return ""
	}

	result, err := pool.CallToolDirectly(ctx, "mem0", "search_memories", map[string]any{
		"query":   query,
		"user_id": userID,
		"limit":   limit,
	})
	if err != nil {
		klog.Errorf("搜索记忆失败: %v", err)
		return ""
	}
	return result
}

func SaveMemory(ctx context.Context, pool *Pool, content string, userID string, runID string) {
	_, ok := pool.GetConnection("mem0")
	if !ok {
		return
	}

	_, err := pool.CallToolDirectly(ctx, "mem0", "add_memory", map[string]any{
		"content": content,
		"user_id": userID,
		"run_id":  runID,
	})
	if err != nil {
		klog.Errorf("保存记忆失败: %v", err)
	}
}

func BuildMemoryPrompt(memories string) string {
	if memories == "" {
		return ""
	}
	return fmt.Sprintf("\n\n[用户相关记忆]\n%s", strings.TrimSpace(memories))
}
