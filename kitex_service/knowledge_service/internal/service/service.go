package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/chunker"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/graph"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/parser"
	"github.com/Airiseina/answer/pkg/ai"
	"github.com/Airiseina/answer/pkg/infra/meilisearch"
	"github.com/Airiseina/answer/pkg/snowflake"
	"github.com/Airiseina/answer/pkg/storage"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/qdrant/go-client/qdrant"
	"github.com/segmentio/kafka-go"
)

type KnowledgeService struct {
	kbDao       dal.KnowledgeBaseDao
	docDao      dal.DocumentDao
	bkDao       dal.BotKnowledgeDao
	qdrant      *qdrant.Client
	meilisearch *meilisearch.MeilisearchDao
	neo4jGraph  *graph.Neo4jGraph
	snow        *snowflake.Node
	kafkaBroker string
	kafkaTopic  string
	rerank      *Reranker
}

func NewKnowledgeService(
	kbDao dal.KnowledgeBaseDao,
	docDao dal.DocumentDao,
	bkDao dal.BotKnowledgeDao,
	qdrantClient *qdrant.Client,
	meilisearchDao *meilisearch.MeilisearchDao,
	neo4jGraph *graph.Neo4jGraph,
	kafkaBroker string,
	kafkaTopic string,
	reranker *Reranker,
) *KnowledgeService {
	return &KnowledgeService{
		kbDao:       kbDao,
		docDao:      docDao,
		bkDao:       bkDao,
		qdrant:      qdrantClient,
		meilisearch: meilisearchDao,
		neo4jGraph:  neo4jGraph,
		snow:        snowflake.NewNode(6),
		kafkaBroker: kafkaBroker,
		kafkaTopic:  kafkaTopic,
		rerank:      reranker,
	}
}

func (svc *KnowledgeService) CreateKnowledgeBase(ctx context.Context, ownerID int64, name, description string) (int64, error) {
	kbID := svc.snow.Generate()
	kb := model.KnowledgeBase{
		ID:          kbID,
		OwnerID:     ownerID,
		Name:        name,
		Description: description,
	}
	if err := svc.kbDao.Create(kb); err != nil {
		return 0, err
	}
	return kbID, nil
}

func (svc *KnowledgeService) GetKnowledgeBase(kbID int64) (model.KnowledgeBase, error) {
	return svc.kbDao.GetByID(kbID)
}

func (svc *KnowledgeService) GetUserKnowledgeBases(ownerID int64) ([]model.KnowledgeBase, error) {
	return svc.kbDao.GetByOwner(ownerID)
}

func (svc *KnowledgeService) UpdateKnowledgeBase(kbID, operatorID int64, updates map[string]interface{}) (bool, error) {
	kb, err := svc.kbDao.GetByID(kbID)
	if err != nil {
		return false, err
	}
	if kb.ID == 0 {
		return false, nil
	}
	if kb.OwnerID != operatorID {
		return false, nil
	}
	return true, svc.kbDao.Update(kbID, updates)
}

func (svc *KnowledgeService) DeleteKnowledgeBase(ctx context.Context, kbID, operatorID int64) (bool, error) {
	kb, err := svc.kbDao.GetByID(kbID)
	if err != nil {
		return false, err
	}
	if kb.ID == 0 {
		return false, nil
	}
	if kb.OwnerID != operatorID {
		return false, nil
	}
	docs, err := svc.docDao.GetByKBID(kbID)
	if err != nil {
		return false, err
	}
	for _, doc := range docs {
		svc.deleteDocVectors(ctx, doc.ID)
	}
	if err := svc.docDao.DeleteByKBID(kbID); err != nil {
		return false, err
	}
	bks, err := svc.bkDao.GetByKBID(kbID)
	if err != nil {
		return false, err
	}
	for _, bk := range bks {
		_ = svc.bkDao.Delete(bk.BotID, bk.KBID)
	}
	return true, svc.kbDao.Delete(kbID)
}

func (svc *KnowledgeService) AddDocument(ctx context.Context, kbID, operatorID int64, fileName, fileURL, fileType string, fileSize int64) (int64, error) {
	kb, err := svc.kbDao.GetByID(kbID)
	if err != nil {
		return 0, err
	}
	if kb.ID == 0 {
		return 0, fmt.Errorf("知识库不存在")
	}
	if kb.OwnerID != operatorID {
		return 0, fmt.Errorf("无权操作此知识库")
	}
	docID := svc.snow.Generate()
	doc := model.KbDocument{
		ID:       docID,
		KBID:     kbID,
		FileName: fileName,
		FileURL:  fileURL,
		FileType: fileType,
		FileSize: fileSize,
		Status:   model.DocStatusPending,
	}
	if err := svc.docDao.Create(doc); err != nil {
		return 0, err
	}
	_ = svc.kbDao.IncrDocCount(kbID, 1)
	if err := svc.publishDocParse(docID, kbID); err != nil {
		klog.Warnf("文档[%d]解析消息发送失败(将等待轮询): %v", docID, err)
	}
	return docID, nil
}

func (svc *KnowledgeService) GetDocuments(kbID int64) ([]model.KbDocument, error) {
	return svc.docDao.GetByKBID(kbID)
}

func (svc *KnowledgeService) DeleteDocument(ctx context.Context, docID, operatorID int64) (bool, error) {
	doc, err := svc.docDao.GetByID(docID)
	if err != nil {
		return false, err
	}
	if doc.ID == 0 {
		return false, nil
	}
	kb, err := svc.kbDao.GetByID(doc.KBID)
	if err != nil {
		return false, err
	}
	if kb.OwnerID != operatorID {
		return false, nil
	}
	svc.deleteDocVectors(ctx, docID)
	if err := svc.docDao.DeleteByID(docID); err != nil {
		return false, err
	}
	_ = svc.kbDao.IncrDocCount(doc.KBID, -1)
	_ = svc.kbDao.IncrChunkCount(doc.KBID, -int(doc.ChunkCount))
	return true, nil
}

