package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Airiseina/answer/kitex_service/open_service/internal/config"
	"github.com/Airiseina/answer/kitex_service/open_service/internal/dal/mysql"
	"github.com/Airiseina/answer/kitex_service/open_service/internal/model"
	openrpc "github.com/Airiseina/answer/kitex_service/open_service/rpc"
	"github.com/Airiseina/answer/pkg/audit"
	"github.com/Airiseina/answer/pkg/snowflake"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/redis/go-redis/v9"
)

// 鉴权失败原因（对外文档需同步）。
const (
	ErrCodeOK               = "ok"
	ErrCodeBadSignature     = "bad_signature"
	ErrCodeTimestampExpired = "timestamp_expired"
	ErrCodeReplay           = "replay"
	ErrCodeForbidden        = "forbidden"
)

// 回调地址校验状态。
const (
	WebhookStatusPending  = "pending"
	WebhookStatusVerified = "verified"
	WebhookStatusFailed   = "failed"
)

// 内置作用域。
var AllScopes = []string{
	"bot:read", "bot:write", "mcp:manage", "knowledge:manage",
	"message:send", "message:read", "conversation:read", "webhook:manage",
}

// AppDTO 是服务层对外的应用视图。
type AppDTO struct {
	AppID     string
	Name      string
	Scopes    string
	Status    int16
	QPSLimit  int32
	OwnerID   int64
	TenantID  int64
	CreatedAt int64
}

// OpenService 实现开放平台的应用管理、Bot 代理、事件与回调逻辑。
//
// 说明：Bot / 会话 / 消息本身仍归属 bot_service 与 chat_service，
// 这里只做归属校验与参数编排，不重复持有业务数据。
type OpenService struct {
	dao      mysql.OpenDao
	audit    audit.Recorder
	rdb      *redis.Client
	snowNode *snowflake.Node

	timestampWindow    int64
	nonceTTL           time.Duration
	defaultQPS         int32
	maxBotsPerApp      int
	webhookTimeout     time.Duration
	webhookMaxAttempts int32
	webhookBackoff     []int
	eventRetentionDays int
	cipherKey          []byte
	httpClient         *http.Client
}

func NewOpenService(dao mysql.OpenDao, recorder audit.Recorder, rdb *redis.Client) *OpenService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	v := config.V
	svc := &OpenService{
		dao:                dao,
		audit:              recorder,
		rdb:                rdb,
		snowNode:           snowflake.NewNode(snowflake.ShardOpen),
		timestampWindow:    int64(v.GetInt("open.timestamp_window")),
		nonceTTL:           time.Duration(v.GetInt("open.nonce_ttl")) * time.Second,
		defaultQPS:         int32(v.GetInt("open.default_qps")),
		maxBotsPerApp:      v.GetInt("open.max_bots_per_app"),
		webhookTimeout:     time.Duration(v.GetInt("open.webhook_timeout")) * time.Second,
		webhookMaxAttempts: int32(v.GetInt("open.webhook_max_attempts")),
		webhookBackoff:     v.GetIntSlice("open.webhook_retry_backoff"),
		eventRetentionDays: v.GetInt("open.event_retention_days"),
		cipherKey:          cipherKeyFrom(v.GetString("open.app_secret_key")),
	}
	if svc.timestampWindow <= 0 {
		svc.timestampWindow = 300
	}
	if svc.nonceTTL <= 0 {
		svc.nonceTTL = 5 * time.Minute
	}
	if svc.defaultQPS <= 0 {
		svc.defaultQPS = 20
	}
	if svc.maxBotsPerApp <= 0 {
		svc.maxBotsPerApp = 50
	}
	if svc.webhookTimeout <= 0 {
		svc.webhookTimeout = 10 * time.Second
	}
	if svc.webhookMaxAttempts <= 0 {
		svc.webhookMaxAttempts = 5
	}
	if len(svc.webhookBackoff) == 0 {
		svc.webhookBackoff = []int{1, 5, 30, 120}
	}
	if svc.eventRetentionDays <= 0 {
		svc.eventRetentionDays = 30
	}
	svc.httpClient = &http.Client{Timeout: svc.webhookTimeout}
	return svc
}

