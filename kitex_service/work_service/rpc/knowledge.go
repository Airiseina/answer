package rpc

import (
	"context"
	"fmt"
	"time"

	knowledge "github.com/Airiseina/answer/kitex_service/knowledge_service/kitex_gen/knowledge"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/kitex_gen/knowledge/knowledgeservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

var knowledgeCli knowledgeservice.Client

func ConnectKnowledgeService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(2)
	fp.WithFixedBackOff(200)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.5,
		MinSample: 30,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("knowledge_service", cbConfig)
	c, err := knowledgeservice.NewClient("knowledge_service",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(30*time.Second), // 知识库检索可能较慢，给30秒
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		klog.Fatalf("连接knowledge_service失败: %v", err)
	}
	knowledgeCli = c
}

// KnowledgeSearchResult 知识库检索结果
type KnowledgeSearchResult struct {
	Content    string
	Source     string
	DocID      int64
	KBID       int64
	ChunkIndex int
	PageNumber *int
	Score      float64
}

// SearchKnowledge 调用knowledge_service进行混合检索（向量+BM25+RRF+Rerank）
func SearchKnowledge(ctx context.Context, kbIDs []int64, query string, topK int32) ([]KnowledgeSearchResult, error) {
	resp, err := knowledgeCli.SearchKnowledge(ctx, &knowledge.SearchKnowledgeReq{
		KbIds: kbIDs,
		Query: query,
		TopK:  topK,
	})
	if err != nil {
		return nil, fmt.Errorf("知识库检索RPC失败: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("知识库检索失败")
	}
	results := make([]KnowledgeSearchResult, 0, len(resp.Chunks))
	for _, chunk := range resp.Chunks {
		r := KnowledgeSearchResult{
			Content:    chunk.Content,
			Source:     chunk.Source,
			DocID:      chunk.DocId,
			KBID:       chunk.KbId,
			ChunkIndex: int(chunk.ChunkIndex),
			Score:      chunk.Score,
		}
		if chunk.PageNumber != nil {
			pageNum := int(*chunk.PageNumber)
			r.PageNumber = &pageNum
		}
		results = append(results, r)
	}
	return results, nil
}

// KnowledgeBaseInfo 知识库信息
type KnowledgeBaseInfo struct {
	ID          int64
	Name        string
	Description string
	DocCount    int32
	ChunkCount  int32
}

// GetBotKnowledgeBases 获取Bot关联的知识库列表
func GetBotKnowledgeBases(ctx context.Context, botID int64) ([]KnowledgeBaseInfo, error) {
	resp, err := knowledgeCli.GetBotKnowledgeBases(ctx, &knowledge.GetBotKnowledgeBasesReq{
		BotId: botID,
	})
	if err != nil {
		return nil, fmt.Errorf("获取Bot知识库列表RPC失败: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("获取Bot知识库列表失败")
	}
	result := make([]KnowledgeBaseInfo, 0, len(resp.KnowledgeBases))
	for _, kb := range resp.KnowledgeBases {
		result = append(result, KnowledgeBaseInfo{
			ID:          kb.KbId,
			Name:        kb.Name,
			Description: kb.Description,
			DocCount:    kb.DocCount,
			ChunkCount:  kb.ChunkCount,
		})
	}
	return result, nil
}
