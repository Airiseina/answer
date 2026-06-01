package handle

import (
	"context"
	"fmt"

	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"

	bot "github.com/Airiseina/answer/kitex_service/bot_service/kitex_gen/bot"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type CreateBotReq struct {
	Name         string `json:"name" vd:"len($) > 0"`
	SystemPrompt string `json:"system_prompt" vd:"len($) > 0"`
	ApiKey       string `json:"api_key"`
	Model        string `json:"model" vd:"len($) > 0"`
	BaseUrl      string `json:"base_url"`
}

func CreateBot(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userId := userInfo.Id

	var req CreateBotReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.CreateBot(ctx, &bot.CreateBotReq{
		CreatorId:    userId,
		Name:         req.Name,
		SystemPrompt: req.SystemPrompt,
		ApiKey:       req.ApiKey,
		Model:        req.Model,
		BaseUrl:      &req.BaseUrl,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "创建Bot失败: %v", err)
		response.Error(c, "创建Bot失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"bot_id": fmt.Sprintf("%d", resp.BotId)})
}

type UpdateBotReq struct {
	BotId        int64  `json:"bot_id,string" vd:"$ > 0"`
	Name         string `json:"name"`
	AvatarUrl    string `json:"avatar_url"`
	SystemPrompt string `json:"system_prompt"`
	ApiKey       string `json:"api_key"`
	Model        string `json:"model"`
	BaseUrl      string `json:"base_url"`
}

func UpdateBot(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userId := userInfo.Id

	var req UpdateBotReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	rpcReq := &bot.UpdateBotReq{
		BotId:      req.BotId,
		OperatorId: userId,
	}
	if req.Name != "" {
		rpcReq.Name = &req.Name
	}
	if req.AvatarUrl != "" {
		rpcReq.AvatarUrl = &req.AvatarUrl
	}
	if req.SystemPrompt != "" {
		rpcReq.SystemPrompt = &req.SystemPrompt
	}
	if req.ApiKey != "" {
		rpcReq.ApiKey = &req.ApiKey
	}
	if req.Model != "" {
		rpcReq.Model = &req.Model
	}
	if req.BaseUrl != "" {
		rpcReq.BaseUrl = &req.BaseUrl
	}
	resp, err := rpc.UpdateBot(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "更新Bot失败: %v", err)
		response.Error(c, "更新Bot失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

type DeleteBotReq struct {
	BotId int64 `json:"bot_id,string" vd:"$ > 0"`
}

func DeleteBot(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userId := userInfo.Id

	var req DeleteBotReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.DeleteBot(ctx, &bot.DeleteBotReq{
		BotId:      req.BotId,
		OperatorId: userId,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "删除Bot失败: %v", err)
		response.Error(c, "删除Bot失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

type botInfoItem struct {
	BotId        string  `json:"bot_id"`
	CreatorId    string  `json:"creator_id"`
	Name         string  `json:"name"`
	AvatarUrl    string  `json:"avatar_url"`
	SystemPrompt string  `json:"system_prompt"`
	Model        string  `json:"model"`
	IsSystem     bool    `json:"is_system"`
	CreatedAt    string  `json:"created_at"`
	BaseUrl      *string `json:"base_url,omitempty"`
}

func GetUserBots(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userId := userInfo.Id

	resp, err := rpc.GetUserBots(ctx, &bot.GetUserBotsReq{CreatorId: userId})
	if err != nil {
		hlog.CtxErrorf(ctx, "获取Bot列表失败: %v", err)
		response.Error(c, "获取Bot列表失败", nil)
		return
	}
	var items []botInfoItem
	for _, b := range resp.Bots {
		items = append(items, botInfoItem{
			BotId:        fmt.Sprintf("%d", b.BotId),
			CreatorId:    fmt.Sprintf("%d", b.CreatorId),
			Name:         b.Name,
			AvatarUrl:    b.AvatarUrl,
			SystemPrompt: b.SystemPrompt,
			Model:        b.Model,
			IsSystem:     b.IsSystem,
			CreatedAt:    fmt.Sprintf("%d", b.CreatedAt),
			BaseUrl:      b.BaseUrl,
		})
	}
	if items == nil {
		items = []botInfoItem{}
	}
	response.Success(c, map[string]interface{}{"bots": items})
}

type AddBotToConversationReq struct {
	BotId            int64 `json:"bot_id,string" vd:"$ > 0"`
	ConversationId   int64 `json:"conversation_id,string"`
	ConversationType int16 `json:"conversation_type" vd:"$ > 0"`
}

func AddBotToConversation(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userId := userInfo.Id

	var req AddBotToConversationReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.AddBotToConversation(ctx, &bot.AddBotToConversationReq{
		OperatorId:       userId,
		BotId:            req.BotId,
		ConversationId:   req.ConversationId,
		ConversationType: req.ConversationType,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "将Bot拉入会话失败: %v", err)
		response.Error(c, "将Bot拉入会话失败", err.Error())
		return
	}
	result := map[string]interface{}{"success": resp.Success}
	if resp.ConversationId != nil {
		result["conversation_id"] = fmt.Sprintf("%d", *resp.ConversationId)
	}
	response.Success(c, result)
}

func GetSystemBot(ctx context.Context, c *app.RequestContext) {
	resp, err := rpc.GetSystemBot(ctx)
	if err != nil {
		hlog.CtxErrorf(ctx, "获取系统Bot失败: %v", err)
		response.Error(c, "获取系统Bot失败", nil)
		return
	}
	result := map[string]interface{}{"success": resp.Success}
	if resp.BotId != 0 {
		result["bot_id"] = fmt.Sprintf("%d", resp.BotId)
	}
	response.Success(c, result)
}

type CreateMcpServerReq struct {
	BotId       int64  `json:"bot_id,string" vd:"$ > 0"`
	Name        string `json:"name" vd:"len($) > 0"`
	Url         string `json:"url" vd:"len($) > 0"`
	Description string `json:"description"`
	Transport   string `json:"transport"`
	AuthType    string `json:"auth_type"`
	AuthToken   string `json:"auth_token"`
}

func CreateMcpServer(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userId := userInfo.Id

	var req CreateMcpServerReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	botResp, botErr := rpc.GetBot(ctx, &bot.GetBotReq{BotId: req.BotId})
	if botErr != nil {
		hlog.CtxErrorf(ctx, "查询Bot[%d]信息失败: %v", req.BotId, botErr)
		response.Error(c, "查询Bot信息失败", nil)
		return
	}
	if botResp == nil || botResp.BotInfo == nil {
		response.Error(c, "Bot不存在", nil)
		return
	}
	if botResp.BotInfo.IsSystem {
		response.Error(c, "系统Bot不支持添加MCP Server", nil)
		return
	}
	rpcReq := &bot.CreateMcpServerReq{
		OperatorId: userId,
		BotId:      req.BotId,
		Name:       req.Name,
		Url:        req.Url,
	}
	if req.Description != "" {
		rpcReq.Description = &req.Description
	}
	if req.Transport != "" {
		rpcReq.Transport = &req.Transport
	}
	if req.AuthType != "" {
		rpcReq.AuthType = &req.AuthType
	}
	if req.AuthToken != "" {
		rpcReq.AuthToken = &req.AuthToken
	}
	resp, err := rpc.CreateMcpServer(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "创建McpServer失败: %v", err)
		response.Error(c, "创建McpServer失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"id": fmt.Sprintf("%d", resp.Id), "success": resp.Success})
}

type GetBotMcpServersReq struct {
	BotId int64 `json:"bot_id,string" vd:"$ > 0" query:"bot_id"`
}

type mcpServerItem struct {
	ID          string `json:"id"`
	BotId       string `json:"bot_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Transport   string `json:"transport"`
	Url         string `json:"url"`
	AuthType    string `json:"auth_type"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
}

func GetBotMcpServers(ctx context.Context, c *app.RequestContext) {
	var req GetBotMcpServersReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.GetBotMcpServers(ctx, &bot.GetBotMcpServersReq{BotId: req.BotId})
	if err != nil {
		hlog.CtxErrorf(ctx, "获取Bot[%d]的McpServer列表失败: %v", req.BotId, err)
		response.Error(c, "获取McpServer列表失败", nil)
		return
	}
	var items []mcpServerItem
	for _, s := range resp.Servers {
		items = append(items, mcpServerItem{
			ID:          fmt.Sprintf("%d", s.Id),
			BotId:       fmt.Sprintf("%d", s.BotId),
			Name:        s.Name,
			Description: s.Description,
			Transport:   s.Transport,
			Url:         s.Url,
			AuthType:    s.AuthType,
			Enabled:     s.Enabled,
			CreatedAt:   fmt.Sprintf("%d", s.CreatedAt),
		})
	}
	if items == nil {
		items = []mcpServerItem{}
	}
	response.Success(c, map[string]interface{}{"servers": items})
}

type UpdateMcpServerReq struct {
	Id          int64  `json:"id,string" vd:"$ > 0"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Transport   string `json:"transport"`
	Url         string `json:"url"`
	AuthType    string `json:"auth_type"`
	AuthToken   string `json:"auth_token"`
	Enabled     *bool  `json:"enabled"`
}

func UpdateMcpServer(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userId := userInfo.Id

	var req UpdateMcpServerReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	rpcReq := &bot.UpdateMcpServerReq{
		Id:         req.Id,
		OperatorId: userId,
	}
	if req.Name != "" {
		rpcReq.Name = &req.Name
	}
	if req.Description != "" {
		rpcReq.Description = &req.Description
	}
	if req.Transport != "" {
		rpcReq.Transport = &req.Transport
	}
	if req.Url != "" {
		rpcReq.Url = &req.Url
	}
	if req.AuthType != "" {
		rpcReq.AuthType = &req.AuthType
	}
	if req.AuthToken != "" {
		rpcReq.AuthToken = &req.AuthToken
	}
	if req.Enabled != nil {
		rpcReq.Enabled = req.Enabled
	}
	resp, err := rpc.UpdateMcpServer(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "更新McpServer失败: %v", err)
		response.Error(c, "更新McpServer失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

type DeleteMcpServerReq struct {
	Id int64 `json:"id,string" vd:"$ > 0"`
}

func DeleteMcpServer(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userId := userInfo.Id

	var req DeleteMcpServerReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.DeleteMcpServer(ctx, &bot.DeleteMcpServerReq{
		Id:         req.Id,
		OperatorId: userId,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "删除McpServer失败: %v", err)
		response.Error(c, "删除McpServer失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}