// ===== 应用管理 =====

// CreateApp 生成 AppID 与 AppSecret；明文密钥仅在此处返回一次。
func (s *OpenService) CreateApp(ctx context.Context, ownerID, tenantID int64, name, scopes string, qpsLimit *int32) (AppDTO, string, error) {
	name = strings.TrimSpace(name)
	if ownerID <= 0 || name == "" {
		return AppDTO{}, "", nil
	}
	scopes = normalizeScopes(scopes)
	if scopes == "" {
		scopes = strings.Join(AllScopes, ",")
	}
	random, err := randomHex(12)
	if err != nil {
		return AppDTO{}, "", err
	}
	appID := "app_" + random
	secret, err := randomHex(32)
	if err != nil {
		return AppDTO{}, "", err
	}
	cipherText, err := encryptSecret(s.cipherKey, secret)
	if err != nil {
		return AppDTO{}, "", err
	}
	qps := s.defaultQPS
	if qpsLimit != nil && *qpsLimit > 0 {
		qps = *qpsLimit
	}
	app := &model.OpenApp{
		ID:              s.snowNode.Generate(),
		TenantID:        tenantID,
		OwnerID:         ownerID,
		Name:            name,
		AppID:           appID,
		AppSecretHash:   sha256Hex(secret),
		AppSecretCipher: cipherText,
		Scopes:          scopes,
		Status:          model.AppStatusEnabled,
		QPSLimit:        qps,
	}
	if err = s.dao.CreateApp(app); err != nil {
		return AppDTO{}, "", err
	}
	_ = s.audit.Record(ctx, audit.Event{
		EventType:  audit.EventOpenAPIRequest,
		ActorType:  audit.ActorUser,
		ActorID:    ownerID,
		TenantID:   tenantID,
		Action:     "create_app",
		TargetType: "open_app",
		TargetID:   appID,
		Detail:     "创建开放平台应用，作用域=" + scopes,
		Result:     audit.ResultSuccess,
	})
	return toAppDTO(*app), secret, nil
}

func (s *OpenService) ListApps(ctx context.Context, ownerID, tenantID int64) ([]AppDTO, error) {
	apps, err := s.dao.ListApps(ownerID, tenantID)
	if err != nil {
		return nil, err
	}
	list := make([]AppDTO, 0, len(apps))
	for _, a := range apps {
		list = append(list, toAppDTO(a))
	}
	return list, nil
}

// GetApp 返回应用视图与密钥哈希（哈希用于让调用方判断密钥是否已轮换，不泄露明文）。
func (s *OpenService) GetApp(ctx context.Context, appID string) (AppDTO, string, bool, error) {
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return AppDTO{}, "", false, err
	}
	if app.AppID == "" {
		return AppDTO{}, "", false, nil
	}
	return toAppDTO(app), app.AppSecretHash, true, nil
}

// VerifyApp 直接校验一对 AppID + AppSecret（无签名场景，如后台自检）。
func (s *OpenService) VerifyApp(ctx context.Context, appID, secret string) (AppDTO, bool, error) {
	if appID == "" || secret == "" {
		return AppDTO{}, false, nil
	}
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return AppDTO{}, false, err
	}
	if app.AppID == "" {
		return AppDTO{}, false, nil
	}
	if !secureCompare(app.AppSecretHash, sha256Hex(secret)) {
		return AppDTO{}, false, nil
	}
	return toAppDTO(app), true, nil
}

