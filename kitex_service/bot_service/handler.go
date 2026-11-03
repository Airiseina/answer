package main

import (
	"context"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/bot_service/kitex_gen/bot"

	"github.com/cloudwego/kitex/pkg/klog"
)

type BotServiceImpl struct {
	botService        *service.BotService
	mcpServerService  *service.McpServerService
	marketService     *service.MarketService
	agentGraphService *service.AgentGraphService
	skillService      *service.SkillService
}

func (s *BotServiceImpl) CreateBot(ctx context.Context, req *bot.CreateBotReq) (resp *bot.CreateBotRes, err error) {
	baseURL := ""
	if req.IsSetBaseUrl() {
		baseURL = req.GetBaseUrl()
	}
	botId, err := s.botService.CreateBot(ctx, req.GetTenantId(), req.CreatorId, req.Name, req.SystemPrompt, req.ApiKey, req.Model, baseURL, req.GetOrigin(), req.GetAppId())
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]创建Bot时发生系统错误: %v", req.CreatorId, err)
		return &bot.CreateBotRes{Success: false}, err
	}
	return &bot.CreateBotRes{Success: true, BotId: botId}, nil
}

func (s *BotServiceImpl) GetBot(ctx context.Context, req *bot.GetBotReq) (resp *bot.GetBotRes, err error) {
	b, err := s.botService.GetBot(req.GetTenantId(), req.BotId)
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
			BaseUrl:      &b.BaseURL,
			Origin:       &b.Origin,
			AppId:        &b.AppID,
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
	bots, err := s.botService.GetUserBots(req.GetTenantId(), req.CreatorId)
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
			BaseUrl:      &b.BaseURL,
			Origin:       &b.Origin,
			AppId:        &b.AppID,
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
	if req.IsSetBaseUrl() {
		updates["base_url"] = req.GetBaseUrl()
	}
	if len(updates) == 0 {
		return &bot.CommonRes{Success: false}, nil
	}
	success, err := s.botService.UpdateBot(req.GetTenantId(), req.BotId, req.OperatorId, updates)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]更新Bot[%d]时发生系统错误: %v", req.OperatorId, req.BotId, err)
		return &bot.CommonRes{Success: false}, err
	}
	return &bot.CommonRes{Success: success}, nil
}

func (s *BotServiceImpl) DeleteBot(ctx context.Context, req *bot.DeleteBotReq) (resp *bot.CommonRes, err error) {
	success, err := s.botService.DeleteBot(req.GetTenantId(), req.BotId, req.OperatorId)
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
	b, err := s.botService.GetBot(req.GetTenantId(), req.BotId)
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
		BaseUrl:      &b.BaseURL,
		Origin:       &b.Origin,
	}, nil
}

func (s *BotServiceImpl) AddBotToConversation(ctx context.Context, req *bot.AddBotToConversationReq) (resp *bot.AddBotToConversationRes, err error) {
	convID, err := s.botService.AddBotToConversation(ctx, req.GetTenantId(), req.OperatorId, req.BotId, req.ConversationId, req.ConversationType)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]将Bot[%d]拉入会话失败: %v", req.OperatorId, req.BotId, err)
		return &bot.AddBotToConversationRes{Success: false}, err
	}
	return &bot.AddBotToConversationRes{
		Success:        true,
		ConversationId: &convID,
	}, nil
}

// CreateMcpServer implements the BotServiceImpl interface.
func (s *BotServiceImpl) CreateMcpServer(ctx context.Context, req *bot.CreateMcpServerReq) (resp *bot.CreateMcpServerRes, err error) {
	description := ""
	if req.IsSetDescription() {
		description = req.GetDescription()
	}
	transport := ""
	if req.IsSetTransport() {
		transport = req.GetTransport()
	}
	authType := ""
	if req.IsSetAuthType() {
		authType = req.GetAuthType()
	}
	authToken := ""
	if req.IsSetAuthToken() {
		authToken = req.GetAuthToken()
	}
	id, err := s.mcpServerService.CreateMcpServer(req.GetTenantId(), req.OperatorId, req.BotId, req.Name, req.Url, description, transport, authType, authToken)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]为Bot[%d]创建McpServer失败: %v", req.OperatorId, req.BotId, err)
		return &bot.CreateMcpServerRes{Success: false}, err
	}
	return &bot.CreateMcpServerRes{Success: true, Id: id}, nil
}

