package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"
	"github.com/Airiseina/answer/kitex_service/open_service/kitex_gen/open"
	"github.com/Airiseina/answer/pkg/observability/meter"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/time/rate"
)

// 开放平台签名请求头。
const (
	headerOpenAppID     = "X-App-Id"
	headerOpenTimestamp = "X-Timestamp"
	headerOpenNonce     = "X-Nonce"
	headerOpenSignature = "X-Signature"
)

// OpenAppKey 保存通过签名校验的开放平台应用，供业务 handler 复用。
const OpenAppKey = "open_app"

// 鉴权失败原因，与 open_service 的 err_code 对齐。
const (
	openErrBadSignature     = "bad_signature"
	openErrTimestampExpired = "timestamp_expired"
	openErrReplay           = "replay"
	openErrForbidden        = "forbidden"
)

// OpenAppIdentity 是开放平台调用方在网关侧的视图，从 AppInfo 转成更精简的结构。
type OpenAppIdentity struct {
	AppID    string
	Name     string
	Scopes   string
	QPSLimit int32
	OwnerID  int64
	TenantID int64
}

// CurrentOpenApp 读取当前开放平台调用方的应用信息。
func CurrentOpenApp(c *app.RequestContext) (*OpenAppIdentity, bool) {
	v, ok := c.Get(OpenAppKey)
	if !ok {
		return nil, false
	}
	identity, ok := v.(*OpenAppIdentity)
	return identity, ok
}

// appLimiter 保存单个 AppID 的令牌桶与最近一次访问时间，用于惰性清理。
type appLimiter struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// openAppLimiters 按 AppID 分摊流控；开放平台的应用数量有限，这里常驻内存即可。
var openAppLimiters sync.Map

const openDefaultQPS = 20

func allowOpenApp(appID string, qps int32) bool {
	if qps <= 0 {
		qps = openDefaultQPS
	}
	val, _ := openAppLimiters.LoadOrStore(appID, &appLimiter{
		limiter:    rate.NewLimiter(rate.Limit(qps), int(qps)),
		lastAccess: time.Now(),
	})
	l := val.(*appLimiter)
	l.lastAccess = time.Now()
	return l.limiter.Allow()
}

// OpenAuth 校验开放平台签名（AppID + HMAC-SHA256 + 时间戳窗口 + nonce 防重放）并按 AppID 限流。
// 校验通过后把应用信息写入上下文，后续中间件或 handler 用 CurrentOpenApp 读取。
func OpenAuth() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		appID := string(c.GetHeader(headerOpenAppID))
		timestamp := string(c.GetHeader(headerOpenTimestamp))
		nonce := string(c.GetHeader(headerOpenNonce))
		signature := string(c.GetHeader(headerOpenSignature))
		if appID == "" || timestamp == "" || signature == "" {
			response.Error(c, "鉴权失败", "缺少 X-App-Id / X-Timestamp / X-Signature 请求头")
			c.Abort()
			return
		}
		// 读取请求体后 Hertz 会把内容留在缓冲区，下游 handler 仍可正常解析。
		raw := c.Request.Body()
		sum := sha256.Sum256(raw)
		res, err := rpc.OpenAuthenticateApp(ctx, &open.AuthenticateAppReq{
			AppId:      appID,
			Timestamp:  timestamp,
			Nonce:      nonce,
			Method:     string(c.Method()),
			Path:       string(c.Request.URI().PathOriginal()),
			BodySha256: hex.EncodeToString(sum[:]),
			Signature:  signature,
		})
		if err != nil {
			hlog.CtxErrorf(ctx, "开放平台鉴权RPC失败, app_id=%s, err=%v", appID, err)
			response.Error(c, "系统繁忙", "请稍后重试")
			c.Abort()
			return
		}
		if !res.Success || res.App == nil {
			rejectOpenApp(c, res.ErrCode)
			c.Abort()
			return
		}
		if !allowOpenApp(appID, res.App.QpsLimit) {
			recordOpenAPIRequest(ctx)
			response.Error(c, "请求过于频繁", "已超出该应用的调用配额，请降低频率后重试")
			c.Abort()
			return
		}
		c.Set(OpenAppKey, &OpenAppIdentity{
			AppID:    res.App.AppId,
			Name:     res.App.Name,
			Scopes:   res.App.Scopes,
			QPSLimit: res.App.QpsLimit,
			OwnerID:  res.App.OwnerId,
			TenantID: res.App.TenantId,
		})
		c.Next(ctx)
	}
}

func rejectOpenApp(c *app.RequestContext, errCode string) {
	switch errCode {
	case openErrTimestampExpired:
		response.Error(c, "签名已过期", "X-Timestamp 偏差超出允许窗口，请校准服务器时间后重试")
	case openErrReplay:
		response.Error(c, "重复请求", "该 nonce 已被使用，请更换后重试")
	case openErrForbidden:
		response.Error(c, "应用不可用", "应用不存在、已停用或密钥不可用")
	default:
		response.Error(c, "签名校验失败", "请检查 AppSecret 与签名算法")
	}
}

func recordOpenAPIRequest(ctx context.Context) {
	if meter.M != nil && meter.M.OpenAPIRequestTotal != nil {
		meter.M.OpenAPIRequestTotal.Add(ctx, 1, metric.WithAttributes())
	}
}

// RequireScope 校验开放平台应用是否具备指定作用域；scope 为空表示不校验。
func RequireScope(scope string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if scope == "" {
			c.Next(ctx)
			return
		}
		identity, ok := CurrentOpenApp(c)
		if !ok {
			response.Error(c, "鉴权失败", "请先完成开放平台签名校验")
			c.Abort()
			return
		}
		if !HasScope(identity.Scopes, scope) {
			response.Error(c, "权限不足", "当前应用缺少 "+scope+" 作用域")
			c.Abort()
			return
		}
		recordOpenAPIRequest(ctx)
		c.Next(ctx)
	}
}

// HasScope 判断逗号分隔的作用域串是否包含目标作用域。
func HasScope(scopes, want string) bool {
	for _, part := range strings.Split(scopes, ",") {
		if strings.TrimSpace(part) == want {
			return true
		}
	}
	return false
}
