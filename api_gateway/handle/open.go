package handle

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Airiseina/answer/api_gateway/config"
	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"
	"github.com/Airiseina/answer/kitex_service/open_service/kitex_gen/open"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// ===== 站内应用管理（普通用户登录态） =====

type openCreateAppParam struct {
	Name     string `json:"name"`
	Scopes   string `json:"scopes"`
	QPSLimit int32  `json:"qps_limit"`
}

// OpenCreateApp 为当前登录用户创建一个开放平台应用，AppSecret 只在创建时返回一次。
func OpenCreateApp(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var param openCreateAppParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "创建开放平台应用参数解析失败, err=%v", err)
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	if strings.TrimSpace(param.Name) == "" {
		response.Error(c, "参数缺失", "应用名称不能为空")
		return
	}
	res, err := rpc.OpenCreateApp(ctx, &open.CreateAppReq{
		OwnerId:  identity.Id,
		TenantId: identity.TenantId,
		Name:     param.Name,
		Scopes:   param.Scopes,
		QpsLimit: &param.QPSLimit,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "创建开放平台应用RPC失败, owner_id=%d, err=%v", identity.Id, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "创建失败", "应用创建失败，请稍后重试")
		return
	}
	response.Success(c, map[string]interface{}{
		"app_id":     res.AppId,
		"app_secret": res.AppSecret,
		"notice":     "AppSecret 仅显示一次，请立即妥善保存",
	})
}