// GetBotMcpServers implements the BotServiceImpl interface.
func (s *BotServiceImpl) GetBotMcpServers(ctx context.Context, req *bot.GetBotMcpServersReq) (resp *bot.GetBotMcpServersRes, err error) {
	dtos, err := s.mcpServerService.GetBotMcpServers(req.GetTenantId(), req.BotId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询Bot[%d]的McpServer列表失败: %v", req.BotId, err)
		return &bot.GetBotMcpServersRes{Success: false}, err
	}
	var list []*bot.McpServerInfo
	for _, d := range dtos {
		item := &bot.McpServerInfo{
			Id:          d.ID,
			BotId:       d.BotID,
			Name:        d.Name,
			Description: d.Description,
			Transport:   d.Transport,
			Url:         d.URL,
			AuthType:    d.AuthType,
			Enabled:     d.Enabled,
			CreatedAt:   d.CreatedAt,
		}
		if d.AuthToken != "" {
			item.AuthToken = &d.AuthToken
		}
		list = append(list, item)
	}
	return &bot.GetBotMcpServersRes{Success: true, Servers: list}, nil
}

// UpdateMcpServer implements the BotServiceImpl interface.
func (s *BotServiceImpl) UpdateMcpServer(ctx context.Context, req *bot.UpdateMcpServerReq) (resp *bot.CommonRes, err error) {
	updates := make(map[string]interface{})
	if req.IsSetName() {
		updates["name"] = req.GetName()
	}
	if req.IsSetDescription() {
		updates["description"] = req.GetDescription()
	}
	if req.IsSetTransport() {
		updates["transport"] = req.GetTransport()
	}
	if req.IsSetUrl() {
		updates["url"] = req.GetUrl()
	}
	if req.IsSetAuthType() {
		updates["auth_type"] = req.GetAuthType()
	}
	if req.IsSetAuthToken() {
		updates["auth_token"] = req.GetAuthToken()
	}
	if req.IsSetEnabled() {
		updates["enabled"] = req.GetEnabled()
	}
	if len(updates) == 0 {
		return &bot.CommonRes{Success: false}, nil
	}
	success, err := s.mcpServerService.UpdateMcpServer(req.GetTenantId(), req.Id, req.OperatorId, updates)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]更新McpServer[%d]失败: %v", req.OperatorId, req.Id, err)
		return &bot.CommonRes{Success: false}, err
	}
	return &bot.CommonRes{Success: success}, nil
}

// DeleteMcpServer implements the BotServiceImpl interface.
func (s *BotServiceImpl) DeleteMcpServer(ctx context.Context, req *bot.DeleteMcpServerReq) (resp *bot.CommonRes, err error) {
	success, err := s.mcpServerService.DeleteMcpServer(req.GetTenantId(), req.Id, req.OperatorId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]删除McpServer[%d]失败: %v", req.OperatorId, req.Id, err)
		return &bot.CommonRes{Success: false}, err
	}
	return &bot.CommonRes{Success: success}, nil
}

// toMarketBotInfo 把服务层的市场条目视图转换为 IDL 结构。
func toMarketBotInfo(d service.MarketBotDTO) *bot.MarketBotInfo {
	info := &bot.MarketBotInfo{
		MarketBotId:   d.MarketBotId,
		PublisherId:   d.PublisherId,
		PublisherName: d.PublisherName,
		Name:          d.Name,
		Description:   d.Description,
		AvatarUrl:     d.AvatarUrl,
		SystemPrompt:  d.SystemPrompt,
		Model:         d.Model,
		Category:      d.Category,
		Tags:          d.Tags,
		Status:        d.Status,
		ReviewNote:    d.ReviewNote,
		InstallCount:  d.InstallCount,
		Rating:        d.Rating,
		RatingCount:   d.RatingCount,
		Version:       d.Version,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
	if d.BaseURL != "" {
		info.BaseUrl = &d.BaseURL
	}
	return info
}

// PublishMarketBot 用户把自己编写的 Bot 定义提交到市场，进入待审核。
func (s *BotServiceImpl) PublishMarketBot(ctx context.Context, req *bot.PublishMarketBotReq) (resp *bot.PublishMarketBotRes, err error) {
	id, status, err := s.marketService.Publish(ctx, req.GetTenantId(), req.PublisherId, req.Name,
		req.SystemPrompt, req.Description, req.GetAvatarUrl(), req.GetModel(), req.GetBaseUrl(),
		req.GetCategory(), req.GetTags())
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]发布市场Bot失败: %v", req.PublisherId, err)
		return &bot.PublishMarketBotRes{Success: false}, err
	}
	return &bot.PublishMarketBotRes{Success: true, MarketBotId: id, Status: status}, nil
}