func (svc *KnowledgeService) RetryDocument(ctx context.Context, docID, operatorID int64) (bool, error) {
	doc, err := svc.docDao.GetByID(docID)
	if err != nil {
		return false, err
	}
	if doc.ID == 0 {
		return false, nil
	}
	if doc.Status != model.DocStatusFailed {
		return false, nil
	}
	kb, err := svc.kbDao.GetByID(doc.KBID)
	if err != nil {
		return false, err
	}
	if kb.OwnerID != operatorID {
		return false, nil
	}
	return true, svc.docDao.UpdateStatus(docID, model.DocStatusPending, int(doc.ChunkCount), "")
}

func (svc *KnowledgeService) BindKnowledgeBase(botID, operatorID, kbID int64) (bool, error) {
	kb, err := svc.kbDao.GetByID(kbID)
	if err != nil {
		return false, err
	}
	if kb.ID == 0 {
		return false, fmt.Errorf("知识库不存在")
	}
	if kb.OwnerID != operatorID {
		return false, fmt.Errorf("无权操作此知识库")
	}
	bkID := svc.snow.Generate()
	bk := model.BotKnowledge{
		ID:    bkID,
		BotID: botID,
		KBID:  kbID,
	}
	return true, svc.bkDao.Create(bk)
}

func (svc *KnowledgeService) UnbindKnowledgeBase(botID, operatorID, kbID int64) (bool, error) {
	kb, err := svc.kbDao.GetByID(kbID)
	if err != nil {
		return false, err
	}
	if kb.ID == 0 {
		return false, fmt.Errorf("知识库不存在")
	}
	if kb.OwnerID != operatorID {
		return false, fmt.Errorf("无权操作此知识库")
	}
	return true, svc.bkDao.Delete(botID, kbID)
}

func (svc *KnowledgeService) GetBotKnowledgeBases(botID int64) ([]model.KnowledgeBase, error) {
	return svc.bkDao.GetKnowledgeBasesByBotID(botID)
}

func (svc *KnowledgeService) BindSystemKnowledgeBase(botID, kbID int64) (bool, error) {
	kb, err := svc.kbDao.GetByID(kbID)
	if err != nil {
		return false, err
	}
	if kb.ID == 0 {
		return false, fmt.Errorf("知识库不存在")
	}
	bkID := svc.snow.Generate()
	bk := model.BotKnowledge{
		ID:    bkID,
		BotID: botID,
		KBID:  kbID,
	}
	return true, svc.bkDao.Create(bk)
}

func (svc *KnowledgeService) AddSystemDocument(ctx context.Context, kbID int64, fileName, fileURL, fileType string, fileSize int64) (int64, error) {
	kb, err := svc.kbDao.GetByID(kbID)
	if err != nil {
		return 0, err
	}
	if kb.ID == 0 {
		return 0, fmt.Errorf("知识库不存在")
	}
	docID := svc.snow.Generate()
	doc := model.KbDocument{
		ID:       docID,
		KBID:     kbID,
		FileName: fileName,
		FileURL:  fileURL,
		FileType: fileType,
		FileSize: fileSize,
		Status:   model.DocStatusPending,
	}
	if err := svc.docDao.Create(doc); err != nil {
		return 0, err
	}
	_ = svc.kbDao.IncrDocCount(kbID, 1)
	if err := svc.publishDocParse(docID, kbID); err != nil {
		klog.Warnf("系统文档[%d]解析消息发送失败(将等待轮询): %v", docID, err)
	}
	return docID, nil
}

func (svc *KnowledgeService) ProcessDocument(ctx context.Context, docID int64) error {
	doc, err := svc.docDao.GetByID(docID)
	if err != nil {
		return err
	}
	if doc.ID == 0 {
		return fmt.Errorf("文档[%d]不存在", docID)
	}
	if doc.Status == model.DocStatusParsing || doc.Status == model.DocStatusParsed {
		return nil
	}
	_ = svc.docDao.UpdateStatus(docID, model.DocStatusParsing, 0, "")
	localPath, err := svc.downloadDocument(ctx, doc)
	if err != nil {
		_ = svc.docDao.UpdateStatus(docID, model.DocStatusFailed, 0, err.Error())
		return err
	}
	defer func() { _ = os.Remove(localPath) }()
	p := parser.GetParser(doc.FileType)
	if p == nil {
		_ = svc.docDao.UpdateStatus(docID, model.DocStatusFailed, 0, fmt.Sprintf("不支持的文件类型: %s", doc.FileType))
		return fmt.Errorf("不支持的文件类型: %s", doc.FileType)
	}
	parsed, err := p.Parse(localPath)
	if err != nil {
		_ = svc.docDao.UpdateStatus(docID, model.DocStatusFailed, 0, err.Error())
		return err
	}
	chunks := svc.chunkDocument(parsed, doc)
	if len(chunks) == 0 {
		_ = svc.docDao.UpdateStatus(docID, model.DocStatusFailed, 0, "文档解析后内容为空")
		return fmt.Errorf("文档解析后内容为空")
	}
	texts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		texts = append(texts, c.Content)
	}
	vectors, err := ai.GetEmbeddings(ctx, texts)
	if err != nil {
		_ = svc.docDao.UpdateStatus(docID, model.DocStatusFailed, 0, fmt.Sprintf("向量化失败: %v", err))
		return err
	}
	if err := svc.storeVectors(ctx, doc, chunks, vectors); err != nil {
		_ = svc.docDao.UpdateStatus(docID, model.DocStatusFailed, 0, fmt.Sprintf("向量存储失败: %v", err))
		return err
	}
	_ = svc.docDao.UpdateStatus(docID, model.DocStatusParsed, len(chunks), "")
	_ = svc.kbDao.IncrChunkCount(doc.KBID, len(chunks))
	klog.Infof("文档[%d]解析完成，生成%d个分块", docID, len(chunks))
	return nil
}

func (svc *KnowledgeService) SearchKnowledge(ctx context.Context, kbIDs []int64, query string, topK int) ([]SearchResult, error) {
	// 三路混合检索：向量检索 + BM25关键词检索 + 图谱检索
	// 优先级：三路 > 两路(向量+BM25) > 纯向量
	if svc.meilisearch != nil && svc.neo4jGraph != nil {
		return svc.hybridSearch(ctx, kbIDs, query, topK)
	}
	if svc.meilisearch != nil {
		return svc.hybridSearchTwoWay(ctx, kbIDs, query, topK)
	}
	// 降级：仅使用向量检索
	return svc.vectorSearch(ctx, kbIDs, query, topK)
}

