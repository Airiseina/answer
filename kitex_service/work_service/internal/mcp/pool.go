package mcp

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Airiseina/answer/kitex_service/work_service/internal/config"
	"github.com/Airiseina/answer/pkg/observability/meter"
	mcptool "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/kitex/pkg/klog"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpprotocol "github.com/mark3labs/mcp-go/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const defaultMcpTimeout = 20 * time.Second
const memoryMcpTimeout = 60 * time.Second
const healthCheckInterval = 60 * time.Second
const healthCheckTimeout = 10 * time.Second

type ServerConfig struct {
	Name      string
	URL       string
	Transport string
	AuthType  string
	AuthToken string
}

type Connection struct {
	Client   mcpclient.MCPClient
	Tools    []tool.BaseTool
	Cfg      ServerConfig
	lastUsed atomic.Int64
}

type Pool struct {
	mu         sync.RWMutex
	conns      map[string]*Connection
	cancelHC   context.CancelFunc
	reconnMu   map[string]*sync.Mutex
	reconnMuMu sync.Mutex
}

func NewPool() *Pool {
	return &Pool{
		conns:    make(map[string]*Connection),
		reconnMu: make(map[string]*sync.Mutex),
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
	conn.lastUsed.Store(time.Now().Unix())
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

// RetryConfig MCP调用重试配置
type RetryConfig struct {
	MaxAttempts     int           // 最大重试次数(不含首次调用)
	InitialInterval time.Duration // 首次重试间隔
}

// DefaultRetryConfig 从配置文件读取重试参数
func DefaultRetryConfig() RetryConfig {
	v := config.V
	return RetryConfig{
		MaxAttempts:     v.GetInt("mcp.retry.max_attempts"),
		InitialInterval: time.Duration(v.GetInt("mcp.retry.initial_interval")) * time.Second,
	}
}

// isRetryableError 判断错误是否可重试(连接断开、超时、transport错误)
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection has been closed") ||
		strings.Contains(errStr, "connection closed") ||
		strings.Contains(errStr, "transport error") ||
		strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "timeout")
}

// isConnectionError 判断错误是否为连接断开(需要重连)
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection has been closed") ||
		strings.Contains(errStr, "connection closed") ||
		strings.Contains(errStr, "transport error")
}

func (p *Pool) CallToolWithTimeout(ctx context.Context, serverName, toolName string, args map[string]any, timeout time.Duration) (string, error) {
	retryCfg := DefaultRetryConfig()
	return p.callToolWithRetry(ctx, serverName, toolName, args, timeout, retryCfg)
}

// callToolWithRetry 带指数退避重试的MCP调用
func (p *Pool) callToolWithRetry(ctx context.Context, serverName, toolName string, args map[string]any, timeout time.Duration, retryCfg RetryConfig) (string, error) {
	p.mu.RLock()
	_, ok := p.conns[serverName]
	p.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("MCP Server %s 未连接", serverName)
	}

	attrs := []attribute.KeyValue{
		attribute.String("server", serverName),
		attribute.String("tool", toolName),
	}

	var lastErr error
	for attempt := 0; attempt <= retryCfg.MaxAttempts; attempt++ {
		if attempt > 0 {
			// 指数退避: InitialInterval * 2^(attempt-1)
			backoff := retryCfg.InitialInterval * time.Duration(math.Pow(2, float64(attempt-1)))
			klog.Infof("MCP调用重试[%s.%s]第%d次，等待%v", serverName, toolName, attempt, backoff)
			meter.M.McpRetryTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

			select {
			case <-ctx.Done():
				meter.M.McpCallTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
				meter.M.McpCallErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
				return "", fmt.Errorf("MCP调用[%s.%s]重试被取消: %w", serverName, toolName, ctx.Err())
			case <-time.After(backoff):
			}
		}

		result, err := p.callToolOnce(ctx, serverName, toolName, args, timeout, attrs)
		if err == nil {
			return result, nil
		}
		lastErr = err

		// 不可重试的错误直接返回
		if !isRetryableError(err) {
			return "", err
		}

		// 超时错误记录指标
		if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "timeout") {
			meter.M.McpCallTimeoutTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
		}

		// 连接断开时尝试重连
		if isConnectionError(err) {
			klog.Warnf("MCP调用连接已断开[%s.%s]，尝试重连: %v", serverName, toolName, err)
			reconnCtx, reconnCancel := context.WithTimeout(context.Background(), defaultMcpTimeout)
			reconnErr := p.Reconnect(reconnCtx, serverName)
			reconnCancel()
			if reconnErr != nil {
				klog.Errorf("MCP调用[%s.%s]重连失败: %v", serverName, toolName, reconnErr)
				// 重连失败，继续重试
			}
		}

		klog.Warnf("MCP调用[%s.%s]第%d次尝试失败: %v", serverName, toolName, attempt+1, err)
	}

	meter.M.McpCallTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	meter.M.McpCallErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	return "", fmt.Errorf("MCP调用[%s.%s]重试%d次后仍失败: %w", serverName, toolName, retryCfg.MaxAttempts, lastErr)
}

