package main

import (
	"context"

	"github.com/Airiseina/answer/kitex_service/open_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/open_service/internal/service"
	"github.com/Airiseina/answer/kitex_service/open_service/kitex_gen/open"
	openrpc "github.com/Airiseina/answer/kitex_service/open_service/rpc"

	"github.com/cloudwego/kitex/pkg/klog"
)

// OpenServiceImpl 把 thrift 请求翻译成 service 层调用，并回填响应。
type OpenServiceImpl struct {
	openService *service.OpenService
}

// ===== 应用管理 =====

func (s *OpenServiceImpl) CreateApp(ctx context.Context, req *open.CreateAppReq) (resp *open.CreateAppRes, err error) {
	dto, secret, err := s.openService.CreateApp(ctx, req.OwnerId, req.TenantId, req.Name, req.Scopes, req.QpsLimit)
	if err != nil {
		klog.CtxErrorf(ctx, "创建应用[%s]失败: %v", req.Name, err)
		return nil, err
	}
	return &open.CreateAppRes{Success: dto.AppID != "", AppId: dto.AppID, AppSecret: secret}, nil
}

func (s *OpenServiceImpl) ListApps(ctx context.Context, req *open.ListAppsReq) (resp *open.ListAppsRes, err error) {
	dtos, err := s.openService.ListApps(ctx, req.OwnerId, req.TenantId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询应用列表失败: %v", err)
		return nil, err
	}
	list := make([]*open.AppInfo, 0, len(dtos))
	for _, dto := range dtos {
		list = append(list, toAppInfo(dto))
	}
	return &open.ListAppsRes{Success: true, Apps: list}, nil
}

func (s *OpenServiceImpl) GetApp(ctx context.Context, req *open.GetAppReq) (resp *open.GetAppRes, err error) {
	dto, secretHash, ok, err := s.openService.GetApp(ctx, req.AppId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询应用[%s]失败: %v", req.AppId, err)
		return nil, err
	}
	if !ok {
		return &open.GetAppRes{}, nil
	}
	return &open.GetAppRes{Success: true, App: toAppInfo(dto), SecretHash: &secretHash}, nil
}

func (s *OpenServiceImpl) VerifyApp(ctx context.Context, req *open.VerifyAppReq) (resp *open.VerifyAppRes, err error) {
	dto, ok, err := s.openService.VerifyApp(ctx, req.AppId, req.AppSecret)
	if err != nil {
		klog.CtxErrorf(ctx, "校验应用[%s]失败: %v", req.AppId, err)
		return nil, err
	}
	if !ok {
		return &open.VerifyAppRes{}, nil
	}
	return &open.VerifyAppRes{Success: true, App: toAppInfo(dto)}, nil
}

func (s *OpenServiceImpl) AuthenticateApp(ctx context.Context, req *open.AuthenticateAppReq) (resp *open.AuthenticateAppRes, err error) {
	dto, errCode, ok, err := s.openService.AuthenticateApp(ctx,
		req.AppId, req.Timestamp, req.Nonce, req.Method, req.Path, req.BodySha256, req.Signature)
	if err != nil {
		klog.CtxErrorf(ctx, "开放平台鉴权失败, app_id=%s, err=%v", req.AppId, err)
		return nil, err
	}
	res := &open.AuthenticateAppRes{Success: ok, ErrCode: errCode}
	if ok {
		res.App = toAppInfo(dto)
	}
	return res, nil
}

func (s *OpenServiceImpl) ResetAppSecret(ctx context.Context, req *open.ResetAppSecretReq) (resp *open.ResetAppSecretRes, err error) {
	secret, ok, err := s.openService.ResetAppSecret(ctx, req.AppId, req.OperatorId)
	if err != nil {
		klog.CtxErrorf(ctx, "轮换应用[%s]密钥失败: %v", req.AppId, err)
		return nil, err
	}
	return &open.ResetAppSecretRes{Success: ok, AppSecret: secret}, nil
}

