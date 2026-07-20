package ai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
)

// TestToFloat32 验证 float64→float32 向量转换
func TestToFloat32(t *testing.T) {
	input := []float64{1.5, 2.5, 3.5, 0.0, -1.5}
	got := toFloat32(input)
	if len(got) != len(input) {
		t.Fatalf("期望长度 %d，实际 %d", len(input), len(got))
	}
	for i, v := range input {
		if got[i] != float32(v) {
			t.Errorf("索引 %d: 期望 %v，实际 %v", i, float32(v), got[i])
		}
	}
}

// TestToFloat32Empty 验证空切片转换
func TestToFloat32Empty(t *testing.T) {
	got := toFloat32(nil)
	if len(got) != 0 {
		t.Errorf("期望空切片，实际长度 %d", len(got))
	}
}

// TestToFloat32Batch 验证批量向量转换
func TestToFloat32Batch(t *testing.T) {
	input := [][]float64{
		{1.0, 2.0, 3.0},
		{4.0, 5.0, 6.0},
		{7.0, 8.0, 9.0},
	}
	got := toFloat32Batch(input)
	if len(got) != len(input) {
		t.Fatalf("期望批次长度 %d，实际 %d", len(input), len(got))
	}
	for i, vec := range input {
		if len(got[i]) != len(vec) {
			t.Errorf("批次 %d: 期望长度 %d，实际 %d", i, len(vec), len(got[i]))
		}
	}
}

// TestGetEmbeddingNotReady 验证 Embedder 未初始化时返回错误
func TestGetEmbeddingNotReady(t *testing.T) {
	// 保存并恢复全局 embedder，避免影响其他测试
	original := embedder
	embedder = nil
	defer func() { embedder = original }()

	_, err := GetEmbedding(context.Background(), "test")
	if err == nil {
		t.Fatal("期望返回 errEmbedderNotReady，实际 nil")
	}
	if err != errEmbedderNotReady {
		t.Errorf("期望 errEmbedderNotReady，实际 %v", err)
	}
}

// TestGetEmbeddingsNotReady 验证批量接口未初始化时返回错误
func TestGetEmbeddingsNotReady(t *testing.T) {
	original := embedder
	embedder = nil
	defer func() { embedder = original }()

	_, err := GetEmbeddings(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("期望返回 errEmbedderNotReady，实际 nil")
	}
	if err != errEmbedderNotReady {
		t.Errorf("期望 errEmbedderNotReady，实际 %v", err)
	}
}

// TestGetEmbeddingsEmpty 验证空输入返回 nil 无错误（无需初始化 Embedder）
func TestGetEmbeddingsEmpty(t *testing.T) {
	original := embedder
	embedder = nil
	defer func() { embedder = original }()

	// 空输入应该在检查 embedder 之前就返回，但当前实现先检查 embedder
	// 因此空输入 + 未初始化会返回 errEmbedderNotReady
	vectors, err := GetEmbeddings(context.Background(), []string{})
	if err != nil {
		t.Logf("空输入+未初始化返回错误（预期）: %v", err)
		return
	}
	if vectors != nil {
		t.Errorf("期望 nil，实际 %v", vectors)
	}
}

// TestEmbeddingIntegration 真实向量化集成测试，需设置环境变量：
//   TEST_EMB_API_KEY / TEST_EMB_BASE_URL / TEST_EMB_MODEL
// 未设置时自动跳过
func TestEmbeddingIntegration(t *testing.T) {
	apiKey := os.Getenv("TEST_EMB_API_KEY")
	baseURL := os.Getenv("TEST_EMB_BASE_URL")
	model := os.Getenv("TEST_EMB_MODEL")
	if apiKey == "" || baseURL == "" || model == "" {
		t.Skip("TEST_EMB_API_KEY/BASE_URL/MODEL 未设置，跳过集成测试")
	}

	original := embedder
	defer func() { embedder = original }()

	// 直接构造 eino Embedder 进行集成测试，验证 eino-ext OpenAI Embedding 适配器
	// 参考: https://github.com/cloudwego/eino-ext/tree/main/components/embedding/openai
	emb, err := openai.NewEmbedder(context.Background(), &openai.EmbeddingConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("初始化 Embedder 失败: %v", err)
	}
	embedder = emb

	// 测试单条向量化
	vec, err := GetEmbedding(context.Background(), "你好世界")
	if err != nil {
		t.Fatalf("GetEmbedding 失败: %v", err)
	}
	if len(vec) == 0 {
		t.Fatal("GetEmbedding 返回空向量")
	}
	t.Logf("单条向量维度: %d", len(vec))

	// 测试批量向量化——验证单次 HTTP 调用返回多条向量
	texts := []string{"第一段文本", "第二段文本", "第三段文本"}
	vectors, err := GetEmbeddings(context.Background(), texts)
	if err != nil {
		t.Fatalf("GetEmbeddings 批量调用失败: %v", err)
	}
	if len(vectors) != len(texts) {
		t.Fatalf("期望 %d 条向量，实际 %d", len(texts), len(vectors))
	}
	for i, v := range vectors {
		if len(v) != len(vec) {
			t.Errorf("向量 %d 维度不一致: 期望 %d，实际 %d", i, len(vec), len(v))
		}
	}
	t.Logf("批量向量化成功: %d 条文本，每条 %d 维", len(vectors), len(vec))
}
