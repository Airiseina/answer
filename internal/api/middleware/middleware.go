package middleware

import (
	"answer/internal/api/response"
	"answer/internal/api/rpc"
	"answer/internal/kitex_service/user_service/kitex_gen/user"
	"answer/pkg/logger"
	"context"
	"errors"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/hertz-contrib/jwt"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var key = viper.GetString("jwt.Key")
var Authmiddleware *jwt.HertzJWTMiddleware
var IdentityKey = "user"

type loginParam struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type Resp struct {
	Id      uint
	Account string
}

func JwtMiddleware() {
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
			if res == nil {
				return nil, errors.New("账号密码错误")
			}
			c.Set("user_id", uint(res.Id))
			c.Set("account", res.Account)
			return &Resp{uint(res.Id), res.Account}, nil
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
			var userId uint
			if idVal, ok := claims["id"]; ok {
				if idFloat, ok := idVal.(float64); ok {
					userId = uint(idFloat)
				}
			}
			account := claims["account"].(string)
			return &Resp{userId, account}
		},
		LoginResponse: func(ctx context.Context, c *app.RequestContext, code int, message string, time time.Time) {
			userID, _ := c.Get("user_id")
			account, _ := c.Get("account")
			response.Success(c, map[string]interface{}{
				"token":   message,
				"id":      userID.(uint),
				"account": account,
			})
		},
	})
	if err != nil {
		logger.Fatal("jwt错误", zap.Error(err))
	}
}
