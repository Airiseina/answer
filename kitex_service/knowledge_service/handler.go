package main

import (
	"context"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/service"
	knowledge "github.com/Airiseina/answer/kitex_service/knowledge_service/kitex_gen/knowledge"

	"github.com/cloudwego/kitex/pkg/klog"
)

type KnowledgeServiceImpl struct {
	knowledgeService *service.KnowledgeService
}

func (s *KnowledgeServiceImpl) CreateKnowledgeBase(ctx context.Context, req *knowledge.CreateKnowledgeBaseReq) (resp *knowledge.CreateKnowledgeBaseRes, err error) {
	description := ""
	if req.IsSetDescription() {
		description = req.GetDescription()
	}
	kbID, err := s.knowledgeService.CreateKnowledgeBase(ctx, req.OwnerId, req.Name, description)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]创建知识库失败: %v", req.OwnerId, err)
		return &knowledge.CreateKnowledgeBaseRes{Success: false}, err
	}
	return &knowledge.CreateKnowledgeBaseRes{Success: true, KbId: kbID}, nil
}

func (s *KnowledgeServiceImpl) GetKnowledgeBase(ctx context.Context, req *knowledge.GetKnowledgeBaseReq) (resp *knowledge.GetKnowledgeBaseRes, err error) {
	kb, err := s.knowledgeService.GetKnowledgeBase(req.KbId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询知识库[%d]失败: %v", req.KbId, err)
		return &knowledge.GetKnowledgeBaseRes{Success: false}, err
	}
	if kb.ID == 0 {
		return &knowledge.GetKnowledgeBaseRes{Success: false}, nil
	}
	return &knowledge.GetKnowledgeBaseRes{
		Success: true,
		KbInfo: &knowledge.KnowledgeBaseInfo{
			KbId:        kb.ID,
			OwnerId:     kb.OwnerID,
			Name:        kb.Name,
			Description: kb.Description,
			DocCount:    kb.DocCount,
			ChunkCount:  kb.ChunkCount,
			CreatedAt:   kb.CreatedAt.UnixMilli(),
		},
	}, nil
}

func (s *KnowledgeServiceImpl) GetUserKnowledgeBases(ctx context.Context, req *knowledge.GetUserKnowledgeBasesReq) (resp *knowledge.GetUserKnowledgeBasesRes, err error) {
	kbs, err := s.knowledgeService.GetUserKnowledgeBases(req.OwnerId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询用户[%d]知识库列表失败: %v", req.OwnerId, err)
		return &knowledge.GetUserKnowledgeBasesRes{Success: false}, err
	}
	var list []*knowledge.KnowledgeBaseInfo
	for _, kb := range kbs {
		list = append(list, &knowledge.KnowledgeBaseInfo{
			KbId:        kb.ID,
			OwnerId:     kb.OwnerID,
			Name:        kb.Name,
			Description: kb.Description,
			DocCount:    kb.DocCount,
			ChunkCount:  kb.ChunkCount,
			CreatedAt:   kb.CreatedAt.UnixMilli(),
		})
	}
	return &knowledge.GetUserKnowledgeBasesRes{Success: true, KnowledgeBases: list}, nil
}

func (s *KnowledgeServiceImpl) UpdateKnowledgeBase(ctx context.Context, req *knowledge.UpdateKnowledgeBaseReq) (resp *knowledge.CommonRes, err error) {
	updates := make(map[string]interface{})
	if req.IsSetName() {
		updates["name"] = req.GetName()
	}
	if req.IsSetDescription() {
		updates["description"] = req.GetDescription()
	}
	if len(updates) == 0 {
		return &knowledge.CommonRes{Success: false}, nil
	}
	success, err := s.knowledgeService.UpdateKnowledgeBase(req.KbId, req.OperatorId, updates)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]更新知识库[%d]失败: %v", req.OperatorId, req.KbId, err)
		return &knowledge.CommonRes{Success: false}, err
	}
	return &knowledge.CommonRes{Success: success}, nil
}

