package middleware

import (
	"context"
	"errors"
	"github.com/Airiseina/answer/api_gateway/config"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"
	"github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user"
	"time"

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
	Id      int64
	Account string
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
			c.Set("account", res.Account)
			c.Set("avatar_url", res.AvatarUrl)
			return &Resp{res.Id, res.Account}, nil
		},
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*Resp); ok {
				return jwt.MapClaims{
					"account": v.Account,
					"id":      v.Id,
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
			account := claims["account"].(string)
			return &Resp{userId, account}
		},
		LoginResponse: func(ctx context.Context, c *app.RequestContext, code int, message string, time time.Time) {
			account, _ := c.Get("account")
			avatarUrl, _ := c.Get("avatar_url")
			response.Success(c, map[string]interface{}{
				"token":      message,
				"account":    account,
				"avatar_url": avatarUrl,
			})
		},
	})
	if err != nil {
		hlog.Fatalf("jwt中间件错误:%v", err)
	}
}