func (s *OpenServiceImpl) UpdateAppStatus(ctx context.Context, req *open.UpdateAppStatusReq) (resp *open.CommonRes, err error) {
	ok, err := s.openService.UpdateAppStatus(ctx, req.AppId, req.OperatorId, req.Status)
	if err != nil {
		klog.CtxErrorf(ctx, "更新应用[%s]状态失败: %v", req.AppId, err)
		return nil, err
	}
	return &open.CommonRes{Success: ok}, nil
}

// ===== Bot 管理 =====

func (s *OpenServiceImpl) CreateBot(ctx context.Context, req *open.CreateOpenBotReq) (resp *open.CreateOpenBotRes, err error) {
	botID, err := s.openService.CreateBot(ctx, req.AppId, req.TenantId, req.CreatorId,
		req.Name, req.SystemPrompt, req.ApiKey, req.Model, req.GetBaseUrl())
	if err != nil {
		klog.CtxErrorf(ctx, "开放平台创建Bot失败, app_id=%s, err=%v", req.AppId, err)
		return nil, err
	}
	return &open.CreateOpenBotRes{Success: true, BotId: botID}, nil
}

func (s *OpenServiceImpl) ListBots(ctx context.Context, req *open.ListOpenBotsReq) (resp *open.ListOpenBotsRes, err error) {
	bots, err := s.openService.ListBots(ctx, req.AppId, req.TenantId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询应用[%s]的Bot列表失败: %v", req.AppId, err)
		return nil, err
	}
	list := make([]*open.OpenBotInfo, 0, len(bots))
	for _, b := range bots {
		list = append(list, toOpenBotInfo(b))
	}
	return &open.ListOpenBotsRes{Success: true, Bots: list}, nil
}

func (s *OpenServiceImpl) GetBot(ctx context.Context, req *open.GetOpenBotReq) (resp *open.GetOpenBotRes, err error) {
	botInfo, ok, err := s.openService.GetBot(ctx, req.AppId, req.TenantId, req.BotId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询开放平台Bot[%d]失败: %v", req.BotId, err)
		return nil, err
	}
	if !ok {
		return &open.GetOpenBotRes{}, nil
	}
	return &open.GetOpenBotRes{Success: true, Bot: toOpenBotInfo(botInfo)}, nil
}