func (s *KnowledgeServiceImpl) DeleteKnowledgeBase(ctx context.Context, req *knowledge.DeleteKnowledgeBaseReq) (resp *knowledge.CommonRes, err error) {
	success, err := s.knowledgeService.DeleteKnowledgeBase(ctx, req.KbId, req.OperatorId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]删除知识库[%d]失败: %v", req.OperatorId, req.KbId, err)
		return &knowledge.CommonRes{Success: false}, err
	}
	return &knowledge.CommonRes{Success: success}, nil
}

func (s *KnowledgeServiceImpl) AddDocument(ctx context.Context, req *knowledge.AddDocumentReq) (resp *knowledge.AddDocumentRes, err error) {
	docID, err := s.knowledgeService.AddDocument(ctx, req.KbId, req.OperatorId, req.FileName, req.FileUrl, req.FileType, req.FileSize)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]添加文档到知识库[%d]失败: %v", req.OperatorId, req.KbId, err)
		return &knowledge.AddDocumentRes{Success: false}, err
	}
	return &knowledge.AddDocumentRes{Success: true, DocId: docID}, nil
}

func (s *KnowledgeServiceImpl) GetDocuments(ctx context.Context, req *knowledge.GetDocumentsReq) (resp *knowledge.GetDocumentsRes, err error) {
	docs, err := s.knowledgeService.GetDocuments(req.KbId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询知识库[%d]文档列表失败: %v", req.KbId, err)
		return &knowledge.GetDocumentsRes{Success: false}, err
	}
	var list []*knowledge.DocumentInfo
	for _, doc := range docs {
		item := &knowledge.DocumentInfo{
			DocId:      doc.ID,
			KbId:       doc.KBID,
			FileName:   doc.FileName,
			FileUrl:    doc.FileURL,
			FileType:   doc.FileType,
			FileSize:   doc.FileSize,
			Status:     doc.Status,
			ChunkCount: doc.ChunkCount,
			CreatedAt:  doc.CreatedAt.UnixMilli(),
		}
		if doc.ErrorMessage != "" {
			item.ErrorMessage = &doc.ErrorMessage
		}
		list = append(list, item)
	}
	return &knowledge.GetDocumentsRes{Success: true, Documents: list}, nil
}

func (s *KnowledgeServiceImpl) DeleteDocument(ctx context.Context, req *knowledge.DeleteDocumentReq) (resp *knowledge.CommonRes, err error) {
	success, err := s.knowledgeService.DeleteDocument(ctx, req.DocId, req.OperatorId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]删除文档[%d]失败: %v", req.OperatorId, req.DocId, err)
		return &knowledge.CommonRes{Success: false}, err
	}
	return &knowledge.CommonRes{Success: success}, nil
}

func (s *KnowledgeServiceImpl) RetryDocument(ctx context.Context, req *knowledge.RetryDocumentReq) (resp *knowledge.CommonRes, err error) {
	success, err := s.knowledgeService.RetryDocument(ctx, req.DocId, req.OperatorId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]重试文档[%d]解析失败: %v", req.OperatorId, req.DocId, err)
		return &knowledge.CommonRes{Success: false}, err
	}
	return &knowledge.CommonRes{Success: success}, nil
}

func (s *KnowledgeServiceImpl) BindKnowledgeBase(ctx context.Context, req *knowledge.BindKnowledgeBaseReq) (resp *knowledge.CommonRes, err error) {
	success, err := s.knowledgeService.BindKnowledgeBase(req.BotId, req.OperatorId, req.KbId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]绑定Bot[%d]与知识库[%d]失败: %v", req.OperatorId, req.BotId, req.KbId, err)
		return &knowledge.CommonRes{Success: false}, err
	}
	return &knowledge.CommonRes{Success: success}, nil
}