// AuthenticateApp 是 open_auth 中间件唯一的鉴权入口：
// 校验应用状态、时间戳窗口、nonce 防重放、HMAC 签名。
func (s *OpenService) AuthenticateApp(ctx context.Context, appID, timestamp, nonce, method, path, bodySHA256, signature string) (AppDTO, string, bool, error) {
	if appID == "" || signature == "" {
		return AppDTO{}, ErrCodeBadSignature, false, nil
	}
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return AppDTO{}, "", false, err
	}
	if app.AppID == "" {
		return AppDTO{}, ErrCodeForbidden, false, nil
	}
	if app.Status != model.AppStatusEnabled {
		return AppDTO{}, ErrCodeForbidden, false, nil
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return AppDTO{}, ErrCodeBadSignature, false, nil
	}
	if now := time.Now().Unix(); ts > now+s.timestampWindow || ts < now-s.timestampWindow {
		return AppDTO{}, ErrCodeTimestampExpired, false, nil
	}
	secret, err := decryptSecret(s.cipherKey, app.AppSecretCipher)
	if err != nil {
		klog.Errorf("开放平台应用[%s]密钥解密失败: %v", appID, err)
		return AppDTO{}, ErrCodeForbidden, false, nil
	}
	expect := hmacSHA256Hex(secret, StringToSign(appID, timestamp, nonce, method, path, bodySHA256))
	if !secureCompare(expect, strings.ToLower(signature)) {
		return AppDTO{}, ErrCodeBadSignature, false, nil
	}
	if nonce != "" && s.rdb != nil {
		key := "open:nonce:" + appID + ":" + nonce
		ok, setErr := s.rdb.SetNX(ctx, key, 1, s.nonceTTL).Result()
		if setErr != nil {
			// Redis 故障时放行会导致重放风险，这里明确拒绝并记录。
			klog.Errorf("开放平台nonce去重失败, app_id=%s, err=%v", appID, setErr)
			return AppDTO{}, ErrCodeReplay, false, nil
		}
		if !ok {
			return AppDTO{}, ErrCodeReplay, false, nil
		}
	}
	return toAppDTO(app), ErrCodeOK, true, nil
}

// ResetAppSecret 轮换密钥，返回新的明文密钥一次。
func (s *OpenService) ResetAppSecret(ctx context.Context, appID string, operatorID int64) (string, bool, error) {
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return "", false, err
	}
	if app.AppID == "" {
		return "", false, nil
	}
	secret, err := randomHex(32)
	if err != nil {
		return "", false, err
	}
	cipherText, err := encryptSecret(s.cipherKey, secret)
	if err != nil {
		return "", false, err
	}
	if err = s.dao.UpdateAppSecret(appID, sha256Hex(secret), cipherText); err != nil {
		return "", false, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		EventType:  audit.EventOpenAPIRequest,
		ActorType:  audit.ActorUser,
		ActorID:    operatorID,
		TenantID:   app.TenantID,
		Action:     "reset_app_secret",
		TargetType: "open_app",
		TargetID:   appID,
		Detail:     "轮换应用密钥",
		Result:     audit.ResultSuccess,
	})
	return secret, true, nil
}

func (s *OpenService) UpdateAppStatus(ctx context.Context, appID string, operatorID int64, status int16) (bool, error) {
	if status != model.AppStatusEnabled && status != model.AppStatusDisabled {
		return false, nil
	}
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return false, err
	}
	if app.AppID == "" {
		return false, nil
	}
	if err = s.dao.UpdateAppStatus(appID, status); err != nil {
		return false, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		EventType:  audit.EventOpenAPIRequest,
		ActorType:  audit.ActorUser,
		ActorID:    operatorID,
		TenantID:   app.TenantID,
		Action:     "update_app_status",
		TargetType: "open_app",
		TargetID:   appID,
		Detail:     fmt.Sprintf("更新应用状态为%d", status),
		Result:     audit.ResultSuccess,
	})
	return true, nil
}

// ===== Bot 管理 =====