// OpenListApps 列出当前登录用户在所在租户下的应用。
func OpenListApps(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	res, err := rpc.OpenListApps(ctx, &open.ListAppsReq{
		OwnerId:  identity.Id,
		TenantId: identity.TenantId,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "查询开放平台应用列表RPC失败, owner_id=%d, err=%v", identity.Id, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	response.Success(c, toAppItems(res.Apps))
}

// OpenGetApp 查询单个应用详情。
func OpenGetApp(ctx context.Context, c *app.RequestContext) {
	appID := c.Query("app_id")
	if appID == "" {
		response.Error(c, "参数缺失", "请选择应用")
		return
	}
	res, err := rpc.OpenGetApp(ctx, &open.GetAppReq{AppId: appID})
	if err != nil {
		hlog.CtxErrorf(ctx, "查询开放平台应用RPC失败, app_id=%s, err=%v", appID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if res.App == nil {
		response.Error(c, "应用不存在", "请检查应用 ID")
		return
	}
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	if res.App.OwnerId != identity.Id {
		response.Error(c, "无权限", "仅应用创建者可查看该应用")
		return
	}
	response.Success(c, toAppItem(res.App))
}

type openAppIDParam struct {
	AppID string `json:"app_id"`
}

// OpenResetAppSecret 轮换应用密钥，新密钥只返回一次。
func OpenResetAppSecret(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var param openAppIDParam
	if err := c.BindJSON(&param); err != nil || param.AppID == "" {
		response.Error(c, "参数缺失", "请选择应用")
		return
	}
	if !ensureAppOwner(ctx, c, identity.Id, param.AppID) {
		return
	}
	res, err := rpc.OpenResetAppSecret(ctx, &open.ResetAppSecretReq{
		AppId:      param.AppID,
		OperatorId: identity.Id,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "轮换应用密钥RPC失败, app_id=%s, err=%v", param.AppID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "应用不存在")
		return
	}
	response.Success(c, map[string]interface{}{
		"app_id":     param.AppID,
		"app_secret": res.AppSecret,
		"notice":     "新 AppSecret 仅显示一次，旧密钥立即失效",
	})
}

type openAppStatusParam struct {
	AppID  string `json:"app_id"`
	Status int16  `json:"status"`
}

// OpenUpdateAppStatus 启用或停用应用（0 启用 / 1 停用）。
func OpenUpdateAppStatus(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var param openAppStatusParam
	if err := c.BindJSON(&param); err != nil || param.AppID == "" {
		response.Error(c, "参数缺失", "请选择应用")
		return
	}
	if param.Status != 0 && param.Status != 1 {
		response.Error(c, "参数错误", "状态取值仅支持 0（启用）或 1（停用）")
		return
	}
	if !ensureAppOwner(ctx, c, identity.Id, param.AppID) {
		return
	}
	res, err := rpc.OpenUpdateAppStatus(ctx, &open.UpdateAppStatusReq{
		AppId:      param.AppID,
		OperatorId: identity.Id,
		Status:     param.Status,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "更新应用状态RPC失败, app_id=%s, err=%v", param.AppID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "应用不存在")
		return
	}
	response.Success(c, "更新成功")
}

// ensureAppOwner 校验应用归属，避免站内用户操作他人的应用。
func ensureAppOwner(ctx context.Context, c *app.RequestContext, ownerID int64, appID string) bool {
	res, err := rpc.OpenGetApp(ctx, &open.GetAppReq{AppId: appID})
	if err != nil {
		hlog.CtxErrorf(ctx, "校验应用归属RPC失败, app_id=%s, err=%v", appID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return false
	}
	if res.App == nil || res.App.OwnerId != ownerID {
		response.Error(c, "无权限", "仅应用创建者可执行该操作")
		return false
	}
	return true
}

func toAppItem(a *open.AppInfo) map[string]interface{} {
	return map[string]interface{}{
		"app_id":     a.AppId,
		"name":       a.Name,
		"scopes":     a.Scopes,
		"status":     a.Status,
		"qps_limit":  a.QpsLimit,
		"owner_id":   strconv.FormatInt(a.OwnerId, 10),
		"tenant_id":  strconv.FormatInt(a.TenantId, 10),
		"created_at": a.CreatedAt,
	}
}

func toAppItems(apps []*open.AppInfo) []map[string]interface{} {
	list := make([]map[string]interface{}, 0, len(apps))
	for _, a := range apps {
		list = append(list, toAppItem(a))
	}
	return list
}

// ===== 开放平台接口（签名鉴权） =====

// currentOpenApp 取出签名校验后的应用信息，缺失时直接返回错误响应。
func currentOpenApp(c *app.RequestContext) (*middleware.OpenAppIdentity, bool) {
	identity, ok := middleware.CurrentOpenApp(c)
	if !ok {
		response.Error(c, "鉴权失败", "请先完成开放平台签名校验")
		return nil, false
	}
	return identity, true
}

type openCreateBotParam struct {
	Name         string `json:"name"`
	SystemPrompt string `json:"system_prompt"`
	APIKey       string `json:"api_key"`
	Model        string `json:"model"`
	BaseURL      string `json:"base_url"`
}

// OpenCreateBot 以第三方应用名义创建 Bot；Bot 归属应用所在租户，且不计入用量统计。
func OpenCreateBot(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	var param openCreateBotParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "开放平台创建Bot参数解析失败, err=%v", err)
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	if strings.TrimSpace(param.Name) == "" || strings.TrimSpace(param.APIKey) == "" || strings.TrimSpace(param.Model) == "" {
		response.Error(c, "参数缺失", "name、api_key、model 均不能为空")
		return
	}
	res, err := rpc.OpenCreateBot(ctx, &open.CreateOpenBotReq{
		AppId:        identity.AppID,
		TenantId:     identity.TenantID,
		CreatorId:    identity.OwnerID,
		Name:         param.Name,
		SystemPrompt: param.SystemPrompt,
		ApiKey:       param.APIKey,
		Model:        param.Model,
		BaseUrl:      &param.BaseURL,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台创建BotRPC失败, app_id=%s, err=%v", identity.AppID, err)
		response.Error(c, "创建失败", "Bot 创建失败，请稍后重试")
		return
	}
	response.Success(c, map[string]interface{}{
		"bot_id": strconv.FormatInt(res.BotId, 10),
	})
}

func OpenListBots(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	res, err := rpc.OpenListBots(ctx, &open.ListOpenBotsReq{
		AppId:    identity.AppID,
		TenantId: identity.TenantID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台查询Bot列表RPC失败, app_id=%s, err=%v", identity.AppID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	list := make([]map[string]interface{}, 0, len(res.Bots))
	for _, b := range res.Bots {
		list = append(list, toOpenBotItem(b))
	}
	response.Success(c, list)
}

func OpenGetBot(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	botID, ok := parseOpenPathID(c, "bot_id")
	if !ok {
		return
	}
	res, err := rpc.OpenGetBot(ctx, &open.GetOpenBotReq{
		AppId:    identity.AppID,
		BotId:    botID,
		TenantId: identity.TenantID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台查询BotRPC失败, bot_id=%d, err=%v", botID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if res.Bot == nil {
		response.Error(c, "Bot不存在", "请检查 bot_id 是否属于当前应用")
		return
	}
	response.Success(c, toOpenBotItem(res.Bot))
}

type openUpdateBotParam struct {
	Name         *string `json:"name"`
	SystemPrompt *string `json:"system_prompt"`
	APIKey       *string `json:"api_key"`
	Model        *string `json:"model"`
	BaseURL      *string `json:"base_url"`
}

func OpenUpdateBot(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	botID, ok := parseOpenPathID(c, "bot_id")
	if !ok {
		return
	}
	var param openUpdateBotParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "开放平台更新Bot参数解析失败, err=%v", err)
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	res, err := rpc.OpenUpdateBot(ctx, &open.UpdateOpenBotReq{
		AppId:        identity.AppID,
		BotId:        botID,
		TenantId:     identity.TenantID,
		Name:         param.Name,
		SystemPrompt: param.SystemPrompt,
		ApiKey:       param.APIKey,
		Model:        param.Model,
		BaseUrl:      param.BaseURL,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台更新BotRPC失败, bot_id=%d, err=%v", botID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "Bot 不存在或不属于当前应用")
		return
	}
	response.Success(c, "更新成功")
}

func OpenDeleteBot(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	botID, ok := parseOpenPathID(c, "bot_id")
	if !ok {
		return
	}
	res, err := rpc.OpenDeleteBot(ctx, &open.DeleteOpenBotReq{
		AppId:    identity.AppID,
		BotId:    botID,
		TenantId: identity.TenantID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台删除BotRPC失败, bot_id=%d, err=%v", botID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "Bot 不存在或不属于当前应用")
		return
	}
	response.Success(c, "删除成功")
}

type openAddBotConversationParam struct {
	ConversationID   int64 `json:"conversation_id"`
	ConversationType int16 `json:"conversation_type"`
}

// OpenAddBotToConversation 把 Bot 拉入指定会话（私聊 conversation_type=1 / 群聊 2）。
func OpenAddBotToConversation(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	botID, ok := parseOpenPathID(c, "bot_id")
	if !ok {
		return
	}
	var param openAddBotConversationParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "开放平台拉Bot入会话参数解析失败, err=%v", err)
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	if param.ConversationType != 1 && param.ConversationType != 2 {
		response.Error(c, "参数错误", "conversation_type 仅支持 1（私聊）或 2（群聊）")
		return
	}
	res, err := rpc.OpenAddBotToConversation(ctx, &open.AddOpenBotConversationReq{
		AppId:            identity.AppID,
		BotId:            botID,
		OperatorId:       identity.OwnerID,
		ConversationId:   param.ConversationID,
		ConversationType: param.ConversationType,
		TenantId:         identity.TenantID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台拉Bot入会话RPC失败, bot_id=%d, err=%v", botID, err)
		response.Error(c, "操作失败", "Bot 不存在、不属于当前应用或会话不可用")
		return
	}
	response.Success(c, map[string]interface{}{
		"conversation_id": strconv.FormatInt(res.ConversationId, 10),
	})
}

type openSendMessageParam struct {
	BotID          int64  `json:"bot_id"`
	ConversationID int64  `json:"conversation_id"`
	PeerID         int64  `json:"peer_id"`
	Content        string `json:"content"`
}

// OpenSendMessage 以 Bot 身份发送消息。
func OpenSendMessage(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	var param openSendMessageParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "开放平台发送消息参数解析失败, err=%v", err)
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	if param.BotID == 0 || strings.TrimSpace(param.Content) == "" {
		response.Error(c, "参数缺失", "bot_id 与 content 不能为空")
		return
	}
	res, err := rpc.OpenSendMessage(ctx, &open.OpenSendMessageReq{
		AppId:          identity.AppID,
		BotId:          param.BotID,
		ConversationId: param.ConversationID,
		PeerId:         param.PeerID,
		Content:        param.Content,
		TenantId:       identity.TenantID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台发送消息RPC失败, bot_id=%d, err=%v", param.BotID, err)
		response.Error(c, "发送失败", "Bot 不存在或会话不可用")
		return
	}
	response.Success(c, map[string]interface{}{
		"msg_id":          strconv.FormatInt(res.MsgId, 10),
		"conversation_id": strconv.FormatInt(res.ConversationId, 10),
	})
}

// OpenListMessages 拉取指定会话的历史消息，before_msg_id 为游标。
func OpenListMessages(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	botID, _ := strconv.ParseInt(c.Query("bot_id"), 10, 64)
	conversationID, _ := strconv.ParseInt(c.Query("conversation_id"), 10, 64)
	beforeMsgID, _ := strconv.ParseInt(c.Query("before_msg_id"), 10, 64)
	limit, _ := strconv.ParseInt(c.Query("limit"), 10, 16)
	if botID == 0 || conversationID == 0 {
		response.Error(c, "参数缺失", "bot_id 与 conversation_id 不能为空")
		return
	}
	res, err := rpc.OpenListMessages(ctx, &open.ListOpenMessagesReq{
		AppId:          identity.AppID,
		BotId:          botID,
		ConversationId: conversationID,
		BeforeMsgId:    beforeMsgID,
		Limit:          int16(limit),
		TenantId:       identity.TenantID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台查询消息RPC失败, bot_id=%d, err=%v", botID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	list := make([]map[string]interface{}, 0, len(res.Messages))
	for _, m := range res.Messages {
		list = append(list, map[string]interface{}{
			"msg_id":          strconv.FormatInt(m.MsgId, 10),
			"sender_id":       strconv.FormatInt(m.SenderId, 10),
			"conversation_id": strconv.FormatInt(m.ConversationId, 10),
			"content":         m.Content,
			"timestamp":       m.Timestamp,
			"sender_name":     m.GetSenderName(),
		})
	}
	response.Success(c, list)
}

// OpenListConversations 列出 Bot 参与的会话。
func OpenListConversations(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	botID, _ := strconv.ParseInt(c.Query("bot_id"), 10, 64)
	if botID == 0 {
		response.Error(c, "参数缺失", "bot_id 不能为空")
		return
	}
	res, err := rpc.OpenListConversations(ctx, &open.ListOpenConversationsReq{
		AppId:    identity.AppID,
		BotId:    botID,
		TenantId: identity.TenantID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台查询会话列表RPC失败, bot_id=%d, err=%v", botID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	list := make([]map[string]interface{}, 0, len(res.Conversations))
	for _, conv := range res.Conversations {
		members := make([]string, 0, len(conv.MemberIds))
		for _, id := range conv.MemberIds {
			members = append(members, strconv.FormatInt(id, 10))
		}
		list = append(list, map[string]interface{}{
			"conversation_id": strconv.FormatInt(conv.ConversationId, 10),
			"type":            conv.Type,
			"name":            conv.Name,
			"member_ids":      members,
		})
	}
	response.Success(c, list)
}

// OpenListEvents 轮询拉取事件，since_id 为游标。
func OpenListEvents(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	sinceID, _ := strconv.ParseInt(c.Query("since_id"), 10, 64)
	limit, _ := strconv.ParseInt(c.Query("limit"), 10, 32)
	if limit <= 0 {
		limit = 50
	}
	limit32 := int32(limit)
	res, err := rpc.OpenListEvents(ctx, &open.ListEventsReq{
		AppId:   identity.AppID,
		SinceId: &sinceID,
		Limit:   &limit32,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台查询事件RPC失败, app_id=%s, err=%v", identity.AppID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	list := make([]map[string]interface{}, 0, len(res.Events))
	for _, ev := range res.Events {
		list = append(list, map[string]interface{}{
			"event_id":   strconv.FormatInt(ev.Id, 10),
			"bot_id":     strconv.FormatInt(ev.BotId, 10),
			"event_type": ev.EventType,
			"payload":    ev.Payload,
			"status":     ev.Status,
			"created_at": ev.CreatedAt,
		})
	}
	response.Success(c, list)
}

type openAckEventsParam struct {
	EventIDs []int64 `json:"event_ids"`
}

// OpenAckEvents 确认事件已处理，避免重复投递。
func OpenAckEvents(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	var param openAckEventsParam
	if err := c.BindJSON(&param); err != nil || len(param.EventIDs) == 0 {
		response.Error(c, "参数缺失", "event_ids 不能为空")
		return
	}
	res, err := rpc.OpenAckEvents(ctx, &open.AckEventsReq{
		AppId:    identity.AppID,
		EventIds: param.EventIDs,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台确认事件RPC失败, app_id=%s, err=%v", identity.AppID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "确认事件失败")
		return
	}
	response.Success(c, "确认成功")
}

type openRegisterWebhookParam struct {
	BotID  int64  `json:"bot_id"`
	URL    string `json:"url"`
	Events string `json:"events"`
}

// OpenRegisterWebhook 注册回调地址，返回用于验签的 webhook secret 与挑战串。
func OpenRegisterWebhook(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	var param openRegisterWebhookParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "开放平台注册回调参数解析失败, err=%v", err)
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	if param.BotID == 0 || strings.TrimSpace(param.URL) == "" {
		response.Error(c, "参数缺失", "bot_id 与 url 不能为空")
		return
	}
	res, err := rpc.OpenRegisterWebhook(ctx, &open.RegisterWebhookReq{
		AppId:  identity.AppID,
		BotId:  param.BotID,
		Url:    param.URL,
		Events: param.Events,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台注册回调RPC失败, app_id=%s, err=%v", identity.AppID, err)
		response.Error(c, "注册失败", "回调注册失败，请检查地址与事件订阅")
		return
	}
	response.Success(c, map[string]interface{}{
		"id":               strconv.FormatInt(res.Id, 10),
		"secret":           res.Secret,
		"verify_challenge": res.VerifyChallenge,
		"notice":           "请在回调地址上原样回显 verify_challenge 完成地址校验",
	})
}

func OpenListWebhooks(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	// bot_id 为可选过滤条件；传 0 或省略时返回该应用下全部回调。
	botID, _ := strconv.ParseInt(c.Query("bot_id"), 10, 64)
	res, err := rpc.OpenListWebhooks(ctx, &open.ListWebhooksReq{
		AppId: identity.AppID,
		BotId: &botID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台查询回调列表RPC失败, app_id=%s, err=%v", identity.AppID, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	list := make([]map[string]interface{}, 0, len(res.Webhooks))
	for _, wh := range res.Webhooks {
		list = append(list, toWebhookItem(wh))
	}
	response.Success(c, list)
}

func OpenGetWebhook(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	id, ok := parseOpenPathID(c, "id")
	if !ok {
		return
	}
	res, err := rpc.OpenGetWebhook(ctx, &open.GetWebhookReq{AppId: identity.AppID, Id: id})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台查询回调RPC失败, id=%d, err=%v", id, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if res.Webhook == nil {
		response.Error(c, "回调不存在", "请检查回调 ID")
		return
	}
	item := toWebhookItem(res.Webhook)
	item["secret"] = res.GetSecret()
	response.Success(c, item)
}

func OpenDeleteWebhook(ctx context.Context, c *app.RequestContext) {
	identity, ok := currentOpenApp(c)
	if !ok {
		return
	}
	id, ok := parseOpenPathID(c, "id")
	if !ok {
		return
	}
	res, err := rpc.OpenDeleteWebhook(ctx, &open.DeleteWebhookReq{AppId: identity.AppID, Id: id})
	if err != nil {
		hlog.CtxErrorf(ctx, "开放平台删除回调RPC失败, id=%d, err=%v", id, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !res.Success {
		response.Error(c, "操作失败", "回调不存在")
		return
	}
	response.Success(c, "删除成功")
}

func toOpenBotItem(b *open.OpenBotInfo) map[string]interface{} {
	return map[string]interface{}{
		"bot_id":        strconv.FormatInt(b.BotId, 10),
		"app_id":        b.AppId,
		"name":          b.Name,
		"system_prompt": b.SystemPrompt,
		"model":         b.Model,
		"base_url":      b.BaseUrl,
		"created_at":    b.CreatedAt,
	}
}

func toWebhookItem(wh *open.WebhookInfo) map[string]interface{} {
	return map[string]interface{}{
		"id":         strconv.FormatInt(wh.Id, 10),
		"bot_id":     strconv.FormatInt(wh.BotId, 10),
		"url":        wh.Url,
		"events":     wh.Events,
		"enabled":    wh.Enabled,
		"status":     wh.Status,
		"created_at": wh.CreatedAt,
	}
}

// parseOpenPathID 解析路径中的整型 ID，失败时直接写出错误响应。
func parseOpenPathID(c *app.RequestContext, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, "参数错误", name+" 非法")
		return 0, false
	}
	return id, true
}

// resolveOpenSpecPath 依次尝试配置路径、进程工作目录与可执行文件同级目录，
// 避免部署时因启动目录不同导致规范文件读取失败。
func resolveOpenSpecPath() string {
	candidates := make([]string, 0, 4)
	if p := config.V.GetString("open.docs_path"); p != "" {
		candidates = append(candidates, p)
	}
	candidates = append(candidates, filepath.Join("docs", "openapi", "open_platform.yaml"))
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "docs", "openapi", "open_platform.yaml"),
			filepath.Join(dir, "..", "docs", "openapi", "open_platform.yaml"),
		)
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return filepath.Join("docs", "openapi", "open_platform.yaml")
}

// OpenDocs 返回可浏览的开放平台接口文档（Redoc 渲染本地 OpenAPI 规范）。
func OpenDocs(ctx context.Context, c *app.RequestContext) {
	specPath := resolveOpenSpecPath()
	if _, err := os.Stat(specPath); err != nil {
		response.Error(c, "文档不可用", "未找到 OpenAPI 规范文件："+specPath)
		return
	}
	c.Data(200, "text/html; charset=utf-8", []byte(openDocsHTML))
}

// OpenAPISpec 直接下发 OpenAPI 规范原文，方便第三方导入到 Postman / Apifox。
func OpenAPISpec(ctx context.Context, c *app.RequestContext) {
	specPath := resolveOpenSpecPath()
	raw, err := os.ReadFile(specPath)
	if err != nil {
		hlog.CtxErrorf(ctx, "读取OpenAPI规范失败, path=%s, err=%v", specPath, err)
		response.Error(c, "文档不可用", "未找到 OpenAPI 规范文件："+specPath)
		return
	}
	c.Data(200, "application/yaml; charset=utf-8", raw)
}

const openDocsHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <title>开放平台接口文档</title>
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <redoc spec-url="/open/openapi.yaml"></redoc>
  <script src="https://cdn.jsdelivr.net/npm/redoc@next/bundles/redoc.standalone.js"></script>
</body>
</html>`