// hybridSearch 三路混合检索：向量检索 + BM25关键词检索 + 图谱检索，使用RRF融合排序
func (svc *KnowledgeService) hybridSearch(ctx context.Context, kbIDs []int64, query string, topK int) ([]SearchResult, error) {
	klog.Infof("混合检索开始: kb_ids=%v, top_k=%d, 图谱=%v", kbIDs, topK, svc.neo4jGraph != nil)

	// 并行执行三路检索
	vectorResults, vectorErr := svc.vectorSearch(ctx, kbIDs, query, topK)
	bm25Results, bm25Err := svc.meilisearch.SearchBM25(ctx, kbIDs, query, topK)
	graphResults, graphErr := svc.graphSearch(ctx, kbIDs, query, topK)

	if vectorErr != nil && bm25Err != nil && graphErr != nil {
		return nil, fmt.Errorf("三路混合检索全部失败: 向量检索=%v, BM25检索=%v, 图谱检索=%v", vectorErr, bm25Err, graphErr)
	}

	// 降级处理：某路失败时使用可用结果
	if vectorErr != nil {
		klog.Warnf("向量检索失败: %v", vectorErr)
	} else {
		klog.Infof("向量检索返回%d条结果", len(vectorResults))
	}
	if bm25Err != nil {
		klog.Warnf("BM25检索失败: %v", bm25Err)
	} else {
		klog.Infof("BM25检索返回%d条结果", len(bm25Results))
	}
	if graphErr != nil {
		klog.Warnf("图谱检索失败: %v", graphErr)
	} else {
		klog.Infof("图谱检索返回%d条结果", len(graphResults))
	}

	// RRF三路融合排序
	rrfResults := rrfFusionThree(vectorResults, bm25Results, graphResults, topK)
	klog.Infof("RRF融合后%d条结果", len(rrfResults))

	// Rerank精排：对RRF粗排结果进行二次精排，只保留最相关的Top-N
	if svc.rerank != nil && svc.rerank.Enabled() {
		reranked, rerankErr := svc.rerank.Rerank(ctx, query, rrfResults)
		if rerankErr != nil {
			klog.Warnf("Rerank精排失败，降级使用RRF结果: %v", rerankErr)
			return rrfResults, nil
		}
		return reranked, nil
	}

	return rrfResults, nil
}

// hybridSearchTwoWay 两路混合检索：向量检索 + BM25关键词检索（无图谱时降级）
func (svc *KnowledgeService) hybridSearchTwoWay(ctx context.Context, kbIDs []int64, query string, topK int) ([]SearchResult, error) {
	vectorResults, vectorErr := svc.vectorSearch(ctx, kbIDs, query, topK)
	bm25Results, bm25Err := svc.meilisearch.SearchBM25(ctx, kbIDs, query, topK)

	if vectorErr != nil && bm25Err != nil {
		return nil, fmt.Errorf("混合检索全部失败: 向量检索=%v, BM25检索=%v", vectorErr, bm25Err)
	}
	if vectorErr != nil {
		klog.Warnf("向量检索失败，仅使用BM25结果: %v", vectorErr)
		results := make([]SearchResult, 0, len(bm25Results))
		for _, r := range bm25Results {
			result := SearchResult{
				Content:    r.Content,
				Source:     r.Source,
				DocID:      r.DocID,
				KBID:       r.KBID,
				ChunkIndex: r.ChunkIndex,
				Score:      r.Score,
			}
			if r.PageNumber > 0 {
				pageNum := r.PageNumber
				result.PageNumber = &pageNum
			}
			results = append(results, result)
		}
		return results, nil
	}
	if bm25Err != nil {
		klog.Warnf("BM25检索失败，仅使用向量检索结果: %v", bm25Err)
		return vectorResults, nil
	}

	rrfResults := rrfFusion(vectorResults, bm25Results, topK)

	if svc.rerank != nil && svc.rerank.Enabled() {
		reranked, rerankErr := svc.rerank.Rerank(ctx, query, rrfResults)
		if rerankErr != nil {
			klog.Warnf("Rerank精排失败，降级使用RRF结果: %v", rerankErr)
			return rrfResults, nil
		}
		return reranked, nil
	}

	return rrfResults, nil
}

// graphSearch 图谱检索：通过关键词在Neo4j中检索相关Chunk
func (svc *KnowledgeService) graphSearch(ctx context.Context, kbIDs []int64, query string, topK int) ([]SearchResult, error) {
	if svc.neo4jGraph == nil {
		klog.Debugf("图谱未初始化，跳过图谱检索")
		return nil, fmt.Errorf("图谱未初始化")
	}

	// 将查询分词为关键词
	keywords := extractKeywords(query)
	if len(keywords) == 0 {
		klog.Debugf("查询无有效关键词，跳过图谱检索: query=%q", query)
		return nil, nil
	}
	klog.Infof("图谱检索: keywords=%v, kb_ids=%v, top_k=%d", keywords, kbIDs, topK)

	graphResults, err := svc.neo4jGraph.SearchByKeywords(ctx, kbIDs, keywords, topK)
	if err != nil {
		return nil, fmt.Errorf("图谱检索失败: %w", err)
	}

	klog.Infof("图谱检索完成: 返回%d条结果", len(graphResults))

	results := make([]SearchResult, 0, len(graphResults))
	for _, r := range graphResults {
		results = append(results, SearchResult{
			Content:    r.Content,
			Source:     r.Source,
			DocID:      r.DocID,
			KBID:       r.KBID,
			ChunkIndex: r.ChunkIndex,
			Score:      r.Score,
		})
	}
	return results, nil
}

