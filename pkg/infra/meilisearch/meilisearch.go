package meilisearch

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Airiseina/answer/pkg/observability/logger"

	"github.com/meilisearch/meilisearch-go"
	"go.uber.org/zap"
)

type MeilisearchDao struct {
	Client meilisearch.ServiceManager
}

const KbChunksIndex = "kb_chunks"

const meilisearchWaitTimeout = 30 * time.Second

func MeilisearchInit(host string, apiKey string) (meilisearch.ServiceManager, error) {
	client := meilisearch.New(host, meilisearch.WithAPIKey(apiKey))
	_, err := client.GetStats()
	if err != nil {
		logger.Error("无法连接到Meilisearch", zap.Error(err))
		return nil, fmt.Errorf("连接Meilisearch失败: %w", err)
	}
	return client, nil
}

func NewMeilisearchDao(client meilisearch.ServiceManager) *MeilisearchDao {
	return &MeilisearchDao{Client: client}
}

// EnsureIndex 确保知识库分块索引存在，并配置可搜索字段和过滤字段
func (m *MeilisearchDao) EnsureIndex(ctx context.Context) error {
	_, err := m.Client.GetIndex(KbChunksIndex)
	if err != nil {
		task, err := m.Client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        KbChunksIndex,
			PrimaryKey: "chunk_id_str",
		})
		if err != nil {
			return fmt.Errorf("创建Meilisearch索引失败: %w", err)
		}
		if _, err := m.Client.WaitForTask(task.TaskUID, meilisearchWaitTimeout); err != nil {
			return fmt.Errorf("等待Meilisearch索引创建失败: %w", err)
		}
	}
	_, err = m.Client.Index(KbChunksIndex).UpdateSearchableAttributes(&[]string{
		"content",
		"source",
	})
	if err != nil {
		logger.Warn("设置Meilisearch可搜索字段失败", zap.Error(err))
	}
	_, err = m.Client.Index(KbChunksIndex).UpdateFilterableAttributes(&[]string{
		"kb_id",
		"doc_id",
	})
	if err != nil {
		logger.Warn("设置Meilisearch过滤字段失败", zap.Error(err))
	}
	return nil
}

// ChunkDocument Meilisearch中存储的分块文档
type ChunkDocument struct {
	ChunkIDStr string `json:"chunk_id_str"`
	ChunkID    int64  `json:"chunk_id"`
	KBID       int64  `json:"kb_id"`
	DocID      int64  `json:"doc_id"`
	Content    string `json:"content"`
	ChunkIndex int    `json:"chunk_index"`
	Source     string `json:"source"`
	PageNumber int    `json:"page_number,omitempty"`
}

// IndexChunks 将分块文档索引到Meilisearch
func (m *MeilisearchDao) IndexChunks(ctx context.Context, docs []ChunkDocument) error {
	if len(docs) == 0 {
		return nil
	}
	task, err := m.Client.Index(KbChunksIndex).AddDocuments(docs)
	if err != nil {
		return fmt.Errorf("索引分块到Meilisearch失败: %w", err)
	}
	if _, err := m.Client.WaitForTask(task.TaskUID, meilisearchWaitTimeout); err != nil {
		return fmt.Errorf("等待Meilisearch索引完成失败: %w", err)
	}
	return nil
}

// BM25SearchResult BM25检索结果
type BM25SearchResult struct {
	ChunkID    int64   `json:"chunk_id"`
	KBID       int64   `json:"kb_id"`
	DocID      int64   `json:"doc_id"`
	Content    string  `json:"content"`
	ChunkIndex int     `json:"chunk_index"`
	Source     string  `json:"source"`
	PageNumber int     `json:"page_number,omitempty"`
	Rank       int     `json:"-"`
	Score      float64 `json:"-"`
}

// SearchBM25 使用Meilisearch进行BM25关键词检索
func (m *MeilisearchDao) SearchBM25(ctx context.Context, kbIDs []int64, query string, topK int) ([]BM25SearchResult, error) {
	var filterStrs []string
	for _, id := range kbIDs {
		filterStrs = append(filterStrs, fmt.Sprintf("kb_id = %d", id))
	}
	filterStr := ""
	if len(filterStrs) > 1 {
		filterStr = "(" + joinOr(filterStrs) + ")"
	} else if len(filterStrs) == 1 {
		filterStr = filterStrs[0]
	}

	req := &meilisearch.SearchRequest{
		Query: query,
		Limit: int64(topK),
	}
	if filterStr != "" {
		req.Filter = filterStr
	}

	resp, err := m.Client.Index(KbChunksIndex).Search(query, req)
	if err != nil {
		return nil, fmt.Errorf("Meilisearch BM25检索失败: %w", err)
	}

	var results []BM25SearchResult
	for i, hit := range resp.Hits {
		h, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}
		result := BM25SearchResult{
			Rank:  i + 1,
			Score: float64(len(results)+1) * -1, // BM25不使用原始分数，RRF只用排名
		}
		if v, ok := h["chunk_id"]; ok {
			result.ChunkID = toInt64(v)
		}
		if v, ok := h["kb_id"]; ok {
			result.KBID = toInt64(v)
		}
		if v, ok := h["doc_id"]; ok {
			result.DocID = toInt64(v)
		}
		if v, ok := h["content"]; ok {
			result.Content = toString(v)
		}
		if v, ok := h["chunk_index"]; ok {
			result.ChunkIndex = int(toInt64(v))
		}
		if v, ok := h["source"]; ok {
			result.Source = toString(v)
		}
		if v, ok := h["page_number"]; ok {
			result.PageNumber = int(toInt64(v))
		}
		results = append(results, result)
	}
	return results, nil
}

// DeleteByDocID 删除指定文档的所有分块
func (m *MeilisearchDao) DeleteByDocID(ctx context.Context, docID int64) error {
	task, err := m.Client.Index(KbChunksIndex).DeleteDocumentsByFilter(fmt.Sprintf("doc_id = %d", docID))
	if err != nil {
		return fmt.Errorf("从Meilisearch删除文档分块失败: %w", err)
	}
	if _, err := m.Client.WaitForTask(task.TaskUID, meilisearchWaitTimeout); err != nil {
		return fmt.Errorf("等待Meilisearch删除完成失败: %w", err)
	}
	return nil
}

func joinOr(strs []string) string {
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += " OR " + strs[i]
	}
	return result
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case json.Number:
		n, _ := val.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	default:
		return 0
	}
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
