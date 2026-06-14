package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	searxngBaseURL string
	searxngTimeout time.Duration
	httpClient     *http.Client
)

func init() {
	searxngBaseURL = getEnv("SEARXNG_BASE_URL", "http://answer_searxng:8080")
	timeoutSec, _ := strconv.ParseFloat(getEnv("SEARXNG_TIMEOUT", "8.0"), 64)
	if timeoutSec <= 0 {
		timeoutSec = 8.0
	}
	searxngTimeout = time.Duration(timeoutSec * float64(time.Second))
	httpClient = &http.Client{
		Timeout: searxngTimeout + 3*time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	log.Printf("SearXNG MCP Server 配置: URL=%s, Timeout=%.1fs", searxngBaseURL, searxngTimeout.Seconds())
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// searxngSearchResult SearXNG搜索结果项
type searxngSearchResult struct {
	Title         string `json:"title"`
	URL           string `json:"url"`
	Content       string `json:"content"`
	PublishedDate string `json:"publishedDate,omitempty"`
}

// searxngResponse SearXNG API响应
type searxngResponse struct {
	Results []searxngSearchResult `json:"results"`
}

// searchResult 搜索结果输出
type searchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Date    string `json:"date,omitempty"`
}

func searchSearXNG(ctx context.Context, query, categories string, maxResults int) (string, error) {
	maxResults = clamp(maxResults, 1, 10)

	reqURL := fmt.Sprintf("%s/search", strings.TrimRight(searxngBaseURL, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	q := req.URL.Query()
	q.Set("q", query)
	q.Set("format", "json")
	q.Set("categories", categories)
	q.Set("language", "zh-CN")
	q.Set("pageno", "1")
	req.URL.RawQuery = q.Encode()

	resp, err := httpClient.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return "搜索服务连接失败，请稍后再试。", nil
		}
		if strings.Contains(err.Error(), "context deadline") || os.IsTimeout(err) {
			return "搜索请求超时，请稍后再试或简化搜索词。", nil
		}
		return "", fmt.Errorf("搜索请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("搜索服务暂时不可用(HTTP %d)，请稍后再试或换一种方式提问。", resp.StatusCode), nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var data searxngResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	results := make([]searchResult, 0, maxResults)
	for i, item := range data.Results {
		if i >= maxResults {
			break
		}
		r := searchResult{
			Title:   item.Title,
			URL:     item.URL,
			Snippet: item.Content,
		}
		if item.PublishedDate != "" {
			r.Date = item.PublishedDate
		}
		results = append(results, r)
	}

	output := map[string]interface{}{
		"results": results,
		"total":   len(results),
	}
	outputBytes, _ := json.Marshal(output)
	return string(outputBytes), nil
}

func clamp(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func main() {
	// 创建MCP Server
	mcpServer := server.NewMCPServer(
		"searxng",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// 注册web_search工具
	webSearchTool := mcp.NewTool("web_search",
		mcp.WithDescription("搜索联网获取最新信息，用于需要实时数据的场合。返回标题、链接、摘要。"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("搜索关键词"),
		),
		mcp.WithNumber("max_results",
			mcp.Description("返回结果数(默认5,最多10)"),
		),
	)
	mcpServer.AddTool(webSearchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := request.GetString("query", "")
		if query == "" {
			return mcp.NewToolResultError("query参数不能为空"), nil
		}
		maxResults := request.GetInt("max_results", 5)

		result, err := searchSearXNG(ctx, query, "general", maxResults)
		if err != nil {
			log.Printf("web_search失败: %v", err)
			return mcp.NewToolResultError("搜索时发生错误，请稍后再试。"), nil
		}
		return mcp.NewToolResultText(result), nil
	})

	// 注册news_search工具
	newsSearchTool := mcp.NewTool("news_search",
		mcp.WithDescription("搜索最新新闻。用于获取近期报道。"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("搜索关键词"),
		),
		mcp.WithNumber("max_results",
			mcp.Description("返回结果数(默认5,最多10)"),
		),
	)
	mcpServer.AddTool(newsSearchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := request.GetString("query", "")
		if query == "" {
			return mcp.NewToolResultError("query参数不能为空"), nil
		}
		maxResults := request.GetInt("max_results", 5)

		result, err := searchSearXNG(ctx, query, "news", maxResults)
		if err != nil {
			log.Printf("news_search失败: %v", err)
			return mcp.NewToolResultError("新闻搜索时发生错误，请稍后再试。"), nil
		}
		return mcp.NewToolResultText(result), nil
	})

	// 启动SSE Server
	port := getEnv("PORT", "8001")
	sseServer := server.NewSSEServer(mcpServer, server.WithSSEEndpoint("/sse"))

	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: sseServer,
	}

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("正在关闭SearXNG MCP Server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()

	log.Printf("SearXNG MCP Server 启动，监听端口 %s", port)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务启动失败: %v", err)
	}
}