// indexEntitiesFromChunks 从Chunk内容中提取实体并索引到Neo4j图谱
// 基于规则的实体提取：提取高频N-gram作为实体，同一文档内共享N-gram的Chunk建立关联
func (svc *KnowledgeService) indexEntitiesFromChunks(ctx context.Context, doc model.KbDocument, chunks []graph.ChunkData) {
	if svc.neo4jGraph == nil {
		return
	}

	// 统计文档内各N-gram出现的Chunk列表
	keywordChunks := make(map[string][]string) // keyword -> []chunkID
	for _, chunk := range chunks {
		keywords := extractNgrams(chunk.Content)
		seen := make(map[string]bool)
		for _, kw := range keywords {
			if !seen[kw] {
				seen[kw] = true
				keywordChunks[kw] = append(keywordChunks[kw], chunk.ChunkID)
			}
		}
	}

	// 只保留出现2次以上的N-gram作为实体（高频词更有可能是实体）
	var entities []graph.EntityData
	var relations []graph.RelationData
	entityNames := make(map[string]bool)

	for kw, chunkIDs := range keywordChunks {
		if len(chunkIDs) < 3 {
			continue
		}
		entityNames[kw] = true
		entities = append(entities, graph.EntityData{
			Name:        kw,
			Type:        "Keyword",
			Description: fmt.Sprintf("文档[%d]中的高频关键词", doc.ID),
			ChunkIDs:    chunkIDs,
		})
	}

	// 在同一文档内，共享3个以上Chunk的实体之间建立关联
	entityList := make([]string, 0, len(entityNames))
	for name := range entityNames {
		entityList = append(entityList, name)
	}
	for i := 0; i < len(entityList); i++ {
		for j := i + 1; j < len(entityList); j++ {
			e1, e2 := entityList[i], entityList[j]
			overlapCount := overlapSize(keywordChunks[e1], keywordChunks[e2])
			if overlapCount >= 3 {
				relations = append(relations, graph.RelationData{
					SourceEntity: e1,
					TargetEntity: e2,
					RelationType: "CO_OCCURS",
				})
			}
		}
	}

	if len(entities) == 0 {
		klog.Infof("文档[%d]未提取到有效实体", doc.ID)
		return
	}

	klog.Infof("文档[%d]提取到%d个实体, %d个关系", doc.ID, len(entities), len(relations))
	if err := svc.neo4jGraph.IndexEntities(ctx, entities, relations); err != nil {
		klog.Warnf("文档[%d]实体索引失败(不影响Chunk索引): %v", doc.ID, err)
	}
}

// extractNgrams 从文本中提取N-gram关键词
// 对中文使用2-4字的N-gram，对英文使用空格分词
func extractNgrams(text string) []string {
	var keywords []string

	// 先按标点分句
	separators := []string{"，", "。", "、", "？", "！", "；", "：", "\"", "'", "\n", "\t",
		",", ".", "?", "!", ";", ":", "(", ")", "（", "）", "【", "】", "[", "]", " ",
		"#", "*", "_", "-", "=", "|", ">", "<", "/", "\\", "~", "`", "^", "&", "%", "$", "@"}
	sentences := []string{text}
	for _, sep := range separators {
		var newSentences []string
		for _, s := range sentences {
			parts := strings.Split(s, sep)
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					newSentences = append(newSentences, p)
				}
			}
		}
		sentences = newSentences
	}

	// 停用词
	stopWords := map[string]bool{
		"的": true, "了": true, "在": true, "是": true, "我": true, "有": true, "和": true,
		"就": true, "不": true, "人": true, "都": true, "一": true, "一个": true, "上": true,
		"也": true, "很": true, "到": true, "说": true, "要": true, "去": true, "你": true,
		"会": true, "着": true, "没有": true, "看": true, "好": true, "自己": true, "这": true,
		"他": true, "她": true, "它": true, "们": true, "那": true, "个": true, "下": true,
		"里": true, "中": true, "来": true, "得": true, "地": true, "把": true, "被": true,
		"从": true, "而": true, "且": true, "但": true, "与": true, "或": true, "以": true,
		"为": true, "于": true, "及": true, "等": true, "之": true, "其": true, "所": true,
		"the": true, "a": true, "an": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true, "could": true,
		"should": true, "may": true, "might": true, "can": true, "shall": true,
		"what": true, "how": true, "why": true, "when": true, "where": true, "who": true,
		"which": true, "that": true, "this": true, "it": true, "of": true, "for": true,
		"in": true, "on": true, "at": true, "to": true, "from": true, "with": true,
		"and": true, "or": true, "but": true, "not": true, "if": true, "then": true,
	}

	seen := make(map[string]bool)
	for _, sentence := range sentences {
		runes := []rune(sentence)
		length := len(runes)

		// 提取2-gram、3-gram、4-gram
		for n := 2; n <= 4; n++ {
			for i := 0; i <= length-n; i++ {
				ngram := string(runes[i : i+n])
				// 过滤：纯停用词组合
				if isStopwordNgram(ngram, stopWords) {
					continue
				}
				// 过滤：纯数字
				if isPureDigit(ngram) {
					continue
				}
				// 过滤：包含特殊字符(#、*、-、=、|等)
				if hasSpecialChar(ngram) {
					continue
				}
				// 过滤：2-gram中任一字是停用词（2-gram太短，要求两个字都有实质含义）
				if n == 2 {
					runes2 := []rune(ngram)
					if stopWords[string(runes2[0])] || stopWords[string(runes2[1])] {
						continue
					}
				}
				if !seen[ngram] {
					seen[ngram] = true
					keywords = append(keywords, ngram)
				}
			}
		}
	}

	return keywords
}

