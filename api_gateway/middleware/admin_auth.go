package middleware

import (
	"context"
	"errors"
	"time"

	"github.com/Airiseina/answer/api_gateway/config"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"
	"github.com/Airiseina/answer/kitex_service/admin_service/kitex_gen/admin"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/hertz-contrib/jwt"
)

// AdminAuthmiddleware 是管理后台独立的 JWT 中间件。
// 它与普通用户登录态完全隔离：独立的密钥、独立的 claims、独立的上下文 Key。
var AdminAuthmiddleware *jwt.HertzJWTMiddleware

// AdminIdentityKey 是管理后台登录态在请求上下文中的 Key。
var AdminIdentityKey = "admin"

const adminRoleSuper = "super"

type adminLoginParam struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

// AdminResp 是管理后台登录态。
type AdminResp struct {
	Id      int64
	Account string
	Name    string
	Role    string
}

func AdminJwtMiddleware() {
	key := config.V.GetString("jwt.AdminKey")
	// 未单独配置后台密钥时回退到主密钥，保证单机部署无需额外配置即可启动。
	if key == "" {
		key = config.V.GetString("jwt.Key")
	}
	var err error
	AdminAuthmiddleware, err = jwt.New(&jwt.HertzJWTMiddleware{
		Key:         []byte(key),
		Timeout:     12 * time.Hour,
		MaxRefresh:  24 * time.Hour,
		TokenLookup: "header: Authorization",
		IdentityKey: AdminIdentityKey,
		Authenticator: func(ctx context.Context, c *app.RequestContext) (interface{}, error) {
			var param adminLoginParam
			if err := c.BindJSON(&param); err != nil {
				return nil, errors.New("参数错误")
			}
			res, err := rpc.AdminLogin(ctx, &admin.AdminLoginReq{
				Account:  param.Account,
				Password: param.Password,
			})
			if err != nil {
				return nil, errors.New("内部错误")
			}
			if res.Id == 0 {
				return nil, errors.New("账号密码错误")
			}
			c.Set("admin_name", res.Name)
			c.Set("admin_role", res.Role)
			return &AdminResp{Id: res.Id, Account: res.Account, Name: res.Name, Role: res.Role}, nil
		},
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*AdminResp); ok {
				return jwt.MapClaims{
					"admin_id":      v.Id,
					"admin_account": v.Account,
					"admin_name":    v.Name,
					"admin_role":    v.Role,
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
			var id int64
			if idVal, ok := claims["admin_id"]; ok {
				if idFloat, ok := idVal.(float64); ok {
					id = int64(idFloat)
				}
			}
			account, _ := claims["admin_account"].(string)
			name, _ := claims["admin_name"].(string)
			role, _ := claims["admin_role"].(string)
			return &AdminResp{Id: id, Account: account, Name: name, Role: role}
		},
		LoginResponse: func(ctx context.Context, c *app.RequestContext, code int, message string, t time.Time) {
			name, _ := c.Get("admin_name")
			role, _ := c.Get("admin_role")
			response.Success(c, map[string]interface{}{
				"token": message,
				"name":  name,
				"role":  role,
			})
		},
	})
	if err != nil {
		hlog.Fatalf("管理后台jwt中间件错误:%v", err)
	}
}

// CurrentAdmin 读取当前管理后台登录态。
func CurrentAdmin(c *app.RequestContext) (*AdminResp, bool) {
	v, ok := c.Get(AdminIdentityKey)
	if !ok {
		return nil, false
	}
	resp, ok := v.(*AdminResp)
	return resp, ok
}

// RequireAdminRole 限定接口仅允许指定角色访问；super 恒可访问。
func RequireAdminRole(roles ...string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		resp, ok := CurrentAdmin(c)
		if !ok {
			response.Error(c, "登录过期", "请重新登录")
			c.Abort()
			return
		}
		if resp.Role == adminRoleSuper {
			c.Next(ctx)
			return
		}
		for _, role := range roles {
			if resp.Role == role {
				c.Next(ctx)
				return
			}
		}
		response.Error(c, "无权限", "当前账号无权执行该操作")
		c.Abort()
	}
}
