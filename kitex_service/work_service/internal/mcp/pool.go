package mcp

import (
	"context"
	"fmt"
	"sync"

	mcptool "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/kitex/pkg/klog"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpprotocol "github.com/mark3labs/mcp-go/mcp"
)

type ServerConfig struct {
	Name      string
	URL       string
	Transport string
	AuthType  string
	AuthToken string
}

type Connection struct {
	Client mcpclient.MCPClient
	Tools  []tool.BaseTool
}

type Pool struct {
	mu    sync.RWMutex
	conns map[string]*Connection
}

func NewPool() *Pool {
	return &Pool{
		conns: make(map[string]*Connection),
	}
}

func (p *Pool) Connect(ctx context.Context, cfg ServerConfig) (*Connection, error) {
	p.mu.RLock()
	if conn, ok := p.conns[cfg.Name]; ok {
		p.mu.RUnlock()
		return conn, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, ok := p.conns[cfg.Name]; ok {
		return conn, nil
	}

	klog.Infof("MCP Pool: 正在连接 %s (%s)", cfg.Name, cfg.URL)

	cli, err := mcpclient.NewSSEMCPClient(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("创建SSE客户端失败[%s]: %w", cfg.Name, err)
	}

	if err := cli.Start(ctx); err != nil {
		return nil, fmt.Errorf("启动SSE连接失败[%s]: %w", cfg.Name, err)
	}

	initReq := mcpprotocol.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcpprotocol.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcpprotocol.Implementation{
		Name:    "answer-work-service",
		Version: "1.0.0",
	}
	if _, err := cli.Initialize(ctx, initReq); err != nil {
		return nil, fmt.Errorf("MCP初始化失败[%s]: %w", cfg.Name, err)
	}

	tools, err := mcptool.GetTools(ctx, &mcptool.Config{Cli: cli})
	if err != nil {
		return nil, fmt.Errorf("获取MCP工具失败[%s]: %w", cfg.Name, err)
	}

	conn := &Connection{Client: cli, Tools: tools}
	p.conns[cfg.Name] = conn

	toolNames := make([]string, 0, len(tools))
	for _, t := range tools {
		info, _ := t.Info(ctx)
		if info != nil {
			toolNames = append(toolNames, info.Name)
		}
	}
	klog.Infof("MCP Pool: %s 已连接，可用工具: %v", cfg.Name, toolNames)

	return conn, nil
}

func (p *Pool) GetAllTools(ctx context.Context, servers []ServerConfig) ([]tool.BaseTool, error) {
	var allTools []tool.BaseTool
	for _, s := range servers {
		conn, err := p.Connect(ctx, s)
		if err != nil {
			klog.Errorf("MCP Pool: 连接 %s 失败: %v", s.Name, err)
			continue
		}
		allTools = append(allTools, conn.Tools...)
	}
	return allTools, nil
}

func (p *Pool) CallToolDirectly(ctx context.Context, serverName, toolName string, args map[string]any) (string, error) {
	p.mu.RLock()
	conn, ok := p.conns[serverName]
	p.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("MCP Server %s 未连接", serverName)
	}

	result, err := conn.Client.CallTool(ctx, mcpprotocol.CallToolRequest{
		Params: mcpprotocol.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	})
	if err != nil {
		return "", fmt.Errorf("MCP调用失败[%s.%s]: %w", serverName, toolName, err)
	}

	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(mcpprotocol.TextContent); ok {
			return textContent.Text, nil
		}
	}
	return fmt.Sprintf("%v", result.Content), nil
}

func (p *Pool) GetConnection(serverName string) (*Connection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	conn, ok := p.conns[serverName]
	return conn, ok
}

func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for name, conn := range p.conns {
		if conn.Client != nil {
			_ = conn.Client.Close()
		}
		delete(p.conns, name)
	}
}