// callToolOnce 单次MCP调用(不含重试逻辑)
func (p *Pool) callToolOnce(ctx context.Context, serverName, toolName string, args map[string]any, timeout time.Duration, attrs []attribute.KeyValue) (string, error) {
	p.mu.RLock()
	conn, ok := p.conns[serverName]
	p.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("MCP Server %s 未连接", serverName)
	}

	start := time.Now()
	callCtx, callCancel := context.WithTimeout(ctx, timeout)
	defer callCancel()
	conn.lastUsed.Store(time.Now().Unix())

	result, err := conn.Client.CallTool(callCtx, mcpprotocol.CallToolRequest{
		Params: mcpprotocol.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	})
	if err != nil {
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

// FallbackFunc 降级函数类型，当MCP调用失败时执行
type FallbackFunc func(ctx context.Context, serverName, toolName string, args map[string]any, err error) (string, error)

// CallToolWithFallback 带降级的MCP调用，当主调用失败时执行降级函数
func (p *Pool) CallToolWithFallback(ctx context.Context, serverName, toolName string, args map[string]any, timeout time.Duration, fallback FallbackFunc) (string, error) {
	result, err := p.CallToolWithTimeout(ctx, serverName, toolName, args, timeout)
	if err != nil {
		fallbackEnabled := config.V.GetBool("mcp.fallback.enabled")
		if !fallbackEnabled || fallback == nil {
			return "", err
		}
		attrs := []attribute.KeyValue{
			attribute.String("server", serverName),
			attribute.String("tool", toolName),
		}
		meter.M.McpFallbackTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
		klog.Warnf("MCP调用[%s.%s]失败，执行降级: %v", serverName, toolName, err)
		return fallback(ctx, serverName, toolName, args, err)
	}
	return result, nil
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
	now := time.Now().Unix()
	for name, conn := range p.conns {
		lastUsed := conn.lastUsed.Load()
		if now-lastUsed < int64(healthCheckInterval/time.Second)*2 {
			continue
		}
		hcCtx, hcCancel := context.WithTimeout(ctx, healthCheckTimeout)
		err := conn.Client.Ping(hcCtx)
		hcCancel()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			unhealthy = append(unhealthy, name)
			klog.Errorf("MCP Pool: 健康检查失败[%s]: %v", name, err)
		} else {
			conn.lastUsed.Store(now)
		}
	}
	return unhealthy
}

func (p *Pool) getReconnMu(serverName string) *sync.Mutex {
	p.reconnMuMu.Lock()
	defer p.reconnMuMu.Unlock()
	if mu, ok := p.reconnMu[serverName]; ok {
		return mu
	}
	mu := &sync.Mutex{}
	p.reconnMu[serverName] = mu
	return mu
}

func (p *Pool) Reconnect(ctx context.Context, serverName string) error {
	mu := p.getReconnMu(serverName)
	mu.Lock()
	defer mu.Unlock()

	p.mu.Lock()
	oldConn, ok := p.conns[serverName]
	if !ok {
		p.mu.Unlock()
		return fmt.Errorf("MCP Server %s 不存在于连接池中", serverName)
	}
	cfg := oldConn.Cfg
	if oldConn.Client != nil {
		if setter, ok := oldConn.Client.(interface{ OnConnectionLost(func(error)) }); ok {
			setter.OnConnectionLost(nil)
		}
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