func (s *OpenServiceImpl) UpdateBot(ctx context.Context, req *open.UpdateOpenBotReq) (resp *open.CommonRes, err error) {
	updates := make(map[string]interface{})
	if req.IsSetName() {
		updates["name"] = req.GetName()
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
	ok, err := s.openService.UpdateBot(ctx, req.AppId, req.TenantId, req.BotId, updates)
	if err != nil {
		klog.CtxErrorf(ctx, "更新开放平台Bot[%d]失败: %v", req.BotId, err)
		return nil, err
	}
	return &open.CommonRes{Success: ok}, nil
}

func (s *OpenServiceImpl) DeleteBot(ctx context.Context, req *open.DeleteOpenBotReq) (resp *open.CommonRes, err error) {
	ok, err := s.openService.DeleteBot(ctx, req.AppId, req.TenantId, req.BotId)
	if err != nil {
		klog.CtxErrorf(ctx, "删除开放平台Bot[%d]失败: %v", req.BotId, err)
		return nil, err
	}
	return &open.CommonRes{Success: ok}, nil
}

func (s *OpenServiceImpl) AddBotToConversation(ctx context.Context, req *open.AddOpenBotConversationReq) (resp *open.AddOpenBotConversationRes, err error) {
	convID, err := s.openService.AddBotToConversation(ctx, req.AppId, req.TenantId,
		req.BotId, req.OperatorId, req.ConversationId, req.ConversationType)
	if err != nil {
		klog.CtxErrorf(ctx, "开放平台Bot[%d]加入会话失败: %v", req.BotId, err)
		return nil, err
	}
	return &open.AddOpenBotConversationRes{Success: true, ConversationId: convID}, nil
}

// ===== 消息与会话 =====

func (s *OpenServiceImpl) SendMessage(ctx context.Context, req *open.OpenSendMessageReq) (resp *open.OpenSendMessageRes, err error) {
	sent, err := s.openService.SendMessage(ctx, req.AppId, req.TenantId, req.BotId,
		req.ConversationId, req.PeerId, req.Content)
	if err != nil {
		klog.CtxErrorf(ctx, "开放平台Bot[%d]发送消息失败: %v", req.BotId, err)
		return nil, err
	}
	return &open.OpenSendMessageRes{Success: true, MsgId: sent.MsgID, ConversationId: sent.ConversationID}, nil
}

func (s *OpenServiceImpl) ListMessages(ctx context.Context, req *open.ListOpenMessagesReq) (resp *open.ListOpenMessagesRes, err error) {
	msgs, err := s.openService.ListMessages(ctx, req.AppId, req.TenantId, req.BotId,
		req.ConversationId, req.BeforeMsgId, req.Limit)
	if err != nil {
		klog.CtxErrorf(ctx, "查询开放平台消息失败: %v", err)
		return nil, err
	}
	list := make([]*open.OpenMessage, 0, len(msgs))
	for _, m := range msgs {
		item := &open.OpenMessage{
			MsgId:          m.MsgID,
			SenderId:       m.SenderID,
			ConversationId: m.Conversation,
			Content:        m.Content,
			Timestamp:      m.Timestamp,
		}
		if m.SenderName != "" {
			name := m.SenderName
			item.SenderName = &name
		}
		list = append(list, item)
	}
	return &open.ListOpenMessagesRes{Success: true, Messages: list}, nil
}

func (s *OpenServiceImpl) ListConversations(ctx context.Context, req *open.ListOpenConversationsReq) (resp *open.ListOpenConversationsRes, err error) {
	convs, err := s.openService.ListConversations(ctx, req.AppId, req.TenantId, req.BotId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询开放平台会话列表失败: %v", err)
		return nil, err
	}
	list := make([]*open.OpenConversationInfo, 0, len(convs))
	for _, c := range convs {
		list = append(list, &open.OpenConversationInfo{
			ConversationId: c.ConversationID,
			Type:           c.Type,
			Name:           c.Name,
			MemberIds:      c.MemberIDs,
			TenantId:       c.TenantID,
		})
	}
	return &open.ListOpenConversationsRes{Success: true, Conversations: list}, nil
}

// ===== 事件 =====

func (s *OpenServiceImpl) ListEvents(ctx context.Context, req *open.ListEventsReq) (resp *open.ListEventsRes, err error) {
	events, err := s.openService.ListEvents(ctx, req.AppId, req.GetSinceId(), int(req.GetLimit()))
	if err != nil {
		klog.CtxErrorf(ctx, "查询应用[%s]事件失败: %v", req.AppId, err)
		return nil, err
	}
	list := make([]*open.OpenEventInfo, 0, len(events))
	for _, ev := range events {
		list = append(list, &open.OpenEventInfo{
			Id:        ev.ID,
			AppId:     ev.AppID,
			BotId:     ev.BotID,
			EventType: ev.EventType,
			Payload:   ev.Payload,
			Status:    ev.Status,
			CreatedAt: ev.CreatedAt.UnixMilli(),
		})
	}
	return &open.ListEventsRes{Success: true, Events: list}, nil
}

func (s *OpenServiceImpl) AckEvents(ctx context.Context, req *open.AckEventsReq) (resp *open.CommonRes, err error) {
	if err = s.openService.AckEvents(ctx, req.AppId, req.EventIds); err != nil {
		klog.CtxErrorf(ctx, "确认事件失败: %v", err)
		return nil, err
	}
	return &open.CommonRes{Success: true}, nil
}

func (s *OpenServiceImpl) RecordEvent(ctx context.Context, req *open.RecordEventReq) (resp *open.CommonRes, err error) {
	if err = s.openService.RecordBotEvent(ctx, req.BotId, req.EventType, req.Payload); err != nil {
		klog.CtxErrorf(ctx, "记录Bot事件失败, bot_id=%d, err=%v", req.BotId, err)
		return nil, err
	}
	return &open.CommonRes{Success: true}, nil
}

// ===== 回调 =====

func (s *OpenServiceImpl) RegisterWebhook(ctx context.Context, req *open.RegisterWebhookReq) (resp *open.RegisterWebhookRes, err error) {
	id, secret, challenge, err := s.openService.RegisterWebhook(ctx, req.AppId, req.BotId, req.Url, req.Events)
	if err != nil {
		klog.CtxErrorf(ctx, "注册回调失败, app_id=%s, err=%v", req.AppId, err)
		return nil, err
	}
	return &open.RegisterWebhookRes{Success: true, Id: id, Secret: secret, VerifyChallenge: challenge}, nil
}

func (s *OpenServiceImpl) ListWebhooks(ctx context.Context, req *open.ListWebhooksReq) (resp *open.ListWebhooksRes, err error) {
	hooks, err := s.openService.ListWebhooks(ctx, req.AppId, req.GetBotId())
	if err != nil {
		klog.CtxErrorf(ctx, "查询回调列表失败: %v", err)
		return nil, err
	}
	list := make([]*open.WebhookInfo, 0, len(hooks))
	for _, wh := range hooks {
		list = append(list, toWebhookInfo(wh))
	}
	return &open.ListWebhooksRes{Success: true, Webhooks: list}, nil
}

func (s *OpenServiceImpl) GetWebhook(ctx context.Context, req *open.GetWebhookReq) (resp *open.GetWebhookRes, err error) {
	wh, ok, err := s.openService.GetWebhook(ctx, req.AppId, req.Id)
	if err != nil {
		klog.CtxErrorf(ctx, "查询回调[%d]失败: %v", req.Id, err)
		return nil, err
	}
	if !ok {
		return &open.GetWebhookRes{}, nil
	}
	secret := wh.Secret
	return &open.GetWebhookRes{Success: true, Webhook: toWebhookInfo(wh), Secret: &secret}, nil
}

func (s *OpenServiceImpl) DeleteWebhook(ctx context.Context, req *open.DeleteWebhookReq) (resp *open.CommonRes, err error) {
	ok, err := s.openService.DeleteWebhook(ctx, req.AppId, req.Id)
	if err != nil {
		klog.CtxErrorf(ctx, "删除回调[%d]失败: %v", req.Id, err)
		return nil, err
	}
	return &open.CommonRes{Success: ok}, nil
}

func (s *OpenServiceImpl) RecordDelivery(ctx context.Context, req *open.RecordDeliveryReq) (resp *open.CommonRes, err error) {
	// 投递记录由 open_service 自身在回调时写入；外部调用仅用于测试/补记，这里保持幂等返回。
	return &open.CommonRes{Success: true}, nil
}

// ===== 映射 =====

func toAppInfo(dto service.AppDTO) *open.AppInfo {
	return &open.AppInfo{
		AppId:     dto.AppID,
		Name:      dto.Name,
		Scopes:    dto.Scopes,
		Status:    dto.Status,
		QpsLimit:  dto.QPSLimit,
		OwnerId:   dto.OwnerID,
		TenantId:  dto.TenantID,
		CreatedAt: dto.CreatedAt,
	}
}

func toOpenBotInfo(b openrpc.BotConfig) *open.OpenBotInfo {
	return &open.OpenBotInfo{
		BotId:        b.BotID,
		AppId:        b.AppID,
		CreatorId:    b.CreatorID,
		Name:         b.Name,
		SystemPrompt: b.SystemPrompt,
		Model:        b.Model,
		BaseUrl:      b.BaseURL,
		CreatedAt:    b.CreatedAt,
	}
}

func toWebhookInfo(wh model.OpenWebhook) *open.WebhookInfo {
	return &open.WebhookInfo{
		Id:        wh.ID,
		AppId:     wh.AppID,
		BotId:     wh.BotID,
		Url:       wh.URL,
		Events:    wh.Events,
		Enabled:   wh.Enabled,
		Status:    wh.Status,
		CreatedAt: wh.CreatedAt.UnixMilli(),
	}
}