// UpdateMarketBot 发布者修改自己的市场条目；改动系统提示词会重新进入待审核。
func (s *BotServiceImpl) UpdateMarketBot(ctx context.Context, req *bot.UpdateMarketBotReq) (resp *bot.CommonRes, err error) {
	updates := make(map[string]interface{})
	if req.IsSetName() {
		updates["name"] = req.GetName()
	}
	if req.IsSetDescription() {
		updates["description"] = req.GetDescription()
	}
	if req.IsSetAvatarUrl() {
		updates["avatar_url"] = req.GetAvatarUrl()
	}
	if req.IsSetSystemPrompt() {
		updates["system_prompt"] = req.GetSystemPrompt()
	}
	if req.IsSetModel() {
		updates["model"] = req.GetModel()
	}
	if req.IsSetBaseUrl() {
		updates["base_url"] = req.GetBaseUrl()
	}
	if req.IsSetCategory() {
		updates["category"] = req.GetCategory()
	}
	if req.IsSetTags() {
		updates["tags"] = req.GetTags()
	}
	if len(updates) == 0 {
		return &bot.CommonRes{Success: false}, nil
	}
	success, err := s.marketService.Update(ctx, req.GetTenantId(), req.MarketBotId, req.OperatorId, updates)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]更新市场Bot[%d]失败: %v", req.OperatorId, req.MarketBotId, err)
		return &bot.CommonRes{Success: false}, err
	}
	return &bot.CommonRes{Success: success}, nil
}

// GetMarketBot 查询市场条目详情；未发布内容仅发布者本人可见。
func (s *BotServiceImpl) GetMarketBot(ctx context.Context, req *bot.GetMarketBotReq) (resp *bot.GetMarketBotRes, err error) {
	dto, err := s.marketService.Get(ctx, req.MarketBotId, req.GetViewerId())
	if err != nil {
		klog.CtxErrorf(ctx, "查询市场Bot[%d]失败: %v", req.MarketBotId, err)
		return &bot.GetMarketBotRes{Success: false}, err
	}
	if dto.MarketBotId == 0 {
		return &bot.GetMarketBotRes{Success: false}, nil
	}
	return &bot.GetMarketBotRes{Success: true, Bot: toMarketBotInfo(dto)}, nil
}

// ListMarketBots 浏览已发布的市场条目（全局可见）。
func (s *BotServiceImpl) ListMarketBots(ctx context.Context, req *bot.ListMarketBotsReq) (resp *bot.ListMarketBotsRes, err error) {
	dtos, total, err := s.marketService.List(ctx, req.GetKeyword(), req.GetCategory(), req.GetPage(), req.GetPageSize())
	if err != nil {
		klog.CtxErrorf(ctx, "浏览市场Bot列表失败: %v", err)
		return &bot.ListMarketBotsRes{Success: false}, err
	}
	list := make([]*bot.MarketBotInfo, 0, len(dtos))
	for _, d := range dtos {
		list = append(list, toMarketBotInfo(d))
	}
	return &bot.ListMarketBotsRes{Success: true, Bots: list, Total: total}, nil
}

// ListMyMarketBots 发布者查看自己的条目；未传状态时返回全部。
func (s *BotServiceImpl) ListMyMarketBots(ctx context.Context, req *bot.ListMyMarketBotsReq) (resp *bot.ListMyMarketBotsRes, err error) {
	status := int16(-1)
	if req.IsSetStatus() {
		status = req.GetStatus()
	}
	dtos, total, err := s.marketService.ListMy(ctx, req.GetTenantId(), req.PublisherId, status, req.GetPage(), req.GetPageSize())
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]查询我的发布失败: %v", req.PublisherId, err)
		return &bot.ListMyMarketBotsRes{Success: false}, err
	}
	list := make([]*bot.MarketBotInfo, 0, len(dtos))
	for _, d := range dtos {
		list = append(list, toMarketBotInfo(d))
	}
	return &bot.ListMyMarketBotsRes{Success: true, Bots: list, Total: total}, nil
}

