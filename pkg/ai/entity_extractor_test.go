package ai

import (
	"context"
	"os"
	"testing"

	einomodel "github.com/cloudwego/eino-ext/components/model/openai"
)

// TestCleanJSONResponse 验证 markdown 代码块标记清理逻辑
// 部分 OpenAI 兼容端点不严格遵守 prompt 约束，需要容错
func TestCleanJSONResponse(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "纯JSON无代码块",
			input: `{"entities":[],"relations":[]}`,
			want:  `{"entities":[],"relations":[]}`,
		},
		{
			name:  "带json代码块标记",
			input: "```json\n{\"entities\":[],\"relations\":[]}\n```",
			want:  `{"entities":[],"relations":[]}`,
		},
		{
			name:  "带普通代码块标记",
			input: "```\n[\"关键词1\"]\n```",
			want:  `["关键词1"]`,
		},
		{
			name:  "仅开头有代码块标记",
			input: "```json\n{\"entities\":[]}",
			want:  `{"entities":[]}`,
		},
		{
			name:  "前后空白",
			input: "  {\"a\":1}  ",
			want:  `{"a":1}`,
		},
		{
			name:  "空字符串",
			input: "",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanJSONResponse(tc.input)
			if got != tc.want {
				t.Errorf("输入 %q\n期望 %q\n实际 %q", tc.input, tc.want, got)
			}
		})
	}
}

// TestTruncateForLog 验证日志截断函数
// 使用 rune 计数，正确处理中文多字节字符
func TestTruncateForLog(t *testing.T) {
	cases := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"短文本", 10, "短文本"},
		{"正好十个字正好十个字", 10, "正好十个字正好十个字"},
		{"这是一个超过最大长度的文本内容需要被截断", 10, "这是一个超过最大长度" + "..."},
		{"", 5, ""},
		{"abcdef", 3, "abc..."},
	}
	for _, tc := range cases {
		got := truncateForLog(tc.input, tc.maxLen)
		if got != tc.want {
			t.Errorf("输入 %q (maxLen=%d)\n期望 %q\n实际 %q", tc.input, tc.maxLen, tc.want, got)
		}
	}
}

// TestExtractEntitiesNotReady 验证 LLM 未初始化时 ExtractEntities 返回 errChatModelNotReady
func TestExtractEntitiesNotReady(t *testing.T) {
	original := chatModel
	chatModel = nil
	defer func() { chatModel = original }()

	result, err := ExtractEntities(context.Background(), "字节跳动成立于2012年")
	if err == nil {
		t.Fatal("期望返回 errChatModelNotReady，实际 nil")
	}
	if err != errChatModelNotReady {
		t.Errorf("期望 errChatModelNotReady，实际 %v", err)
	}
	if result != nil {
		t.Errorf("期望 nil，实际 %v", result)
	}
}

// TestExtractKeywordsNotReady 验证 LLM 未初始化时 ExtractKeywords 返回 errChatModelNotReady
func TestExtractKeywordsNotReady(t *testing.T) {
	original := chatModel
	chatModel = nil
	defer func() { chatModel = original }()

	keywords, err := ExtractKeywords(context.Background(), "字节跳动的产品")
	if err == nil {
		t.Fatal("期望返回 errChatModelNotReady，实际 nil")
	}
	if err != errChatModelNotReady {
		t.Errorf("期望 errChatModelNotReady，实际 %v", err)
	}
	if keywords != nil {
		t.Errorf("期望 nil，实际 %v", keywords)
	}
}

// TestExtractEntitiesEmptyInput 验证空输入在 LLM 未初始化前提前返回
// ExtractEntities 内部先检查 ChatModelReady，未就绪直接返回 errChatModelNotReady
// 即使空输入也会先触发就绪检查
func TestExtractEntitiesEmptyInput(t *testing.T) {
	original := chatModel
	chatModel = nil
	defer func() { chatModel = original }()

	_, err := ExtractEntities(context.Background(), "")
	if err != errChatModelNotReady {
		t.Errorf("期望 errChatModelNotReady，实际 %v", err)
	}
}

// TestExtractKeywordsEmptyInput 验证空查询在 LLM 未初始化前提前返回
func TestExtractKeywordsEmptyInput(t *testing.T) {
	original := chatModel
	chatModel = nil
	defer func() { chatModel = original }()

	_, err := ExtractKeywords(context.Background(), "")
	if err != errChatModelNotReady {
		t.Errorf("期望 errChatModelNotReady，实际 %v", err)
	}
}

// TestExtractEntitiesIntegration 真实 LLM 实体抽取集成测试
// 需设置环境变量 TEST_LLM_API_KEY / TEST_LLM_BASE_URL / TEST_LLM_MODEL
// 未设置时自动跳过
func TestExtractEntitiesIntegration(t *testing.T) {
	apiKey := os.Getenv("TEST_LLM_API_KEY")
	baseURL := os.Getenv("TEST_LLM_BASE_URL")
	modelName := os.Getenv("TEST_LLM_MODEL")
	if apiKey == "" || baseURL == "" || modelName == "" {
		t.Skip("TEST_LLM_API_KEY/BASE_URL/MODEL 未设置，跳过集成测试")
	}

	original := chatModel
	defer func() { chatModel = original }()

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

	text := "字节跳动成立于2012年，总部位于北京，旗下拥有抖音、TikTok等产品。"
	result, err := ExtractEntities(context.Background(), text)
	if err != nil {
		t.Fatalf("ExtractEntities 失败: %v", err)
	}
	if result == nil {
		t.Fatal("ExtractEntities 返回 nil")
	}
	t.Logf("抽取到 %d 个实体, %d 个关系", len(result.Entities), len(result.Relations))
	for _, e := range result.Entities {
		t.Logf("  实体: name=%q type=%q description=%q", e.Name, e.Type, e.Description)
	}
}

// TestExtractKeywordsIntegration 真实 LLM 关键词抽取集成测试
func TestExtractKeywordsIntegration(t *testing.T) {
	apiKey := os.Getenv("TEST_LLM_API_KEY")
	baseURL := os.Getenv("TEST_LLM_BASE_URL")
	modelName := os.Getenv("TEST_LLM_MODEL")
	if apiKey == "" || baseURL == "" || modelName == "" {
		t.Skip("TEST_LLM_API_KEY/BASE_URL/MODEL 未设置，跳过集成测试")
	}

	original := chatModel
	defer func() { chatModel = original }()

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

	keywords, err := ExtractKeywords(context.Background(), "字节跳动的抖音产品有什么特点？")
	if err != nil {
		t.Fatalf("ExtractKeywords 失败: %v", err)
	}
	if len(keywords) == 0 {
		t.Fatal("ExtractKeywords 返回空列表")
	}
	t.Logf("抽取到关键词: %v", keywords)
}
