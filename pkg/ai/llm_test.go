package ai

import (
	"context"
	"os"
	"testing"

	einomodel "github.com/cloudwego/eino-ext/components/model/openai"
)

// TestChatModelReadyNotInitialized 验证 ChatModel 未初始化时 ChatModelReady 返回 false
func TestChatModelReadyNotInitialized(t *testing.T) {
	original := chatModel
	chatModel = nil
	defer func() { chatModel = original }()

	if ChatModelReady() {
		t.Fatal("期望 ChatModelReady 返回 false，实际 true")
	}
}

// TestChatModelNameEmpty 验证未初始化时 ChatModelName 返回空串
func TestChatModelNameEmpty(t *testing.T) {
	original := chatModelName
	chatModelName = ""
	defer func() { chatModelName = original }()

	if name := ChatModelName(); name != "" {
		t.Errorf("期望空串，实际 %q", name)
	}
}

// TestChatNotReady 验证 ChatModel 未初始化时 Chat 返回 errChatModelNotReady
func TestChatNotReady(t *testing.T) {
	original := chatModel
	chatModel = nil
	defer func() { chatModel = original }()

	resp, err := Chat(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("期望返回 errChatModelNotReady，实际 nil")
	}
	if err != errChatModelNotReady {
		t.Errorf("期望 errChatModelNotReady，实际 %v", err)
	}
	if resp != "" {
		t.Errorf("期望空响应，实际 %q", resp)
	}
}

// TestChatModelReadyAfterSet 验证 chatModel 设置后 ChatModelReady 返回 true
// 通过构造一个真实 ChatModel 实例来测试（不发起网络请求，仅检测就绪状态）
func TestChatModelReadyAfterSet(t *testing.T) {
	original := chatModel
	originalName := chatModelName
	defer func() {
		chatModel = original
		chatModelName = originalName
	}()

	// 构造一个 ChatModel 实例（NewChatModel 仅创建客户端，不发请求）
	cm, err := einomodel.NewChatModel(context.Background(), &einomodel.ChatModelConfig{
		APIKey: "test-key",
		Model:  "test-model",
	})
	if err != nil {
		t.Fatalf("构造 ChatModel 失败: %v", err)
	}
	chatModel = cm
	chatModelName = "test-model"

	if !ChatModelReady() {
		t.Error("期望 ChatModelReady 返回 true，实际 false")
	}
	if name := ChatModelName(); name != "test-model" {
		t.Errorf("期望 test-model，实际 %q", name)
	}
}

// TestChatIntegration 真实 LLM 调用集成测试，需设置环境变量：
//   TEST_LLM_API_KEY / TEST_LLM_BASE_URL / TEST_LLM_MODEL
// 未设置时自动跳过
func TestChatIntegration(t *testing.T) {
	apiKey := os.Getenv("TEST_LLM_API_KEY")
	baseURL := os.Getenv("TEST_LLM_BASE_URL")
	modelName := os.Getenv("TEST_LLM_MODEL")
	if apiKey == "" || baseURL == "" || modelName == "" {
		t.Skip("TEST_LLM_API_KEY/BASE_URL/MODEL 未设置，跳过集成测试")
	}

	original := chatModel
	originalName := chatModelName
	defer func() {
		chatModel = original
		chatModelName = originalName
	}()

	cm, err := einomodel.NewChatModel(context.Background(), &einomodel.ChatModelConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   modelName,
		Timeout: defaultChatTimeout,
	})
	if err != nil {
		t.Fatalf("初始化 ChatModel 失败: %v", err)
	}
	chatModel = cm
	chatModelName = modelName

	resp, err := Chat(context.Background(), "", "请回复：pong")
	if err != nil {
		t.Fatalf("Chat 调用失败: %v", err)
	}
	if resp == "" {
		t.Fatal("Chat 返回空响应")
	}
	t.Logf("Chat 集成测试成功，响应长度: %d", len(resp))
}