// CreateBot 以开放平台应用的名义创建 Bot，归属应用所在租户，origin 固定为 openapi。
func (s *OpenService) CreateBot(ctx context.Context, appID string, tenantID, creatorID int64, name, systemPrompt, apiKey, modelName, baseURL string) (int64, error) {
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return 0, err
	}
	if app.AppID == "" || app.Status != model.AppStatusEnabled {
		return 0, fmt.Errorf("应用不存在或已停用")
	}
	if name == "" || apiKey == "" || modelName == "" {
		return 0, fmt.Errorf("name、api_key、model 不能为空")
	}
	bots, err := openrpc.ListUserBots(ctx, app.TenantID, app.OwnerID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, b := range bots {
		if b.AppID == appID {
			count++
		}
	}
	if count >= s.maxBotsPerApp {
		return 0, fmt.Errorf("单应用Bot数量已达上限(%d)", s.maxBotsPerApp)
	}
	botID, err := openrpc.CreateBot(ctx, app.TenantID, app.OwnerID, name, systemPrompt, apiKey, modelName, baseURL,
		model.BotOriginOpenAPI, appID)
	if err != nil {
		return 0, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		EventType:  audit.EventOpenAPIRequest,
		ActorType:  audit.ActorApp,
		TenantID:   app.TenantID,
		Action:     "create_bot",
		TargetType: "bot",
		TargetID:   strconv.FormatInt(botID, 10),
		Detail:     "开放平台创建Bot, app_id=" + appID,
		Result:     audit.ResultSuccess,
	})
	return botID, nil
}

// ListBots 只返回归属该应用的 Bot，避免开放平台越权看到站内 Bot。
func (s *OpenService) ListBots(ctx context.Context, appID string, tenantID int64) ([]openrpc.BotConfig, error) {
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return nil, err
	}
	if app.AppID == "" {
		return nil, fmt.Errorf("应用不存在")
	}
	bots, err := openrpc.ListUserBots(ctx, app.TenantID, app.OwnerID)
	if err != nil {
		return nil, err
	}
	list := make([]openrpc.BotConfig, 0, len(bots))
	for _, b := range bots {
		if b.AppID == appID {
			list = append(list, b)
		}
	}
	return list, nil
}

func (s *OpenService) GetBot(ctx context.Context, appID string, tenantID, botID int64) (openrpc.BotConfig, bool, error) {
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return openrpc.BotConfig{}, false, err
	}
	if app.AppID == "" {
		return openrpc.BotConfig{}, false, nil
	}
	botInfo, err := openrpc.GetBot(ctx, app.TenantID, botID)
	if err != nil {
		return openrpc.BotConfig{}, false, err
	}
	if botInfo.BotID == 0 || botInfo.AppID != appID || botInfo.Origin != model.BotOriginOpenAPI {
		return openrpc.BotConfig{}, false, nil
	}
	return botInfo, true, nil
}

func (s *OpenService) UpdateBot(ctx context.Context, appID string, tenantID, botID int64, updates map[string]interface{}) (bool, error) {
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return false, err
	}
	if app.AppID == "" {
		return false, nil
	}
	if _, ok, err := s.GetBot(ctx, appID, tenantID, botID); err != nil || !ok {
		return false, err
	}
	return openrpc.UpdateBot(ctx, app.TenantID, botID, app.OwnerID, updates)
}

func (s *OpenService) DeleteBot(ctx context.Context, appID string, tenantID, botID int64) (bool, error) {
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return false, err
	}
	if app.AppID == "" {
		return false, nil
	}
	if _, ok, err := s.GetBot(ctx, appID, tenantID, botID); err != nil || !ok {
		return false, err
	}
	return openrpc.DeleteBot(ctx, app.TenantID, botID, app.OwnerID)
}

func (s *OpenService) AddBotToConversation(ctx context.Context, appID string, tenantID, botID, operatorID, conversationID int64, convType int16) (int64, error) {
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return 0, err
	}
	if app.AppID == "" {
		return 0, fmt.Errorf("应用不存在")
	}
	if _, ok, err := s.GetBot(ctx, appID, tenantID, botID); err != nil || !ok {
		return 0, fmt.Errorf("Bot不存在或不属于该应用")
	}
	return openrpc.AddBotToConversation(ctx, app.TenantID, app.OwnerID, botID, conversationID, convType)
}