// InstallMarketBot 安装市场 Bot 为安装者自己的实例（origin=market）。
func (s *BotServiceImpl) InstallMarketBot(ctx context.Context, req *bot.InstallMarketBotReq) (resp *bot.InstallMarketBotRes, err error) {
	botId, version, err := s.marketService.Install(ctx, req.GetTenantId(), req.UserId, req.MarketBotId,
		req.Name, req.ApiKey, req.Model, req.GetBaseUrl())
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]安装市场Bot[%d]失败: %v", req.UserId, req.MarketBotId, err)
		return &bot.InstallMarketBotRes{Success: false}, err
	}
	return &bot.InstallMarketBotRes{Success: true, BotId: botId, Version: version}, nil
}

// RateMarketBot 用户对已发布的市场 Bot 评分（1-5）。
func (s *BotServiceImpl) RateMarketBot(ctx context.Context, req *bot.RateMarketBotReq) (resp *bot.CommonRes, err error) {
	success, err := s.marketService.Rate(ctx, req.GetTenantId(), req.UserId, req.MarketBotId, req.Score)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]评价市场Bot[%d]失败: %v", req.UserId, req.MarketBotId, err)
		return &bot.CommonRes{Success: false}, err
	}
	return &bot.CommonRes{Success: success}, nil
}

// OfflineMarketBot 发布者下架自己的市场条目。
func (s *BotServiceImpl) OfflineMarketBot(ctx context.Context, req *bot.OfflineMarketBotReq) (resp *bot.CommonRes, err error) {
	success, err := s.marketService.Offline(ctx, req.GetTenantId(), req.MarketBotId, req.OperatorId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]下架市场Bot[%d]失败: %v", req.OperatorId, req.MarketBotId, err)
		return &bot.CommonRes{Success: false}, err
	}
	return &bot.CommonRes{Success: success}, nil
}

// ListMarketReviews 管理员审核列表；未传状态时默认查看待审核队列。
func (s *BotServiceImpl) ListMarketReviews(ctx context.Context, req *bot.ListMarketReviewsReq) (resp *bot.ListMarketReviewsRes, err error) {
	status := int16(-1)
	if req.IsSetStatus() {
		status = req.GetStatus()
	}
	dtos, total, err := s.marketService.ListReviews(ctx, status, req.GetPage(), req.GetPageSize())
	if err != nil {
		klog.CtxErrorf(ctx, "查询市场审核列表失败: %v", err)
		return &bot.ListMarketReviewsRes{Success: false}, err
	}
	list := make([]*bot.MarketBotInfo, 0, len(dtos))
	for _, d := range dtos {
		list = append(list, toMarketBotInfo(d))
	}
	return &bot.ListMarketReviewsRes{Success: true, Bots: list, Total: total}, nil
}

// ReviewMarketBot 管理员审核：通过上架或驳回并记录意见。
func (s *BotServiceImpl) ReviewMarketBot(ctx context.Context, req *bot.ReviewMarketBotReq) (resp *bot.CommonRes, err error) {
	success, err := s.marketService.Review(ctx, req.AdminId, req.MarketBotId, req.Approve, req.GetNote())
	if err != nil {
		klog.CtxErrorf(ctx, "管理员[%d]审核市场Bot[%d]失败: %v", req.AdminId, req.MarketBotId, err)
		return &bot.CommonRes{Success: false}, err
	}
	return &bot.CommonRes{Success: success}, nil
}

// SaveAgentGraph 保存 Bot 的多 Agent 协作编排图（新建或更新草稿）。
func (s *BotServiceImpl) SaveAgentGraph(ctx context.Context, req *bot.SaveAgentGraphReq) (resp *bot.SaveAgentGraphRes, err error) {
	var graphId *int64
	if req.IsSetGraphId() {
		id := req.GetGraphId()
		graphId = &id
	}
	result, err := s.agentGraphService.Save(ctx, req.GetTenantId(), req.BotId, req.OperatorId,
		graphId, req.GetName(), req.GetGraphJson())
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]保存Bot[%d]编排图失败: %v", req.OperatorId, req.BotId, err)
		return &bot.SaveAgentGraphRes{Success: false}, err
	}
	// 解析失败时 GraphID 为 0，表示未落库；校验不通过的草稿仍会保存并回传错误列表。
	success := result.GraphID != 0
	return &bot.SaveAgentGraphRes{
		Success: success,
		GraphId: result.GraphID,
		Version: result.Version,
		Status:  result.Status,
		Errors:  result.Errors,
	}, nil
}

