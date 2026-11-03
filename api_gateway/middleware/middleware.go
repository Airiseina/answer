package middleware

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/Airiseina/answer/api_gateway/config"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"
	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/hertz-contrib/jwt"
)

var Authmiddleware *jwt.HertzJWTMiddleware
var IdentityKey = "user"

type loginParam struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type Resp struct {
	Id       int64
	Account  string
	TenantId int64
}

// CurrentUser 读取当前登录态；未登录返回 false。
func CurrentUser(c *app.RequestContext) (*Resp, bool) {
	v, ok := c.Get(IdentityKey)
	if !ok {
		return nil, false
	}
	resp, ok := v.(*Resp)
	return resp, ok
}

func JwtMiddleware() {
	key := config.V.GetString("jwt.Key")
	var err error
	Authmiddleware, err = jwt.New(&jwt.HertzJWTMiddleware{
		Key:         []byte(key),
		Timeout:     24 * time.Hour,
		MaxRefresh:  7 * 24 * time.Hour,
		TokenLookup: "header: Authorization",
		IdentityKey: IdentityKey,
		Authenticator: func(ctx context.Context, c *app.RequestContext) (interface{}, error) {
			var param loginParam
			if err := c.BindJSON(&param); err != nil {
				return nil, errors.New("参数错误")
			}
			res, err := rpc.Login(ctx, &user.LoginReq{
				Account:  param.Account,
				Password: param.Password,
			})
			if err != nil {
				return nil, errors.New("内部错误")
			}
			if res.Id == 0 {
				return nil, errors.New("账号密码错误")
			}
			// 默认进入个人租户；老数据缺少个人租户时退回第一个可用租户。
			tenantID := res.PersonalTenantId
			if tenantID == 0 && len(res.Tenants) > 0 {
				tenantID = res.Tenants[0].Id
			}
			c.Set("account", res.Account)
			c.Set("avatar_url", res.AvatarUrl)
			c.Set("tenants", res.Tenants)
			identity := &Resp{Id: res.Id, Account: res.Account, TenantId: tenantID}
			// 登录响应回调读不到 jwt 中间件的登录态，这里先落到上下文供其复用。
			c.Set(IdentityKey, identity)
			return identity, nil
		},
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*Resp); ok {
				return jwt.MapClaims{
					"account":   v.Account,
					"id":        v.Id,
					"tenant_id": v.TenantId,
				}
			}
			return jwt.MapClaims{}
		},
		Unauthorized: func(ctx context.Context, c *app.RequestContext, code int, message string) {
			switch message {
			case "参数错误":
				response.Error(c, message, "请重新输入参数")
			case "内部错误":
				response.Error(c, "系统繁忙", "请稍后重试")
			case "账号密码错误":
				response.Error(c, "操作失败", "账号或密码错误")
			default:
				response.Error(c, "登录过期", "请重新登录")
			}
		},
		IdentityHandler: func(ctx context.Context, c *app.RequestContext) interface{} {
			claims := jwt.ExtractClaims(ctx, c)
			var userId int64
			if idVal, ok := claims["id"]; ok {
				if idFloat, ok := idVal.(float64); ok {
					userId = int64(idFloat)
				}
			}
			var tenantId int64
			if tVal, ok := claims["tenant_id"]; ok {
				if tFloat, ok := tVal.(float64); ok {
					tenantId = int64(tFloat)
				}
			}
			account, _ := claims["account"].(string)
			return &Resp{Id: userId, Account: account, TenantId: tenantId}
		},
		LoginResponse: func(ctx context.Context, c *app.RequestContext, code int, message string, time time.Time) {
			account, _ := c.Get("account")
			avatarUrl, _ := c.Get("avatar_url")
			identity, _ := CurrentUser(c)
			data := map[string]interface{}{
				"token":      message,
				"account":    account,
				"avatar_url": avatarUrl,
			}
			if identity != nil {
				data["user_id"] = strconv.FormatInt(identity.Id, 10)
				data["tenant_id"] = strconv.FormatInt(identity.TenantId, 10)
			}
			if tenants, ok := c.Get("tenants"); ok {
				data["tenants"] = tenants
			}
			response.Success(c, data)
		},
	})
	if err != nil {
		hlog.Fatalf("jwt中间件错误:%v", err)
	}
}