// isPureDigit 检查字符串是否全为数字
func isPureDigit(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// hasSpecialChar 检查是否包含特殊字符
func hasSpecialChar(s string) bool {
	specialChars := "#*-=|<>{}[]()_+/\\~`@#$%^&"
	for _, r := range s {
		if strings.ContainsRune(specialChars, r) {
			return true
		}
	}
	return false
}

// isStopwordNgram 检查N-gram是否全部由停用词组成
func isStopwordNgram(ngram string, stopWords map[string]bool) bool {
	runes := []rune(ngram)
	allStop := true
	for _, r := range runes {
		if !stopWords[string(r)] {
			allStop = false
			break
		}
	}
	return allStop
}

// overlapSize 计算两个字符串切片的交集大小
func overlapSize(a, b []string) int {
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	count := 0
	for _, s := range b {
		if set[s] {
			count++
		}
	}
	return count
}

// extractKeywords 从查询文本中提取关键词
// 简单实现：按空格和标点分词，过滤停用词
func extractKeywords(query string) []string {
	// 按空格、标点分词
	separators := []string{" ", "，", "。", "、", "？", "！", "；", "：", "\"", "'", "\n", "\t", ",", ".", "?", "!", ";", ":", "(", ")", "（", "）", "【", "】", "[", "]"}
	tokens := []string{query}
	for _, sep := range separators {
		var newTokens []string
		for _, t := range tokens {
			parts := strings.Split(t, sep)
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					newTokens = append(newTokens, p)
				}
			}
		}
		tokens = newTokens
	}

	// 过滤停用词
	stopWords := map[string]bool{
		"的": true, "了": true, "在": true, "是": true, "我": true, "有": true, "和": true,
		"就": true, "不": true, "人": true, "都": true, "一": true, "一个": true, "上": true,
		"也": true, "很": true, "到": true, "说": true, "要": true, "去": true, "你": true,
		"会": true, "着": true, "没有": true, "看": true, "好": true, "自己": true, "这": true,
		"the": true, "a": true, "an": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true, "could": true,
		"should": true, "may": true, "might": true, "can": true, "shall": true,
		"what": true, "how": true, "why": true, "when": true, "where": true, "who": true,
		"which": true, "that": true, "this": true, "it": true, "of": true, "for": true,
		"in": true, "on": true, "at": true, "to": true, "from": true, "with": true,
		"and": true, "or": true, "but": true, "not": true, "if": true, "then": true,
	}

	var keywords []string
	for _, t := range tokens {
		if !stopWords[strings.ToLower(t)] && len(t) > 0 {
			keywords = append(keywords, t)
		}
	}
	return keywords
}

// vectorSearch 纯向量检索（Qdrant）
func (svc *KnowledgeService) vectorSearch(ctx context.Context, kbIDs []int64, query string, topK int) ([]SearchResult, error) {
	queryVector, err := ai.GetEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询向量化失败: %w", err)
	}
	kbIDValues := make([]*qdrant.Condition, 0, len(kbIDs))
	for _, id := range kbIDs {
		kbIDValues = append(kbIDValues, qdrant.NewMatch("kb_id", fmt.Sprintf("%d", id)))
	}
	filter := &qdrant.Filter{
		Should: kbIDValues,
	}
	searchResult, err := svc.qdrant.Query(ctx, &qdrant.QueryPoints{
		CollectionName: KbChunksCollection,
		Query:          qdrant.NewQuery(queryVector...),
		Filter:         filter,
		Limit:          qdrant.PtrOf(uint64(topK)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}
	var results []SearchResult
	for _, point := range searchResult {
		result := SearchResult{Score: float64(point.Score)}
		if payload := point.Payload; payload != nil {
			if v, ok := payload["content"]; ok {
				result.Content = getStringValue(v)
			}
			if v, ok := payload["source"]; ok {
				result.Source = getStringValue(v)
			}
			if v, ok := payload["doc_id"]; ok {
				result.DocID = getInt64Value(v)
			}
			if v, ok := payload["kb_id"]; ok {
				result.KBID = getInt64Value(v)
			}
			if v, ok := payload["chunk_index"]; ok {
				result.ChunkIndex = int(getInt64Value(v))
			}
			if v, ok := payload["page_number"]; ok {
				pageNum := int(getInt64Value(v))
				result.PageNumber = &pageNum
			}
		}
		results = append(results, result)
	}
	return results, nil
}

// rrfFusion RRF（Reciprocal Rank Fusion）融合排序
// 算法：对于每个文档，RRF得分 = Σ(1 / (k + rank_i))，其中k=60是平滑常数
// 向量检索和BM25检索各贡献一个排名，最终按RRF得分降序排列
func rrfFusion(vectorResults []SearchResult, bm25Results []meilisearch.BM25SearchResult, topK int) []SearchResult {
	const k = 60 // RRF平滑常数，标准值60

	// 使用 chunk 唯一标识：(doc_id, chunk_index) 作为去重键
	type chunkKey struct {
		DocID      int64
		ChunkIndex int
	}

	type fusedResult struct {
		SearchResult
		rrfScore float64
	}

	fusedMap := make(map[chunkKey]*fusedResult)

	// 向量检索结果贡献排名
	for rank, r := range vectorResults {
		key := chunkKey{DocID: r.DocID, ChunkIndex: r.ChunkIndex}
		if _, exists := fusedMap[key]; !exists {
			fusedMap[key] = &fusedResult{
				SearchResult: r,
				rrfScore:     0,
			}
		}
		fusedMap[key].rrfScore += 1.0 / float64(k+rank+1)
	}

	// BM25检索结果贡献排名
	for rank, r := range bm25Results {
		key := chunkKey{DocID: r.DocID, ChunkIndex: r.ChunkIndex}
		if _, exists := fusedMap[key]; !exists {
			sr := SearchResult{
				Content:    r.Content,
				Source:     r.Source,
				DocID:      r.DocID,
				KBID:       r.KBID,
				ChunkIndex: r.ChunkIndex,
			}
			if r.PageNumber > 0 {
				pageNum := r.PageNumber
				sr.PageNumber = &pageNum
			}
			fusedMap[key] = &fusedResult{
				SearchResult: sr,
				rrfScore:     0,
			}
		}
		fusedMap[key].rrfScore += 1.0 / float64(k+rank+1)
	}

	// 收集并按RRF得分降序排列
	allResults := make([]*fusedResult, 0, len(fusedMap))
	for _, fr := range fusedMap {
		fr.Score = fr.rrfScore
		allResults = append(allResults, fr)
	}
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].rrfScore > allResults[j].rrfScore
	})

	// 截取topK
	if len(allResults) > topK {
		allResults = allResults[:topK]
	}

	results := make([]SearchResult, 0, len(allResults))
	for _, fr := range allResults {
		results = append(results, fr.SearchResult)
	}
	return results
}