// GetAgentGraph 查询 Bot 的编排图；不存在时返回 Success=false。
func (s *BotServiceImpl) GetAgentGraph(ctx context.Context, req *bot.GetAgentGraphReq) (resp *bot.GetAgentGraphRes, err error) {
	dto, err := s.agentGraphService.Get(ctx, req.GetTenantId(), req.BotId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询Bot[%d]编排图失败: %v", req.BotId, err)
		return &bot.GetAgentGraphRes{Success: false}, err
	}
	if dto.GraphID == 0 {
		return &bot.GetAgentGraphRes{Success: false}, nil
	}
	return &bot.GetAgentGraphRes{Success: true, Graph: toAgentGraphInfo(dto)}, nil
}

// ValidateAgentGraph 只做编排图结构与内容校验，不落库。
func (s *BotServiceImpl) ValidateAgentGraph(ctx context.Context, req *bot.ValidateAgentGraphReq) (resp *bot.ValidateAgentGraphRes, err error) {
	valid, errs := s.agentGraphService.Validate(ctx, req.BotId, req.GetGraphJson())
	return &bot.ValidateAgentGraphRes{Valid: valid, Errors: errs}, nil
}

// PublishAgentGraph 发布编排图，发布后 work_service 才会按图协作执行。
func (s *BotServiceImpl) PublishAgentGraph(ctx context.Context, req *bot.PublishAgentGraphReq) (resp *bot.CommonRes, err error) {
	success, errs, err := s.agentGraphService.Publish(ctx, req.GetTenantId(), req.BotId, req.OperatorId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]发布Bot[%d]编排图失败: %v", req.OperatorId, req.BotId, err)
		return &bot.CommonRes{Success: false}, err
	}
	if !success && len(errs) > 0 {
		klog.CtxInfof(ctx, "Bot[%d]编排图发布被拒绝: %v", req.BotId, errs)
	}
	return &bot.CommonRes{Success: success}, nil
}

// ListAgentGraphHistory 查询编排图的历史版本快照。
func (s *BotServiceImpl) ListAgentGraphHistory(ctx context.Context, req *bot.ListAgentGraphHistoryReq) (resp *bot.ListAgentGraphHistoryRes, err error) {
	var limit int32
	if req.IsSetLimit() {
		limit = req.GetLimit()
	}
	dtos, err := s.agentGraphService.ListHistory(ctx, req.GetTenantId(), req.BotId, limit)
	if err != nil {
		klog.CtxErrorf(ctx, "查询Bot[%d]编排图历史失败: %v", req.BotId, err)
		return &bot.ListAgentGraphHistoryRes{Success: false}, err
	}
	items := make([]*bot.AgentGraphHistoryInfo, 0, len(dtos))
	for _, d := range dtos {
		items = append(items, &bot.AgentGraphHistoryInfo{
			Id:         d.ID,
			GraphId:    d.GraphID,
			Version:    d.Version,
			GraphJson:  d.GraphJSON,
			OperatorId: d.OperatorID,
			CreatedAt:  d.CreatedAt,
		})
	}
	return &bot.ListAgentGraphHistoryRes{Success: true, Items: items}, nil
}

// toAgentGraphInfo 把服务层的编排图视图转换为 IDL 结构。
func toAgentGraphInfo(d service.AgentGraphDTO) *bot.AgentGraphInfo {
	return &bot.AgentGraphInfo{
		GraphId:   d.GraphID,
		BotId:     d.BotID,
		Name:      d.Name,
		GraphJson: d.GraphJSON,
		Status:    d.Status,
		Version:   d.Version,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// UploadUserSkill 解析并保存用户上传的个人 Skill 包（zip 或单个 markdown）。
func (s *BotServiceImpl) UploadUserSkill(ctx context.Context, req *bot.UploadUserSkillReq) (resp *bot.UploadUserSkillRes, err error) {
	result, err := s.skillService.Upload(ctx, req.GetTenantId(), req.OwnerId, req.FileName, req.Data)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]上传个人Skill[%s]失败: %v", req.OwnerId, req.FileName, err)
		return &bot.UploadUserSkillRes{Success: false}, err
	}
	return &bot.UploadUserSkillRes{
		Success:   true,
		SkillId:   result.SkillId,
		FileCount: result.FileCount,
		SizeBytes: result.SizeBytes,
		Warnings:  result.Warnings,
	}, nil
}