// ===== 消息与会话 =====

// SendMessage 以 Bot 自身的用户身份发送消息（而非应用所有者），保证会话成员一致性。
func (s *OpenService) SendMessage(ctx context.Context, appID string, tenantID, botID, conversationID, peerID int64, content string) (openrpc.SentMessage, error) {
	if strings.TrimSpace(content) == "" {
		return openrpc.SentMessage{}, fmt.Errorf("content 不能为空")
	}
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return openrpc.SentMessage{}, err
	}
	if app.AppID == "" {
		return openrpc.SentMessage{}, fmt.Errorf("应用不存在")
	}
	if _, ok, err := s.GetBot(ctx, appID, tenantID, botID); err != nil || !ok {
		return openrpc.SentMessage{}, fmt.Errorf("Bot不存在或不属于该应用")
	}
	cfg, err := openrpc.GetBotConfig(ctx, app.TenantID, botID)
	if err != nil {
		return openrpc.SentMessage{}, err
	}
	if cfg.UserID == 0 {
		return openrpc.SentMessage{}, fmt.Errorf("Bot用户记录缺失")
	}
	return openrpc.SendMessage(ctx, app.TenantID, cfg.UserID, conversationID, peerID, content)
}

func (s *OpenService) ListMessages(ctx context.Context, appID string, tenantID, botID, conversationID, beforeMsgID int64, limit int16) ([]openrpc.MessageItem, error) {
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return nil, err
	}
	if app.AppID == "" {
		return nil, fmt.Errorf("应用不存在")
	}
	if _, ok, err := s.GetBot(ctx, appID, tenantID, botID); err != nil || !ok {
		return nil, fmt.Errorf("Bot不存在或不属于该应用")
	}
	cfg, err := openrpc.GetBotConfig(ctx, app.TenantID, botID)
	if err != nil {
		return nil, err
	}
	if cfg.UserID == 0 {
		return nil, fmt.Errorf("Bot用户记录缺失")
	}
	return openrpc.GetHistory(ctx, app.TenantID, cfg.UserID, conversationID, beforeMsgID, limit)
}

func (s *OpenService) ListConversations(ctx context.Context, appID string, tenantID, botID int64) ([]openrpc.ConversationItem, error) {
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return nil, err
	}
	if app.AppID == "" {
		return nil, fmt.Errorf("应用不存在")
	}
	if _, ok, err := s.GetBot(ctx, appID, tenantID, botID); err != nil || !ok {
		return nil, fmt.Errorf("Bot不存在或不属于该应用")
	}
	cfg, err := openrpc.GetBotConfig(ctx, app.TenantID, botID)
	if err != nil {
		return nil, err
	}
	if cfg.UserID == 0 {
		return nil, fmt.Errorf("Bot用户记录缺失")
	}
	return openrpc.GetConversations(ctx, app.TenantID, cfg.UserID)
}

// ===== 事件 =====

func (s *OpenService) ListEvents(ctx context.Context, appID string, sinceID int64, limit int) ([]model.OpenEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.dao.ListEvents(appID, sinceID, limit)
}

func (s *OpenService) AckEvents(ctx context.Context, appID string, ids []int64) error {
	return s.dao.AckEvents(appID, ids)
}

// RecordBotEvent 由事件消费者调用：把 Bot 维度的事件落库并按需回调。
func (s *OpenService) RecordBotEvent(ctx context.Context, botID int64, eventType, payload string) error {
	if botID == 0 || eventType == "" {
		return nil
	}
	botInfo, err := openrpc.GetBot(ctx, 0, botID)
	if err != nil {
		return err
	}
	if botInfo.BotID == 0 || botInfo.Origin != model.BotOriginOpenAPI || botInfo.AppID == "" {
		return nil
	}
	ev := &model.OpenEvent{
		AppID:     botInfo.AppID,
		BotID:     botID,
		EventType: eventType,
		Payload:   payload,
		Status:    model.EventStatusPending,
	}
	if err = s.dao.AppendEvent(ev); err != nil {
		return err
	}
	s.dispatchEvent(ctx, ev)
	return nil
}

