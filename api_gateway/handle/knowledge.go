package handle

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"

	bot "github.com/Airiseina/answer/kitex_service/bot_service/kitex_gen/bot"
	knowledge "github.com/Airiseina/answer/kitex_service/knowledge_service/kitex_gen/knowledge"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type knowledgeBaseItem struct {
	KbId        string `json:"kb_id"`
	OwnerId     string `json:"owner_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DocCount    int32  `json:"doc_count"`
	ChunkCount  int32  `json:"chunk_count"`
	CreatedAt   string `json:"created_at"`
}

func convertKnowledgeBaseInfo(kb *knowledge.KnowledgeBaseInfo) knowledgeBaseItem {
	return knowledgeBaseItem{
		KbId:        fmt.Sprintf("%d", kb.KbId),
		OwnerId:     fmt.Sprintf("%d", kb.OwnerId),
		Name:        kb.Name,
		Description: kb.Description,
		DocCount:    kb.DocCount,
		ChunkCount:  kb.ChunkCount,
		CreatedAt:   fmt.Sprintf("%d", kb.CreatedAt),
	}
}

type documentItem struct {
	DocId        string  `json:"doc_id"`
	KbId         string  `json:"kb_id"`
	FileName     string  `json:"file_name"`
	FileUrl      string  `json:"file_url"`
	FileType     string  `json:"file_type"`
	FileSize     int64   `json:"file_size"`
	Status       string  `json:"status"`
	ChunkCount   int32   `json:"chunk_count"`
	ErrorMessage *string `json:"error_message,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

func convertDocumentInfo(doc *knowledge.DocumentInfo) documentItem {
	return documentItem{
		DocId:        fmt.Sprintf("%d", doc.DocId),
		KbId:         fmt.Sprintf("%d", doc.KbId),
		FileName:     doc.FileName,
		FileUrl:      doc.FileUrl,
		FileType:     doc.FileType,
		FileSize:     doc.FileSize,
		Status:       doc.Status,
		ChunkCount:   doc.ChunkCount,
		ErrorMessage: doc.ErrorMessage,
		CreatedAt:    fmt.Sprintf("%d", doc.CreatedAt),
	}
}

type CreateKnowledgeBaseReq struct {
	Name        string `json:"name" vd:"len($) > 0"`
	Description string `json:"description"`
}

func CreateKnowledgeBase(ctx context.Context, c *app.RequestContext) {
	identity, _ := c.Get(middleware.IdentityKey)
	userInfo := identity.(*middleware.Resp)
	userId := userInfo.Id
	var req CreateKnowledgeBaseReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.CreateKnowledgeBase(ctx, &knowledge.CreateKnowledgeBaseReq{
		OwnerId:     userId,
		Name:        req.Name,
		Description: &req.Description,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "创建知识库失败: %v", err)
		response.Error(c, "创建知识库失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"kb_id": fmt.Sprintf("%d", resp.KbId)})
}

func GetKnowledgeBase(ctx context.Context, c *app.RequestContext) {
	kbIDStr := c.Query("kb_id")
	kbID, err := strconv.ParseInt(kbIDStr, 10, 64)
	if err != nil {
		response.Error(c, "参数错误", "kb_id无效")
		return
	}
	resp, err := rpc.GetKnowledgeBase(ctx, &knowledge.GetKnowledgeBaseReq{KbId: kbID})
	if err != nil {
		hlog.CtxErrorf(ctx, "查询知识库失败: %v", err)
		response.Error(c, "查询知识库失败", nil)
		return
	}
	if !resp.Success {
		response.Error(c, "知识库不存在", nil)
		return
	}
	response.Success(c, convertKnowledgeBaseInfo(resp.KbInfo))
}

func GetUserKnowledgeBases(ctx context.Context, c *app.RequestContext) {
	identity, _ := c.Get(middleware.IdentityKey)
	userInfo := identity.(*middleware.Resp)
	userId := userInfo.Id

	resp, err := rpc.GetUserKnowledgeBases(ctx, &knowledge.GetUserKnowledgeBasesReq{OwnerId: userId})
	if err != nil {
		hlog.CtxErrorf(ctx, "查询用户知识库列表失败: %v", err)
		response.Error(c, "查询知识库列表失败", nil)
		return
	}
	var items []knowledgeBaseItem
	for _, kb := range resp.KnowledgeBases {
		items = append(items, convertKnowledgeBaseInfo(kb))
	}
	if items == nil {
		items = []knowledgeBaseItem{}
	}
	response.Success(c, map[string]interface{}{"knowledge_bases": items})
}

type UpdateKnowledgeBaseReq struct {
	KbID        int64  `json:"kb_id,string" vd:"$ > 0"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func UpdateKnowledgeBase(ctx context.Context, c *app.RequestContext) {
	identity, _ := c.Get(middleware.IdentityKey)
	userInfo := identity.(*middleware.Resp)
	userId := userInfo.Id

	var req UpdateKnowledgeBaseReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	kitexReq := &knowledge.UpdateKnowledgeBaseReq{
		KbId:       req.KbID,
		OperatorId: userId,
	}
	if req.Name != "" {
		kitexReq.Name = &req.Name
	}
	if req.Description != "" {
		kitexReq.Description = &req.Description
	}
	resp, err := rpc.UpdateKnowledgeBase(ctx, kitexReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "更新知识库失败: %v", err)
		response.Error(c, "更新知识库失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

type DeleteKnowledgeBaseReq struct {
	KbID int64 `json:"kb_id,string" vd:"$ > 0"`
}

func DeleteKnowledgeBase(ctx context.Context, c *app.RequestContext) {
	identity, _ := c.Get(middleware.IdentityKey)
	userInfo := identity.(*middleware.Resp)
	userId := userInfo.Id

	var req DeleteKnowledgeBaseReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.DeleteKnowledgeBase(ctx, &knowledge.DeleteKnowledgeBaseReq{
		KbId:       req.KbID,
		OperatorId: userId,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "删除知识库失败: %v", err)
		response.Error(c, "删除知识库失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

type AddDocumentReq struct {
	KbID     int64  `json:"kb_id,string" vd:"$ > 0"`
	FileName string `json:"file_name" vd:"len($) > 0"`
	FileURL  string `json:"file_url" vd:"len($) > 0"`
	FileType string `json:"file_type" vd:"len($) > 0"`
	FileSize int64  `json:"file_size"`
}

func AddDocument(ctx context.Context, c *app.RequestContext) {
	identity, _ := c.Get(middleware.IdentityKey)
	userInfo := identity.(*middleware.Resp)
	userId := userInfo.Id

	var req AddDocumentReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.AddDocument(ctx, &knowledge.AddDocumentReq{
		KbId:       req.KbID,
		OperatorId: userId,
		FileName:   req.FileName,
		FileUrl:    req.FileURL,
		FileType:   req.FileType,
		FileSize:   req.FileSize,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "添加文档失败: %v", err)
		response.Error(c, "添加文档失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"doc_id": fmt.Sprintf("%d", resp.DocId)})
}

func GetDocuments(ctx context.Context, c *app.RequestContext) {
	kbIDStr := c.Query("kb_id")
	kbID, err := strconv.ParseInt(kbIDStr, 10, 64)
	if err != nil {
		response.Error(c, "参数错误", "kb_id无效")
		return
	}
	resp, err := rpc.GetDocuments(ctx, &knowledge.GetDocumentsReq{KbId: kbID})
	if err != nil {
		hlog.CtxErrorf(ctx, "查询文档列表失败: %v", err)
		response.Error(c, "查询文档列表失败", nil)
		return
	}
	var items []documentItem
	for _, doc := range resp.Documents {
		items = append(items, convertDocumentInfo(doc))
	}
	if items == nil {
		items = []documentItem{}
	}
	response.Success(c, map[string]interface{}{"documents": items})
}

type DeleteDocumentReq struct {
	DocID int64 `json:"doc_id,string" vd:"$ > 0"`
}

func DeleteDocument(ctx context.Context, c *app.RequestContext) {
	identity, _ := c.Get(middleware.IdentityKey)
	userInfo := identity.(*middleware.Resp)
	userId := userInfo.Id

	var req DeleteDocumentReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.DeleteDocument(ctx, &knowledge.DeleteDocumentReq{
		DocId:      req.DocID,
		OperatorId: userId,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "删除文档失败: %v", err)
		response.Error(c, "删除文档失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

type RetryDocumentReq struct {
	DocID int64 `json:"doc_id,string" vd:"$ > 0"`
}

func RetryDocument(ctx context.Context, c *app.RequestContext) {
	identity, _ := c.Get(middleware.IdentityKey)
	userInfo := identity.(*middleware.Resp)
	userId := userInfo.Id

	var req RetryDocumentReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.RetryDocument(ctx, &knowledge.RetryDocumentReq{
		DocId:      req.DocID,
		OperatorId: userId,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "重试文档解析失败: %v", err)
		response.Error(c, "重试文档解析失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

type BindKnowledgeBaseReq struct {
	BotID int64 `json:"bot_id,string" vd:"$ > 0"`
	KbID  int64 `json:"kb_id,string" vd:"$ > 0"`
}

func BindKnowledgeBase(ctx context.Context, c *app.RequestContext) {
	identity, _ := c.Get(middleware.IdentityKey)
	userInfo := identity.(*middleware.Resp)
	userId := userInfo.Id

	var req BindKnowledgeBaseReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	botResp, botErr := rpc.GetBot(ctx, &bot.GetBotReq{BotId: req.BotID})
	if botErr != nil {
		hlog.CtxErrorf(ctx, "查询Bot[%d]信息失败: %v", req.BotID, botErr)
		response.Error(c, "查询Bot信息失败", nil)
		return
	}
	if botResp == nil || botResp.BotInfo == nil {
		response.Error(c, "Bot不存在", nil)
		return
	}
	if botResp.BotInfo.IsSystem {
		response.Error(c, "系统Bot不支持绑定知识库", nil)
		return
	}
	resp, err := rpc.BindKnowledgeBase(ctx, &knowledge.BindKnowledgeBaseReq{
		BotId:      req.BotID,
		OperatorId: userId,
		KbId:       req.KbID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "绑定知识库失败: %v", err)
		response.Error(c, "绑定知识库失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

type UnbindKnowledgeBaseReq struct {
	BotID int64 `json:"bot_id,string" vd:"$ > 0"`
	KbID  int64 `json:"kb_id,string" vd:"$ > 0"`
}

func UnbindKnowledgeBase(ctx context.Context, c *app.RequestContext) {
	identity, _ := c.Get(middleware.IdentityKey)
	userInfo := identity.(*middleware.Resp)
	userId := userInfo.Id

	var req UnbindKnowledgeBaseReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.UnbindKnowledgeBase(ctx, &knowledge.UnbindKnowledgeBaseReq{
		BotId:      req.BotID,
		OperatorId: userId,
		KbId:       req.KbID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "解绑知识库失败: %v", err)
		response.Error(c, "解绑知识库失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

func GetBotKnowledgeBases(ctx context.Context, c *app.RequestContext) {
	botIDStr := c.Query("bot_id")
	botID, err := strconv.ParseInt(botIDStr, 10, 64)
	if err != nil {
		response.Error(c, "参数错误", "bot_id无效")
		return
	}
	resp, err := rpc.GetBotKnowledgeBases(ctx, &knowledge.GetBotKnowledgeBasesReq{BotId: botID})
	if err != nil {
		hlog.CtxErrorf(ctx, "查询Bot知识库列表失败: %v", err)
		response.Error(c, "查询Bot知识库列表失败", nil)
		return
	}
	var items []knowledgeBaseItem
	for _, kb := range resp.KnowledgeBases {
		items = append(items, convertKnowledgeBaseInfo(kb))
	}
	if items == nil {
		items = []knowledgeBaseItem{}
	}
	response.Success(c, map[string]interface{}{"knowledge_bases": items})
}