// rrfFusionThree 三路RRF融合排序：向量检索 + BM25检索 + 图谱检索
func rrfFusionThree(vectorResults []SearchResult, bm25Results []meilisearch.BM25SearchResult, graphResults []SearchResult, topK int) []SearchResult {
	const k = 60 // RRF平滑常数

	type chunkKey struct {
		DocID      int64
		ChunkIndex int
	}

	type fusedResult struct {
		SearchResult
		rrfScore float64
	}

	fusedMap := make(map[chunkKey]*fusedResult)

	// 向量检索结果贡献排名
	for rank, r := range vectorResults {
		key := chunkKey{DocID: r.DocID, ChunkIndex: r.ChunkIndex}
		if _, exists := fusedMap[key]; !exists {
			fusedMap[key] = &fusedResult{
				SearchResult: r,
				rrfScore:     0,
			}
		}
		fusedMap[key].rrfScore += 1.0 / float64(k+rank+1)
	}

	// BM25检索结果贡献排名
	for rank, r := range bm25Results {
		key := chunkKey{DocID: r.DocID, ChunkIndex: r.ChunkIndex}
		if _, exists := fusedMap[key]; !exists {
			sr := SearchResult{
				Content:    r.Content,
				Source:     r.Source,
				DocID:      r.DocID,
				KBID:       r.KBID,
				ChunkIndex: r.ChunkIndex,
			}
			if r.PageNumber > 0 {
				pageNum := r.PageNumber
				sr.PageNumber = &pageNum
			}
			fusedMap[key] = &fusedResult{
				SearchResult: sr,
				rrfScore:     0,
			}
		}
		fusedMap[key].rrfScore += 1.0 / float64(k+rank+1)
	}

	// 图谱检索结果贡献排名
	for rank, r := range graphResults {
		key := chunkKey{DocID: r.DocID, ChunkIndex: r.ChunkIndex}
		if _, exists := fusedMap[key]; !exists {
			fusedMap[key] = &fusedResult{
				SearchResult: r,
				rrfScore:     0,
			}
		}
		fusedMap[key].rrfScore += 1.0 / float64(k+rank+1)
	}

	// 收集并按RRF得分降序排列
	allResults := make([]*fusedResult, 0, len(fusedMap))
	for _, fr := range fusedMap {
		fr.Score = fr.rrfScore
		allResults = append(allResults, fr)
	}
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].rrfScore > allResults[j].rrfScore
	})

	// 截取topK
	if len(allResults) > topK {
		allResults = allResults[:topK]
	}

	results := make([]SearchResult, 0, len(allResults))
	for _, fr := range allResults {
		results = append(results, fr.SearchResult)
	}
	return results
}

type SearchResult struct {
	Content    string  `json:"content"`
	Source     string  `json:"source"`
	DocID      int64   `json:"doc_id"`
	KBID       int64   `json:"kb_id"`
	ChunkIndex int     `json:"chunk_index"`
	PageNumber *int    `json:"page_number,omitempty"`
	Score      float64 `json:"score"`
}

const KbChunksCollection = "kb_chunks"

func (svc *KnowledgeService) chunkDocument(parsed *parser.ParsedDocument, doc model.KbDocument) []chunker.Chunk {
	opts := chunker.ChunkOptions{
		ChunkSize:    chunker.DefaultChunkSize,
		ChunkOverlap: chunker.DefaultChunkOverlap,
		Source:       doc.FileName,
	}
	if len(parsed.Sections) > 0 {
		hasHeadings := false
		for _, sec := range parsed.Sections {
			if sec.Level > 0 {
				hasHeadings = true
				break
			}
		}
		if hasHeadings {
			structuralChunker := chunker.NewStructuralChunker()
			return structuralChunker.ChunkFromSections(parsed.Sections, opts)
		}
	}
	recursiveChunker := chunker.NewRecursiveChunker()
	return recursiveChunker.Chunk(parsed.Content, opts)
}

func (svc *KnowledgeService) downloadDocument(ctx context.Context, doc model.KbDocument) (string, error) {
	fileURL := doc.FileURL
	fileURL = strings.TrimPrefix(fileURL, storage.PublicURL)
	fileURL = strings.TrimPrefix(fileURL, storage.BasePath)
	fileURL = strings.TrimPrefix(fileURL, "/")
	data, err := storage.Client.GetObject(ctx, fileURL)
	if err != nil {
		return "", fmt.Errorf("下载文档失败: %w", err)
	}
	ext := filepath.Ext(doc.FileName)
	tmpFile, err := os.CreateTemp("", "kb-doc-*"+ext)
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("写入临时文件失败: %w", err)
	}
	_ = tmpFile.Close()
	return tmpFile.Name(), nil
}

func (svc *KnowledgeService) storeVectors(ctx context.Context, doc model.KbDocument, chunks []chunker.Chunk, vectors [][]float32) error {
	svc.ensureCollection(ctx)
	points := make([]*qdrant.PointStruct, 0, len(chunks))
	meiliDocs := make([]meilisearch.ChunkDocument, 0, len(chunks))
	for i, c := range chunks {
		chunkID := svc.snow.Generate()
		payload := map[string]interface{}{
			"chunk_id":    chunkID,
			"kb_id":       doc.KBID,
			"doc_id":      doc.ID,
			"content":     c.Content,
			"chunk_index": c.Index,
			"source":      c.Source,
		}
		if c.PageNumber > 0 {
			payload["page_number"] = c.PageNumber
		}
		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewIDNum(uint64(chunkID)),
			Vectors: qdrant.NewVectors(vectors[i]...),
			Payload: qdrant.NewValueMap(payload),
		})
		meiliDoc := meilisearch.ChunkDocument{
			ChunkIDStr: fmt.Sprintf("%d", chunkID),
			ChunkID:    chunkID,
			KBID:       doc.KBID,
			DocID:      doc.ID,
			Content:    c.Content,
			ChunkIndex: c.Index,
			Source:     c.Source,
		}
		if c.PageNumber > 0 {
			meiliDoc.PageNumber = c.PageNumber
		}
		meiliDocs = append(meiliDocs, meiliDoc)
	}
	_, err := svc.qdrant.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: KbChunksCollection,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("向量存储失败: %w", err)
	}
	// 同步索引到Meilisearch
	if svc.meilisearch != nil {
		if indexErr := svc.meilisearch.IndexChunks(ctx, meiliDocs); indexErr != nil {
			klog.Warnf("文档[%d]Meilisearch索引失败(不影响向量存储): %v", doc.ID, indexErr)
		}
	}
	// 同步索引到Neo4j知识图谱
	if svc.neo4jGraph != nil {
		chunkData := make([]graph.ChunkData, 0, len(chunks))
		for i, c := range chunks {
			chunkData = append(chunkData, graph.ChunkData{
				ChunkID:    fmt.Sprintf("%d", meiliDocs[i].ChunkID),
				KBID:       doc.KBID,
				DocID:      doc.ID,
				Content:    c.Content,
				ChunkIndex: c.Index,
				Source:     c.Source,
			})
		}
		if graphErr := svc.neo4jGraph.IndexChunks(ctx, chunkData); graphErr != nil {
			klog.Warnf("文档[%d]Neo4j图谱索引失败(不影响向量存储): %v", doc.ID, graphErr)
		} else {
			// 从Chunk内容中提取实体并索引到图谱
			svc.indexEntitiesFromChunks(ctx, doc, chunkData)
		}
	}
	return nil
}