// dispatchEvent 把事件推给所有订阅该类型的启用回调。
func (s *OpenService) dispatchEvent(ctx context.Context, ev *model.OpenEvent) {
	hooks, err := s.dao.ListEnabledWebhooks(ev.AppID, ev.EventType)
	if err != nil {
		klog.Errorf("查询回调失败, app_id=%s, err=%v", ev.AppID, err)
		return
	}
	if len(hooks) == 0 {
		return
	}
	delivered := false
	for _, wh := range hooks {
		if wh.BotID != 0 && wh.BotID != ev.BotID {
			continue
		}
		existing, err := s.dao.GetDeliveryByEventWebhook(wh.ID, ev.ID)
		if err != nil {
			klog.Errorf("查询投递记录失败: %v", err)
			continue
		}
		if existing.ID != 0 {
			continue
		}
		d := &model.OpenWebhookDelivery{
			AppID:     ev.AppID,
			WebhookID: wh.ID,
			EventID:   ev.ID,
			Status:    model.DeliveryStatusPending,
		}
		if err = s.dao.CreateDelivery(d); err != nil {
			klog.Errorf("创建投递记录失败: %v", err)
			continue
		}
		if err = s.attemptDelivery(ctx, d, wh, ev); err != nil {
			klog.Warnf("回调投递失败, webhook_id=%d, err=%v", wh.ID, err)
			continue
		}
		delivered = true
	}
	if delivered {
		_ = s.dao.MarkEventsDelivered([]int64{ev.ID})
	}
}

// attemptDelivery 执行一次投递；失败时按退避策略排下一次重试，超过上限置死信。
func (s *OpenService) attemptDelivery(ctx context.Context, d *model.OpenWebhookDelivery, wh model.OpenWebhook, ev *model.OpenEvent) error {
	d.Attempts++
	err := s.postWebhook(ctx, wh, ev)
	if err == nil {
		d.Status = model.DeliveryStatusSuccess
		d.LastError = ""
		d.NextRetryAt = 0
		return s.dao.UpdateDelivery(d)
	}
	d.LastError = truncate(err.Error(), 500)
	if d.Attempts >= s.webhookMaxAttempts {
		d.Status = model.DeliveryStatusDead
		d.NextRetryAt = 0
		return s.dao.UpdateDelivery(d)
	}
	idx := int(d.Attempts) - 1
	if idx >= len(s.webhookBackoff) {
		idx = len(s.webhookBackoff) - 1
	}
	d.NextRetryAt = time.Now().Add(time.Duration(s.webhookBackoff[idx]) * time.Second).Unix()
	if updErr := s.dao.UpdateDelivery(d); updErr != nil {
		return updErr
	}
	return err
}

