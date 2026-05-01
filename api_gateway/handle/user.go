package handle

import (
	"api_gateway/response"
	"api_gateway/rpc"
	"context"
	"user_service/kitex_gen/user"

	"github.com/cloudwego/hertz/pkg/app"
)

type registerParam struct {
	Account  string `json:"account"`
	Name     string `json:"username"`
	Password string `json:"password"`
}

func Register(ctx context.Context, c *app.RequestContext) {
	var param registerParam
	if err := c.BindJSON(&param); err != nil {
		response.Error(c, "参数缺失", "请重新输入参数")
		return
	}
	res, err := rpc.Register(ctx, &user.RegisterReq{
		Account:  param.Account,
		Name:     param.Name,
		Password: param.Password,
	})
	if err != nil {
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if res.IsExit {
		response.Error(c, "操作失败", "用户已存在或完善你的信息")
		return
	}
	response.Success(c, "注册成功")
}