// ListUserSkills 返回当前用户在当前租户下的个人 Skill 列表。
func (s *BotServiceImpl) ListUserSkills(ctx context.Context, req *bot.ListUserSkillsReq) (resp *bot.ListUserSkillsRes, err error) {
	dtos, err := s.skillService.List(req.GetTenantId(), req.OwnerId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询用户[%d]个人Skill列表失败: %v", req.OwnerId, err)
		return &bot.ListUserSkillsRes{Success: false}, err
	}
	list := make([]*bot.UserSkillInfo, 0, len(dtos))
	for _, d := range dtos {
		list = append(list, toUserSkillInfo(d))
	}
	return &bot.ListUserSkillsRes{Success: true, Skills: list}, nil
}

// GetUserSkill 读取个人 Skill 详情与其资料文件列表；不存在时返回 Success=false。
func (s *BotServiceImpl) GetUserSkill(ctx context.Context, req *bot.GetUserSkillReq) (resp *bot.GetUserSkillRes, err error) {
	dto, files, err := s.skillService.Get(req.GetTenantId(), req.SkillId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询个人Skill[%d]失败: %v", req.SkillId, err)
		return &bot.GetUserSkillRes{Success: false}, err
	}
	if dto.SkillId == 0 {
		return &bot.GetUserSkillRes{Success: false}, nil
	}
	fileList := make([]*bot.UserSkillFileInfo, 0, len(files))
	for _, f := range files {
		fileList = append(fileList, &bot.UserSkillFileInfo{
			FileId:    f.FileId,
			SkillId:   f.SkillId,
			RelPath:   f.RelPath,
			ObjectUrl: f.ObjectUrl,
			SizeBytes: f.SizeBytes,
		})
	}
	return &bot.GetUserSkillRes{Success: true, Skill: toUserSkillInfo(dto), Files: fileList}, nil
}

// DeleteUserSkill 删除个人 Skill，并回收其独占知识库与所有 Bot 绑定。
func (s *BotServiceImpl) DeleteUserSkill(ctx context.Context, req *bot.DeleteUserSkillReq) (resp *bot.CommonRes, err error) {
	success, err := s.skillService.Delete(ctx, req.GetTenantId(), req.SkillId, req.OperatorId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]删除个人Skill[%d]失败: %v", req.OperatorId, req.SkillId, err)
		return &bot.CommonRes{Success: false}, err
	}
	return &bot.CommonRes{Success: success}, nil
}

// BindUserSkill 把个人 Skill 绑定到 / 解绑自 Bot，绑定后并入既有知识库检索链路。
func (s *BotServiceImpl) BindUserSkill(ctx context.Context, req *bot.BindUserSkillReq) (resp *bot.CommonRes, err error) {
	success, err := s.skillService.Bind(ctx, req.GetTenantId(), req.OwnerId, req.BotId, req.SkillId, req.Enabled)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]绑定个人Skill[%d]到Bot[%d]失败: %v", req.OwnerId, req.SkillId, req.BotId, err)
		return &bot.CommonRes{Success: false}, err
	}
	return &bot.CommonRes{Success: success}, nil
}

// ListBotSkills 返回某个 Bot 已绑定的个人 Skill，仅 Bot 创建者可读。
func (s *BotServiceImpl) ListBotSkills(ctx context.Context, req *bot.ListBotSkillsReq) (resp *bot.ListBotSkillsRes, err error) {
	dtos, err := s.skillService.ListBotSkills(ctx, req.GetTenantId(), req.BotId, req.OperatorId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询Bot[%d]已绑定Skill失败: %v", req.BotId, err)
		return &bot.ListBotSkillsRes{Success: false}, err
	}
	list := make([]*bot.BotSkillInfo, 0, len(dtos))
	for _, d := range dtos {
		list = append(list, &bot.BotSkillInfo{
			SkillId:     d.SkillId,
			Name:        d.Name,
			Description: d.Description,
			Version:     d.Version,
			Enabled:     d.Enabled,
		})
	}
	return &bot.ListBotSkillsRes{Success: true, Skills: list}, nil
}

// toUserSkillInfo 把服务层的 Skill 视图转换为 IDL 结构。
func toUserSkillInfo(d service.SkillDTO) *bot.UserSkillInfo {
	return &bot.UserSkillInfo{
		SkillId:     d.SkillId,
		OwnerId:     d.OwnerId,
		Name:        d.Name,
		Description: d.Description,
		Version:     d.Version,
		Status:      d.Status,
		KbId:        d.KbId,
		SizeBytes:   d.SizeBytes,
		FileCount:   d.FileCount,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}