// postWebhook 以 webhook.secret 对请求体签名后 POST 给第三方。
func (s *OpenService) postWebhook(ctx context.Context, wh model.OpenWebhook, ev *model.OpenEvent) error {
	body := map[string]interface{}{
		"event_id":   ev.ID,
		"event_type": ev.EventType,
		"app_id":     ev.AppID,
		"bot_id":     ev.BotID,
		"created_at": ev.CreatedAt.UnixMilli(),
	}
	var payload interface{}
	if json.Unmarshal([]byte(ev.Payload), &payload) == nil {
		body["payload"] = payload
	} else {
		body["payload"] = ev.Payload
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("序列化回调体失败: %w", err)
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := randomHex(8)
	if err != nil {
		return err
	}
	bodySHA := sha256Hex(string(raw))
	signature := hmacSHA256Hex(wh.Secret, StringToSign(ev.AppID, timestamp, nonce, "POST", "/", bodySHA))

	reqCtx, cancel := context.WithTimeout(ctx, s.webhookTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, wh.URL, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("构造回调请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Id", ev.AppID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", signature)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("回调请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("回调返回非2xx状态码: %d", resp.StatusCode)
	}
	return nil
}

// ===== 回调管理 =====

// RegisterWebhook 注册回调并生成挑战串，注册后立即发起一次地址校验。
func (s *OpenService) RegisterWebhook(ctx context.Context, appID string, botID int64, url, events string) (int64, string, string, error) {
	app, err := s.dao.GetAppByAppID(appID)
	if err != nil {
		return 0, "", "", err
	}
	if app.AppID == "" {
		return 0, "", "", fmt.Errorf("应用不存在")
	}
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return 0, "", "", fmt.Errorf("回调地址必须是 http(s) URL")
	}
	secret, err := randomHex(24)
	if err != nil {
		return 0, "", "", err
	}
	challenge, err := randomHex(16)
	if err != nil {
		return 0, "", "", err
	}
	wh := &model.OpenWebhook{
		ID:      s.snowNode.Generate(),
		AppID:   appID,
		BotID:   botID,
		URL:     url,
		Secret:  secret,
		Events:  normalizeScopes(events),
		Enabled: true,
		Status:  WebhookStatusPending,
	}
	if err = s.dao.CreateWebhook(wh); err != nil {
		return 0, "", "", err
	}
	if verifyErr := s.verifyWebhook(ctx, *wh, challenge); verifyErr != nil {
		klog.Warnf("回调地址校验未通过, webhook_id=%d, url=%s, err=%v", wh.ID, url, verifyErr)
	}
	return wh.ID, secret, challenge, nil
}

// VerifyWebhook 重新发起一次地址有效性校验（供 /webhooks/{id}/verify 调用）。
func (s *OpenService) VerifyWebhook(ctx context.Context, appID string, id int64) (bool, error) {
	wh, err := s.dao.GetWebhook(appID, id)
	if err != nil {
		return false, err
	}
	if wh.ID == 0 {
		return false, nil
	}
	challenge, err := randomHex(16)
	if err != nil {
		return false, err
	}
	if err = s.verifyWebhook(ctx, wh, challenge); err != nil {
		return false, err
	}
	return true, nil
}

// verifyWebhook 向第三方 POST 一个 url_verification 挑战，要求其回显 challenge。
func (s *OpenService) verifyWebhook(ctx context.Context, wh model.OpenWebhook, challenge string) error {
	raw, err := json.Marshal(map[string]string{
		"type":      "url_verification",
		"challenge": challenge,
	})
	if err != nil {
		return err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := randomHex(8)
	if err != nil {
		return err
	}
	signature := hmacSHA256Hex(wh.Secret, StringToSign(wh.AppID, timestamp, nonce, "POST", "/", sha256Hex(string(raw))))
	reqCtx, cancel := context.WithTimeout(ctx, s.webhookTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, wh.URL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Id", wh.AppID)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", signature)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		_ = s.dao.UpdateWebhookStatus(wh.ID, WebhookStatusFailed)
		return fmt.Errorf("回调校验请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = s.dao.UpdateWebhookStatus(wh.ID, WebhookStatusFailed)
		return fmt.Errorf("回调校验返回状态码 %d", resp.StatusCode)
	}
	// 接受两种回显：纯文本 challenge，或 {"challenge": "..."} JSON。
	bodyText := strings.TrimSpace(string(body))
	var echoed struct {
		Challenge string `json:"challenge"`
	}
	if json.Unmarshal(body, &echoed) == nil && echoed.Challenge != "" {
		bodyText = echoed.Challenge
	}
	if bodyText != challenge {
		_ = s.dao.UpdateWebhookStatus(wh.ID, WebhookStatusFailed)
		return fmt.Errorf("回调未正确回显挑战串")
	}
	return s.dao.UpdateWebhookStatus(wh.ID, WebhookStatusVerified)
}

func (s *OpenService) ListWebhooks(ctx context.Context, appID string, botID int64) ([]model.OpenWebhook, error) {
	return s.dao.ListWebhooks(appID, botID)
}

func (s *OpenService) GetWebhook(ctx context.Context, appID string, id int64) (model.OpenWebhook, bool, error) {
	wh, err := s.dao.GetWebhook(appID, id)
	if err != nil {
		return model.OpenWebhook{}, false, err
	}
	return wh, wh.ID != 0, nil
}

func (s *OpenService) DeleteWebhook(ctx context.Context, appID string, id int64) (bool, error) {
	wh, err := s.dao.GetWebhook(appID, id)
	if err != nil {
		return false, err
	}
	if wh.ID == 0 {
		return false, nil
	}
	if err = s.dao.DeleteWebhook(appID, id); err != nil {
		return false, err
	}
	return true, nil
}

// RetryDueDeliveries 扫描到期的重试投递，供后台循环调用。
func (s *OpenService) RetryDueDeliveries(ctx context.Context) {
	due, err := s.dao.ListDueDeliveries(time.Now().Unix(), 100)
	if err != nil {
		klog.Errorf("查询待重试回调失败: %v", err)
		return
	}
	for _, d := range due {
		wh, err := s.dao.GetWebhook(d.AppID, d.WebhookID)
		if err != nil || wh.ID == 0 {
			continue
		}
		evs, err := s.dao.ListEvents(d.AppID, d.EventID-1, 1)
		if err != nil || len(evs) == 0 || evs[0].ID != d.EventID {
			// 事件已被清理，标记死信避免无限重试。
			d.Status = model.DeliveryStatusDead
			d.LastError = "事件已过期"
			d.NextRetryAt = 0
			_ = s.dao.UpdateDelivery(&d)
			continue
		}
		if err = s.attemptDelivery(ctx, &d, wh, &evs[0]); err != nil {
			klog.Warnf("回调重试失败, delivery_id=%d, err=%v", d.ID, err)
		}
	}
}

// PurgeExpiredEvents 清理超过保留期的事件。
func (s *OpenService) PurgeExpiredEvents(ctx context.Context) {
	deadline := time.Now().AddDate(0, 0, -s.eventRetentionDays).Unix()
	if n, err := s.dao.PurgeEventsBefore(deadline); err != nil {
		klog.Errorf("清理过期事件失败: %v", err)
	} else if n > 0 {
		klog.Infof("清理过期事件 %d 条", n)
	}
}

// ===== 辅助 =====

// StringToSign 是开放平台统一的待签串格式（入站验签与出站回调共用）。
//
//	app_id + "\n" + timestamp + "\n" + nonce + "\n" + method + "\n" + path + "\n" + body_sha256
func StringToSign(appID, timestamp, nonce, method, path, bodySHA256 string) string {
	return strings.Join([]string{appID, timestamp, nonce, strings.ToUpper(method), path, bodySHA256}, "\n")
}

// normalizeScopes 去掉空白项并去重，保持逗号分隔的稳定顺序。
func normalizeScopes(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, 8)
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return strings.Join(out, ",")
}

// HasScope 判断作用域串是否包含指定作用域。
func HasScope(scopes, want string) bool {
	if want == "" {
		return true
	}
	for _, part := range strings.Split(scopes, ",") {
		if strings.TrimSpace(part) == want {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func toAppDTO(a model.OpenApp) AppDTO {
	return AppDTO{
		AppID:     a.AppID,
		Name:      a.Name,
		Scopes:    a.Scopes,
		Status:    a.Status,
		QPSLimit:  a.QPSLimit,
		OwnerID:   a.OwnerID,
		TenantID:  a.TenantID,
		CreatedAt: a.CreatedAt.UnixMilli(),
	}
}
