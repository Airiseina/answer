package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestNewClient 验证客户端构造
func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("NewClient() returned nil")
	}
}

// TestChatCancelledContext 验证取消的上下文会返回错误，且不依赖真实网络
func TestChatCancelledContext(t *testing.T) {
	c := NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := c.Chat(ctx, "fake-key", "http://127.0.0.1:1", "fake-model", "system", nil, "user", nil)
	if err == nil {
		t.Fatal("期望返回错误，实际为 nil")
	}
	t.Logf("取消上下文错误（预期）: %v", err)
}

// TestChatWithMockServer 通过 mock OpenAI 兼容服务器验证完整 Chat 流程
// 覆盖：消息构造（system/history/user）、HTTP 调用、响应解析
func TestChatWithMockServer(t *testing.T) {
	// 启动 mock OpenAI 兼容服务器，使用根路由捕获所有路径
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 校验请求方法和路径
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST 请求，实际 %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("期望路径以 /chat/completions 结尾，实际 %s", r.URL.Path)
		}

		// 解析请求体，校验消息构造
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("解析请求体失败: %v", err)
		}
		messages, ok := payload["messages"].([]any)
		if !ok || len(messages) < 2 {
			t.Errorf("期望至少 2 条消息，实际 %v", payload["messages"])
		}
		// 校验模型名传递
		if payload["model"] != "test-model" {
			t.Errorf("期望 model=test-model，实际 %v", payload["model"])
		}

		// 返回标准 OpenAI ChatCompletion 响应
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "hello from mock"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 构造含 system + history + user 的完整对话
	history := []ChatMessage{
		{Role: "user", Content: "previous question"},
		{Role: "assistant", Content: "previous answer"},
	}

	resp, err := c.Chat(ctx, "fake-key", server.URL, "test-model", "you are helpful", history, "hi", nil)
	if err != nil {
		t.Fatalf("Chat 调用失败: %v", err)
	}
	if resp != "hello from mock" {
		t.Errorf("期望 'hello from mock'，实际 %q", resp)
	}
}

// TestChatWithImageAndMockServer 验证多模态消息构造（含图片）
func TestChatWithImageAndMockServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		// 校验最后一条消息是多模态格式
		messages := payload["messages"].([]any)
		last := messages[len(messages)-1].(map[string]any)
		content, ok := last["content"].([]any)
		if !ok {
			t.Errorf("期望多模态消息 content 为数组，实际 %T", last["content"])
			return
		}
		if len(content) < 2 {
			t.Errorf("期望至少 2 个 content part（text+image），实际 %d", len(content))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-img",
			"object": "chat.completion",
			"model": "test-model",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "image described"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	img := &ImageData{
		Base64Data: "aGVsbG8=", // "hello" 的 base64
		MIMEType:   "image/png",
	}
	resp, err := c.Chat(ctx, "fake-key", server.URL, "test-model", "", nil, "describe this image", img)
	if err != nil {
		t.Fatalf("Chat 多模态调用失败: %v", err)
	}
	if resp != "image described" {
		t.Errorf("期望 'image described'，实际 %q", resp)
	}
}

// TestChatIntegration 真实 LLM 集成测试，需设置环境变量：
//   TEST_LLM_API_KEY / TEST_LLM_BASE_URL / TEST_LLM_MODEL
// 未设置时自动跳过，便于在 CI 中无网络环境运行
func TestChatIntegration(t *testing.T) {
	apiKey := os.Getenv("TEST_LLM_API_KEY")
	baseURL := os.Getenv("TEST_LLM_BASE_URL")
	modelName := os.Getenv("TEST_LLM_MODEL")
	if apiKey == "" || baseURL == "" || modelName == "" {
		t.Skip("TEST_LLM_API_KEY/BASE_URL/MODEL 未设置，跳过集成测试")
	}

	c := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := c.Chat(ctx, apiKey, baseURL, modelName, "You are a helpful assistant.", nil, "Say 'hello' in one word.", nil)
	if err != nil {
		t.Fatalf("集成测试 Chat 失败: %v", err)
	}
	if resp == "" {
		t.Fatal("集成测试返回空响应")
	}
	t.Logf("集成测试响应: %s", resp)
}