func (s *KnowledgeServiceImpl) UnbindKnowledgeBase(ctx context.Context, req *knowledge.UnbindKnowledgeBaseReq) (resp *knowledge.CommonRes, err error) {
	success, err := s.knowledgeService.UnbindKnowledgeBase(req.BotId, req.OperatorId, req.KbId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]解绑Bot[%d]与知识库[%d]失败: %v", req.OperatorId, req.BotId, req.KbId, err)
		return &knowledge.CommonRes{Success: false}, err
	}
	return &knowledge.CommonRes{Success: success}, nil
}

func (s *KnowledgeServiceImpl) GetBotKnowledgeBases(ctx context.Context, req *knowledge.GetBotKnowledgeBasesReq) (resp *knowledge.GetBotKnowledgeBasesRes, err error) {
	kbs, err := s.knowledgeService.GetBotKnowledgeBases(req.BotId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询Bot[%d]关联的知识库失败: %v", req.BotId, err)
		return &knowledge.GetBotKnowledgeBasesRes{Success: false}, err
	}
	var list []*knowledge.KnowledgeBaseInfo
	for _, kb := range kbs {
		list = append(list, &knowledge.KnowledgeBaseInfo{
			KbId:        kb.ID,
			OwnerId:     kb.OwnerID,
			Name:        kb.Name,
			Description: kb.Description,
			DocCount:    kb.DocCount,
			ChunkCount:  kb.ChunkCount,
			CreatedAt:   kb.CreatedAt.UnixMilli(),
		})
	}
	return &knowledge.GetBotKnowledgeBasesRes{Success: true, KnowledgeBases: list}, nil
}

func (s *KnowledgeServiceImpl) SearchKnowledge(ctx context.Context, req *knowledge.SearchKnowledgeReq) (resp *knowledge.SearchKnowledgeRes, err error) {
	topK := int(req.TopK)
	if topK <= 0 {
		topK = 5
	}
	results, err := s.knowledgeService.SearchKnowledge(ctx, req.KbIds, req.Query, topK)
	if err != nil {
		klog.CtxErrorf(ctx, "知识库检索失败: %v", err)
		return &knowledge.SearchKnowledgeRes{Success: false}, err
	}
	var chunks []*knowledge.KnowledgeChunk
	for _, r := range results {
		item := &knowledge.KnowledgeChunk{
			Content:    r.Content,
			Source:     r.Source,
			DocId:      r.DocID,
			KbId:       r.KBID,
			ChunkIndex: int32(r.ChunkIndex),
			Score:      r.Score,
		}
		if r.PageNumber != nil {
			pageNum := int32(*r.PageNumber)
			item.PageNumber = &pageNum
		}
		chunks = append(chunks, item)
	}
	return &knowledge.SearchKnowledgeRes{Success: true, Chunks: chunks}, nil
}

// BindSystemKnowledgeBase implements the KnowledgeServiceImpl interface.
func (s *KnowledgeServiceImpl) BindSystemKnowledgeBase(ctx context.Context, req *knowledge.BindSystemKnowledgeBaseReq) (resp *knowledge.CommonRes, err error) {
	success, err := s.knowledgeService.BindSystemKnowledgeBase(req.BotId, req.KbId)
	if err != nil {
		klog.CtxErrorf(ctx, "系统Bot[%d]绑定知识库[%d]失败: %v", req.BotId, req.KbId, err)
		return &knowledge.CommonRes{Success: false}, err
	}
	return &knowledge.CommonRes{Success: success}, nil
}

// AddSystemDocument implements the KnowledgeServiceImpl interface.
func (s *KnowledgeServiceImpl) AddSystemDocument(ctx context.Context, req *knowledge.AddSystemDocumentReq) (resp *knowledge.AddDocumentRes, err error) {
	docID, err := s.knowledgeService.AddSystemDocument(ctx, req.KbId, req.FileName, req.FileUrl, req.FileType, req.FileSize)
	if err != nil {
		klog.CtxErrorf(ctx, "系统知识库[%d]添加文档[%s]失败: %v", req.KbId, req.FileName, err)
		return &knowledge.AddDocumentRes{Success: false}, err
	}
	return &knowledge.AddDocumentRes{Success: true, DocId: docID}, nil
}
