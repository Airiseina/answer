package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/chunker"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/parser"
	"github.com/Airiseina/answer/pkg/ai"
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
	snow        *snowflake.Node
	kafkaBroker string
	kafkaTopic  string
}

func NewKnowledgeService(
	kbDao dal.KnowledgeBaseDao,
	docDao dal.DocumentDao,
	bkDao dal.BotKnowledgeDao,
	qdrantClient *qdrant.Client,
	kafkaBroker string,
	kafkaTopic string,
) *KnowledgeService {
	return &KnowledgeService{
		kbDao:       kbDao,
		docDao:      docDao,
		bkDao:       bkDao,
		qdrant:      qdrantClient,
		snow:        snowflake.NewNode(6),
		kafkaBroker: kafkaBroker,
		kafkaTopic:  kafkaTopic,
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
	defer os.Remove(localPath)
	p := parser.GetParser(doc.FileType)
	if p == nil {
		errMsg := fmt.Sprintf("不支持的文件类型: %s", doc.FileType)
		_ = svc.docDao.UpdateStatus(docID, model.DocStatusFailed, 0, errMsg)
		return fmt.Errorf(errMsg)
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
		return nil, fmt.Errorf("知识库检索失败: %w", err)
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
	if strings.HasPrefix(fileURL, storage.PublicURL) {
		fileURL = strings.TrimPrefix(fileURL, storage.PublicURL)
	}
	if strings.HasPrefix(fileURL, storage.BasePath) {
		fileURL = strings.TrimPrefix(fileURL, storage.BasePath)
	}
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
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("写入临时文件失败: %w", err)
	}
	tmpFile.Close()
	return tmpFile.Name(), nil
}

func (svc *KnowledgeService) storeVectors(ctx context.Context, doc model.KbDocument, chunks []chunker.Chunk, vectors [][]float32) error {
	svc.ensureCollection(ctx)
	points := make([]*qdrant.PointStruct, 0, len(chunks))
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
	}
	_, err := svc.qdrant.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: KbChunksCollection,
		Points:         points,
	})
	return err
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
	defer writer.Close()
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
