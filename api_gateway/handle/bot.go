package handle

import (
	"api_gateway/middleware"
	"api_gateway/response"
	"api_gateway/rpc"
	"context"

	bot "bot_service/kitex_gen/bot"
	work "work_service/kitex_gen/work"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type CreateBotReq struct {
	Name         string `json:"name" vd:"len($) > 0"`
	AvatarUrl    string `json:"avatar_url"`
	SystemPrompt string `json:"system_prompt" vd:"len($) > 0"`
	ApiKey       string `json:"api_key"`
	Model        string `json:"model" vd:"len($) > 0"`
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
		AvatarUrl:    req.AvatarUrl,
		SystemPrompt: req.SystemPrompt,
		ApiKey:       req.ApiKey,
		Model:        req.Model,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "创建Bot失败: %v", err)
		response.Error(c, "创建Bot失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"bot_id": resp.BotId})
}

type UpdateBotReq struct {
	BotId        int64  `json:"bot_id" vd:"$ > 0"`
	Name         string `json:"name"`
	AvatarUrl    string `json:"avatar_url"`
	SystemPrompt string `json:"system_prompt"`
	ApiKey       string `json:"api_key"`
	Model        string `json:"model"`
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
	resp, err := rpc.UpdateBot(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "更新Bot失败: %v", err)
		response.Error(c, "更新Bot失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.Success})
}

type DeleteBotReq struct {
	BotId int64 `json:"bot_id" vd:"$ > 0"`
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
	response.Success(c, map[string]interface{}{"bots": resp.Bots})
}

type ChatWithBotReq struct {
	BotId          int64    `json:"bot_id" vd:"$ > 0"`
	ConversationId int64    `json:"conversation_id" vd:"$ > 0"`
	Content        string   `json:"content" vd:"len($) > 0"`
	History        []string `json:"history"`
}

type AddBotToConversationReq struct {
	BotId            int64 `json:"bot_id" vd:"$ > 0"`
	ConversationId   int64 `json:"conversation_id"`
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
		result["conversation_id"] = *resp.ConversationId
	}
	response.Success(c, result)
}

func ChatWithBot(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userId := userInfo.Id

	var req ChatWithBotReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.HandleMessage(ctx, &work.HandleMessageReq{
		BotId:          req.BotId,
		ConversationId: req.ConversationId,
		SenderId:       userId,
		Content:        req.Content,
		History:        req.History,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "Bot对话失败: %v", err)
		response.Error(c, "Bot对话失败", nil)
		return
	}
	response.Success(c, map[string]interface{}{
		"success": resp.Success,
	})
}
