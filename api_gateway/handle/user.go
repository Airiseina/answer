package handle

import (
	"api_gateway/response"
	"api_gateway/rpc"
	"context"
	"user_service/kitex_gen/user"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type registerParam struct {
	Account  string `json:"account"`
	Name     string `json:"username"`
	Password string `json:"password"`
}

func Register(ctx context.Context, c *app.RequestContext) {
	var param registerParam
	if err := c.BindJSON(&param); err != nil {
		hlog.CtxErrorf(ctx, "注册参数解析失败, err=%v, client_ip=%s", err, c.ClientIP())
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	res, err := rpc.Register(ctx, &user.RegisterReq{
		Account:  param.Account,
		Name:     param.Name,
		Password: param.Password,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "注册RPC调用失败, account=%s, err=%v", param.Account, err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if res.IsExit {
		hlog.CtxWarnf(ctx, "注册重复, account=%s", param.Account)
		response.Error(c, "操作失败", "用户已存在或完善你的信息")
		return
	}
	hlog.CtxInfof(ctx, "注册成功, account=%s, name=%s", param.Account, param.Name)
	response.Success(c, "注册成功")
}
