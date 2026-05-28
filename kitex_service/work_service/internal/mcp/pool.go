package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Airiseina/answer/pkg/meter"
	mcptool "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/kitex/pkg/klog"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpprotocol "github.com/mark3labs/mcp-go/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const defaultMcpTimeout = 15 * time.Second
const memoryMcpTimeout = 60 * time.Second
const healthCheckInterval = 30 * time.Second
const healthCheckTimeout = 5 * time.Second

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
	Cfg    ServerConfig
}

type Pool struct {
	mu       sync.RWMutex
	conns    map[string]*Connection
	cancelHC context.CancelFunc
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
	start := time.Now()
	cli, err := mcpclient.NewSSEMCPClient(cfg.URL,
		transport.WithResponseTimeout(memoryMcpTimeout),
		transport.WithEndpointTimeout(defaultMcpTimeout),
	)
	if err != nil {
		meter.M.McpConnectTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("server", cfg.Name)))
		meter.M.McpConnectErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("server", cfg.Name)))
		return nil, fmt.Errorf("创建SSE客户端失败[%s]: %w", cfg.Name, err)
	}
	cli.OnConnectionLost(func(err error) {
		klog.Warnf("MCP Pool: SSE连接断开[%s]: %v，将触发自动重连", cfg.Name, err)
		reconnCtx, reconnCancel := context.WithTimeout(context.Background(), defaultMcpTimeout)
		defer reconnCancel()
		if reconnErr := p.Reconnect(reconnCtx, cfg.Name); reconnErr != nil {
			klog.Errorf("MCP Pool: 断线自动重连 %s 失败: %v", cfg.Name, reconnErr)
		}
	})
	if err := cli.Start(context.Background()); err != nil {
		meter.M.McpConnectTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("server", cfg.Name)))
		meter.M.McpConnectErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("server", cfg.Name)))
		return nil, fmt.Errorf("启动SSE连接失败[%s]: %w", cfg.Name, err)
	}
	initCtx, initCancel := context.WithTimeout(ctx, defaultMcpTimeout)
	defer initCancel()
	initReq := mcpprotocol.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcpprotocol.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcpprotocol.Implementation{
		Name:    "answer-work-service",
		Version: "1.0.0",
	}
	if _, err := cli.Initialize(initCtx, initReq); err != nil {
		meter.M.McpConnectTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("server", cfg.Name)))
		meter.M.McpConnectErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("server", cfg.Name)))
		return nil, fmt.Errorf("MCP初始化失败[%s]: %w", cfg.Name, err)
	}
	tools, err := mcptool.GetTools(initCtx, &mcptool.Config{Cli: cli})
	if err != nil {
		meter.M.McpConnectTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("server", cfg.Name)))
		meter.M.McpConnectErrors.Add(ctx, 1, metric.WithAttributes(attribute.String("server", cfg.Name)))
		return nil, fmt.Errorf("获取MCP工具失败[%s]: %w", cfg.Name, err)
	}
	conn := &Connection{Client: cli, Tools: tools, Cfg: cfg}
	p.conns[cfg.Name] = conn
	meter.M.McpConnectTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("server", cfg.Name)))
	klog.Infof("MCP Pool: %s 已连接(耗时%v)，可用工具: %v", cfg.Name, time.Since(start), func() []string {
		toolNames := make([]string, 0, len(tools))
		for _, t := range tools {
			info, _ := t.Info(ctx)
			if info != nil {
				toolNames = append(toolNames, info.Name)
			}
		}
		return toolNames
	}())
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
	return p.CallToolWithTimeout(ctx, serverName, toolName, args, defaultMcpTimeout)
}

func (p *Pool) CallToolWithTimeout(ctx context.Context, serverName, toolName string, args map[string]any, timeout time.Duration) (string, error) {
	p.mu.RLock()
	conn, ok := p.conns[serverName]
	p.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("MCP Server %s 未连接", serverName)
	}
	start := time.Now()
	attrs := []attribute.KeyValue{
		attribute.String("server", serverName),
		attribute.String("tool", toolName),
	}
	callCtx, callCancel := context.WithTimeout(ctx, timeout)
	defer callCancel()
	result, err := conn.Client.CallTool(callCtx, mcpprotocol.CallToolRequest{
		Params: mcpprotocol.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	})
	if err != nil {
		meter.M.McpCallTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
		meter.M.McpCallErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
		meter.M.McpCallLatency.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attrs...))
		return "", fmt.Errorf("MCP调用失败[%s.%s]: %w", serverName, toolName, err)
	}
	meter.M.McpCallTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	meter.M.McpCallLatency.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attrs...))
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

func (p *Pool) HealthCheck(ctx context.Context) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var unhealthy []string
	for name, conn := range p.conns {
		hcCtx, hcCancel := context.WithTimeout(ctx, healthCheckTimeout)
		err := conn.Client.Ping(hcCtx)
		hcCancel()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			unhealthy = append(unhealthy, name)
			klog.Errorf("MCP Pool: 健康检查失败[%s]: %v", name, err)
		}
	}
	return unhealthy
}

func (p *Pool) Reconnect(ctx context.Context, serverName string) error {
	p.mu.Lock()
	oldConn, ok := p.conns[serverName]
	if !ok {
		p.mu.Unlock()
		return fmt.Errorf("MCP Server %s 不存在于连接池中", serverName)
	}
	cfg := oldConn.Cfg
	if oldConn.Client != nil {
		_ = oldConn.Client.Close()
	}
	delete(p.conns, serverName)
	p.mu.Unlock()
	_, err := p.Connect(ctx, cfg)
	if err != nil {
		return fmt.Errorf("重连 MCP Server %s 失败: %w", serverName, err)
	}
	meter.M.McpReconnectTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("server", serverName)))
	klog.Infof("MCP Pool: %s 重连成功", serverName)
	return nil
}

func (p *Pool) StartHealthCheck() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancelHC = cancel
	go func() {
		ticker := time.NewTicker(healthCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				unhealthy := p.HealthCheck(ctx)
				for _, name := range unhealthy {
					reconnCtx, reconnCancel := context.WithTimeout(context.Background(), defaultMcpTimeout)
					if err := p.Reconnect(reconnCtx, name); err != nil {
						klog.Errorf("MCP Pool: 自动重连 %s 失败: %v", name, err)
					}
					reconnCancel()
				}
			}
		}
	}()
	klog.Infof("MCP Pool: 健康检查已启动，间隔 %v", healthCheckInterval)
}

func (p *Pool) StopHealthCheck() {
	if p.cancelHC != nil {
		p.cancelHC()
		p.cancelHC = nil
		klog.Infof("MCP Pool: 健康检查已停止")
	}
}

func (p *Pool) Close() {
	p.StopHealthCheck()
	p.mu.Lock()
	defer p.mu.Unlock()
	for name, conn := range p.conns {
		if conn.Client != nil {
			_ = conn.Client.Close()
		}
		delete(p.conns, name)
	}
}
