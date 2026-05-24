package main

import (
	"bot_service/internal/service"
	"bot_service/kitex_gen/bot"
	"context"

	"github.com/cloudwego/kitex/pkg/klog"
)

type BotServiceImpl struct {
	botService *service.BotService
}

func (s *BotServiceImpl) CreateBot(ctx context.Context, req *bot.CreateBotReq) (resp *bot.CreateBotRes, err error) {
	botId, err := s.botService.CreateBot(ctx, req.CreatorId, req.Name, req.AvatarUrl, req.SystemPrompt, req.ApiKey, req.Model)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]创建Bot时发生系统错误: %v", req.CreatorId, err)
		return &bot.CreateBotRes{Success: false}, err
	}
	return &bot.CreateBotRes{Success: true, BotId: botId}, nil
}

func (s *BotServiceImpl) GetBot(ctx context.Context, req *bot.GetBotReq) (resp *bot.GetBotRes, err error) {
	b, err := s.botService.GetBot(req.BotId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询Bot[%d]时发生系统错误: %v", req.BotId, err)
		return &bot.GetBotRes{Success: false}, err
	}
	if b.BotId == 0 {
		klog.CtxInfof(ctx, "查询不存在的Bot[%d]", req.BotId)
		return &bot.GetBotRes{Success: false}, nil
	}
	return &bot.GetBotRes{
		Success: true,
		BotInfo: &bot.BotInfo{
			BotId:        b.BotId,
			CreatorId:    b.CreatorId,
			Name:         b.Name,
			AvatarUrl:    b.AvatarUrl,
			SystemPrompt: b.SystemPrompt,
			Model:        b.Model,
			IsSystem:     b.IsSystem,
			CreatedAt:    b.CreatedAt,
		},
	}, nil
}

func (s *BotServiceImpl) GetSystemBot(ctx context.Context) (resp *bot.GetSystemBotRes, err error) {
	b, err := s.botService.GetSystemBot()
	if err != nil {
		klog.CtxErrorf(ctx, "查询系统Bot时发生系统错误: %v", err)
		return &bot.GetSystemBotRes{Success: false}, err
	}
	if b == 0 {
		return &bot.GetSystemBotRes{Success: false}, nil
	}
	return &bot.GetSystemBotRes{Success: true, BotId: b}, nil
}

func (s *BotServiceImpl) GetUserBots(ctx context.Context, req *bot.GetUserBotsReq) (resp *bot.GetUserBotsRes, err error) {
	bots, err := s.botService.GetUserBots(req.CreatorId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询用户[%d]Bot列表时发生系统错误: %v", req.CreatorId, err)
		return &bot.GetUserBotsRes{Success: false}, err
	}
	var list []*bot.BotInfo
	for _, b := range bots {
		list = append(list, &bot.BotInfo{
			BotId:        b.BotId,
			CreatorId:    b.CreatorId,
			Name:         b.Name,
			AvatarUrl:    b.AvatarUrl,
			SystemPrompt: b.SystemPrompt,
			Model:        b.Model,
			IsSystem:     b.IsSystem,
			CreatedAt:    b.CreatedAt,
		})
	}
	return &bot.GetUserBotsRes{Success: true, Bots: list}, nil
}

func (s *BotServiceImpl) UpdateBot(ctx context.Context, req *bot.UpdateBotReq) (resp *bot.CommonRes, err error) {
	updates := make(map[string]interface{})
	if req.IsSetName() {
		updates["name"] = req.GetName()
	}
	if req.IsSetAvatarUrl() {
		updates["avatar_url"] = req.GetAvatarUrl()
	}
	if req.IsSetSystemPrompt() {
		updates["system_prompt"] = req.GetSystemPrompt()
	}
	if req.IsSetApiKey() {
		updates["api_key"] = req.GetApiKey()
	}
	if req.IsSetModel() {
		updates["model"] = req.GetModel()
	}
	if len(updates) == 0 {
		return &bot.CommonRes{Success: false}, nil
	}
	success, err := s.botService.UpdateBot(req.BotId, req.OperatorId, updates)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]更新Bot[%d]时发生系统错误: %v", req.OperatorId, req.BotId, err)
		return &bot.CommonRes{Success: false}, err
	}
	return &bot.CommonRes{Success: success}, nil
}

func (s *BotServiceImpl) DeleteBot(ctx context.Context, req *bot.DeleteBotReq) (resp *bot.CommonRes, err error) {
	success, err := s.botService.DeleteBot(req.BotId, req.OperatorId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]删除Bot[%d]时发生系统错误: %v", req.OperatorId, req.BotId, err)
		return &bot.CommonRes{Success: false}, err
	}
	return &bot.CommonRes{Success: success}, nil
}

func (s *BotServiceImpl) IsBot(ctx context.Context, req *bot.IsBotReq) (resp *bot.IsBotRes, err error) {
	isBot, botId, err := s.botService.IsBot(req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询用户[%d]是否为Bot时发生系统错误: %v", req.UserId, err)
		return &bot.IsBotRes{IsBot: false}, err
	}
	res := &bot.IsBotRes{IsBot: isBot}
	if isBot {
		res.BotId = &botId
	}
	return res, nil
}

func (s *BotServiceImpl) GetBotConfig(ctx context.Context, req *bot.GetBotConfigReq) (resp *bot.GetBotConfigRes, err error) {
	b, err := s.botService.GetBot(req.BotId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询Bot[%d]配置时发生系统错误: %v", req.BotId, err)
		return &bot.GetBotConfigRes{Success: false}, err
	}
	return &bot.GetBotConfigRes{
		Success:      true,
		ApiKey:       &b.ApiKey,
		Model:        &b.Model,
		SystemPrompt: &b.SystemPrompt,
		UserId:       &b.UserId,
	}, nil
}

func (s *BotServiceImpl) AddBotToConversation(ctx context.Context, req *bot.AddBotToConversationReq) (resp *bot.AddBotToConversationRes, err error) {
	convID, err := s.botService.AddBotToConversation(ctx, req.OperatorId, req.BotId, req.ConversationId, req.ConversationType)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]将Bot[%d]拉入会话失败: %v", req.OperatorId, req.BotId, err)
		return &bot.AddBotToConversationRes{Success: false}, err
	}
	return &bot.AddBotToConversationRes{
		Success:        true,
		ConversationId: &convID,
	}, nil
}