func (svc *KnowledgeService) ensureCollection(ctx context.Context) {
	exists, err := svc.qdrant.CollectionExists(ctx, KbChunksCollection)
	if err != nil {
		klog.Errorf("检查知识库Collection失败: %v", err)
		return
	}
	if !exists {
		err = svc.qdrant.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: KbChunksCollection,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     2048,
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			klog.Errorf("创建知识库Collection失败: %v", err)
		}
	}
}

// BackfillEntities 回填已有Chunk的实体到Neo4j图谱
// 用于在服务启动时为历史数据补充实体提取
func (svc *KnowledgeService) BackfillEntities(ctx context.Context) {
	if svc.neo4jGraph == nil {
		return
	}

	// 检查是否已有实体，如果已有则跳过
	cnt, err := svc.neo4jGraph.GetEntityCount(ctx)
	if err != nil {
		klog.Warnf("检查实体数量失败: %v", err)
		return
	}
	if cnt > 0 {
		klog.Infof("图谱已有%d个实体，跳过回填", cnt)
		return
	}

	// 获取所有按文档分组的Chunk
	docChunks, err := svc.neo4jGraph.GetAllChunksByDoc(ctx)
	if err != nil {
		klog.Warnf("获取已有Chunk失败: %v", err)
		return
	}

	if len(docChunks) == 0 {
		klog.Info("图谱中无Chunk数据，尝试重新触发文档解析以重建图谱")
		svc.reparseAllDocs(ctx)
		return
	}

	klog.Infof("开始回填实体: 共%d个文档需要处理", len(docChunks))
	totalEntities := 0
	totalRelations := 0

	for docID, chunks := range docChunks {
		// 统计文档内各N-gram出现的Chunk列表
		keywordChunks := make(map[string][]string)
		for _, chunk := range chunks {
			keywords := extractNgrams(chunk.Content)
			seen := make(map[string]bool)
			for _, kw := range keywords {
				if !seen[kw] {
					seen[kw] = true
					keywordChunks[kw] = append(keywordChunks[kw], chunk.ChunkID)
				}
			}
		}

		var entities []graph.EntityData
		var relations []graph.RelationData
		entityNames := make(map[string]bool)

		for kw, chunkIDs := range keywordChunks {
			if len(chunkIDs) < 3 {
				continue
			}
			entityNames[kw] = true
			entities = append(entities, graph.EntityData{
				Name:        kw,
				Type:        "Keyword",
				Description: fmt.Sprintf("文档[%d]中的高频关键词", docID),
				ChunkIDs:    chunkIDs,
			})
		}

		entityList := make([]string, 0, len(entityNames))
		for name := range entityNames {
			entityList = append(entityList, name)
		}
		for i := 0; i < len(entityList); i++ {
			for j := i + 1; j < len(entityList); j++ {
				e1, e2 := entityList[i], entityList[j]
				if overlapSize(keywordChunks[e1], keywordChunks[e2]) >= 3 {
					relations = append(relations, graph.RelationData{
						SourceEntity: e1,
						TargetEntity: e2,
						RelationType: "CO_OCCURS",
					})
				}
			}
		}

		if len(entities) > 0 {
			if err := svc.neo4jGraph.IndexEntities(ctx, entities, relations); err != nil {
				klog.Warnf("文档[%d]实体回填失败: %v", docID, err)
			} else {
				totalEntities += len(entities)
				totalRelations += len(relations)
				klog.Infof("文档[%d]回填%d个实体, %d个关系", docID, len(entities), len(relations))
			}
		}
	}

	klog.Infof("实体回填完成: 共回填%d个实体, %d个关系", totalEntities, totalRelations)
}

// reparseAllDocs 重新触发所有已解析文档的解析，以重建Neo4j图谱数据
func (svc *KnowledgeService) reparseAllDocs(ctx context.Context) {
	docs, err := svc.docDao.GetParsedDocuments()
	if err != nil {
		klog.Warnf("获取已解析文档失败: %v", err)
		return
	}
	if len(docs) == 0 {
		klog.Info("无已解析文档可重新解析")
		return
	}

	klog.Infof("开始重新解析%d个文档以重建图谱", len(docs))
	for _, doc := range docs {
		_ = svc.docDao.UpdateStatus(doc.ID, model.DocStatusPending, 0, "")
		_ = svc.RepublishDocParse(doc.ID, doc.KBID)
	}
	klog.Infof("已重新发布%d个文档的解析任务", len(docs))
}

func (svc *KnowledgeService) deleteDocVectors(ctx context.Context, docID int64) {
	_, err := svc.qdrant.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: KbChunksCollection,
		Points: qdrant.NewPointsSelectorFilter(&qdrant.Filter{
			Must: []*qdrant.Condition{qdrant.NewMatch("doc_id", fmt.Sprintf("%d", docID))},
		}),
	})
	if err != nil {
		klog.Errorf("删除文档[%d]向量失败: %v", docID, err)
	}
	// 同步从Meilisearch删除
	if svc.meilisearch != nil {
		if delErr := svc.meilisearch.DeleteByDocID(ctx, docID); delErr != nil {
			klog.Warnf("删除文档[%d]Meilisearch索引失败: %v", docID, delErr)
		}
	}
	// 同步从Neo4j知识图谱删除
	if svc.neo4jGraph != nil {
		if delErr := svc.neo4jGraph.DeleteByDocID(ctx, docID); delErr != nil {
			klog.Warnf("删除文档[%d]Neo4j图谱数据失败: %v", docID, delErr)
		}
	}
}

func getStringValue(v *qdrant.Value) string {
	if v == nil {
		return ""
	}
	switch val := v.Kind.(type) {
	case *qdrant.Value_StringValue:
		return val.StringValue
	default:
		return fmt.Sprintf("%v", v)
	}
}

func getInt64Value(v *qdrant.Value) int64 {
	if v == nil {
		return 0
	}
	switch val := v.Kind.(type) {
	case *qdrant.Value_IntegerValue:
		return val.IntegerValue
	default:
		return 0
	}
}

func (svc *KnowledgeService) publishDocParse(docID int64, kbID int64) error {
	if svc.kafkaBroker == "" || svc.kafkaTopic == "" {
		return nil
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(svc.kafkaBroker),
		Topic:        svc.kafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
	}
	defer func() { _ = writer.Close() }()
	type docParseMsg struct {
		DocID int64 `json:"doc_id"`
		KbID  int64 `json:"kb_id"`
	}
	msg := docParseMsg{DocID: docID, KbID: kbID}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化文档解析消息失败: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return writer.WriteMessages(ctx, kafka.Message{Value: data})
}

func (svc *KnowledgeService) RepublishDocParse(docID int64, kbID int64) error {
	return svc.publishDocParse(docID, kbID)
}

// Reranker Rerank精排模型客户端
// 支持两种模式:
//   - jina: 调用Jina Reranker API（云端，无需下载模型，适合开发环境）
//   - vllm: 调用vLLM部署的BGE-Reranker /score API（本地GPU，适合生产环境）
//
// 在RRF粗排后对候选文档进行精准打分精排，只保留最相关的Top-N结果
type Reranker struct {
	baseURL string
	model   string
	topN    int
	enabled bool
	mode    string // "jina" 或 "vllm"
	apiKey  string // Jina API Key（jina模式必填）
	client  *http.Client
}

func NewReranker(baseURL, model string, topN int, enabled bool, mode, apiKey string) *Reranker {
	if !enabled {
		return &Reranker{enabled: false}
	}
	// 默认使用jina模式
	if mode == "" {
		mode = "jina"
	}
	return &Reranker{
		baseURL: baseURL,
		model:   model,
		topN:    topN,
		enabled: true,
		mode:    mode,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (r *Reranker) Enabled() bool {
	return r != nil && r.enabled
}

// rerankVLLMRequest vLLM /score API请求体
type rerankVLLMRequest struct {
	Model string   `json:"model"`
	Text1 string   `json:"text_1"`
	Text2 []string `json:"text_2"`
}

// rerankVLLMResponse vLLM /score API响应体
type rerankVLLMResponse struct {
	ID   string `json:"id"`
	Data []struct {
		Index int     `json:"index"`
		Score float64 `json:"score"`
	} `json:"data"`
}

// rerankJinaRequest Jina Reranker API请求体
type rerankJinaRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n"`
}

// rerankJinaResponse Jina Reranker API响应体
type rerankJinaResponse struct {
	Model   string `json:"model"`
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

// Rerank 对RRF粗排结果进行精排，返回Top-N最相关结果
func (r *Reranker) Rerank(ctx context.Context, query string, results []SearchResult) ([]SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	// 构建候选文档列表
	documents := make([]string, 0, len(results))
	for _, res := range results {
		documents = append(documents, res.Content)
	}

	var scoreMap map[int]float64
	var err error

	switch r.mode {
	case "jina":
		scoreMap, err = r.rerankJina(ctx, query, documents)
	case "vllm":
		scoreMap, err = r.rerankVLLM(ctx, query, documents)
	default:
		return nil, fmt.Errorf("不支持的rerank模式: %s (支持: jina, vllm)", r.mode)
	}

	if err != nil {
		return nil, err
	}

	// 按rerank得分降序排列
	type scoredResult struct {
		SearchResult
		rerankScore float64
	}
	scored := make([]scoredResult, 0, len(results))
	for i, res := range results {
		score := scoreMap[i] // 未返回的默认0分
		scored = append(scored, scoredResult{
			SearchResult: res,
			rerankScore:  score,
		})
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].rerankScore > scored[j].rerankScore
	})

	// 截取Top-N
	topN := r.topN
	if topN > len(scored) {
		topN = len(scored)
	}
	scored = scored[:topN]

	// 更新Score为rerank得分
	finalResults := make([]SearchResult, 0, topN)
	for _, s := range scored {
		s.Score = s.rerankScore
		finalResults = append(finalResults, s.SearchResult)
	}

	klog.Infof("Rerank精排完成(mode=%s): 输入%d条, 输出%d条", r.mode, len(results), topN)
	return finalResults, nil
}

// rerankVLLM 调用vLLM部署的BGE-Reranker /score API
func (r *Reranker) rerankVLLM(ctx context.Context, query string, documents []string) (map[int]float64, error) {
	reqBody := rerankVLLMRequest{
		Model: r.model,
		Text1: query,
		Text2: documents,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化rerank请求失败: %w", err)
	}

	url := strings.TrimRight(r.baseURL, "/") + "/score"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("创建rerank请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用rerank服务失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rerank服务返回非200状态码: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var rerankResp rerankVLLMResponse
	if err := json.NewDecoder(resp.Body).Decode(&rerankResp); err != nil {
		return nil, fmt.Errorf("解析rerank响应失败: %w", err)
	}

	scoreMap := make(map[int]float64)
	for _, item := range rerankResp.Data {
		scoreMap[item.Index] = item.Score
	}
	return scoreMap, nil
}

// rerankJina 调用Jina Reranker API（云端，无需本地部署模型）
func (r *Reranker) rerankJina(ctx context.Context, query string, documents []string) (map[int]float64, error) {
	reqBody := rerankJinaRequest{
		Model:     r.model,
		Query:     query,
		Documents: documents,
		TopN:      r.topN,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化jina rerank请求失败: %w", err)
	}

	url := strings.TrimRight(r.baseURL, "/") + "/rerank"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("创建jina rerank请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用jina rerank服务失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jina rerank服务返回非200状态码: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var jinaResp rerankJinaResponse
	if err := json.NewDecoder(resp.Body).Decode(&jinaResp); err != nil {
		return nil, fmt.Errorf("解析jina rerank响应失败: %w", err)
	}

	scoreMap := make(map[int]float64)
	for _, item := range jinaResp.Results {
		scoreMap[item.Index] = item.RelevanceScore
	}
	return scoreMap, nil
}
