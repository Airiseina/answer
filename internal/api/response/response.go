package response

import (
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
)

type Response struct {
	Code int         `json:"code"` // 业务状态码
	Msg  string      `json:"msg"`  // 提示信息
	Data interface{} `json:"data"` // 数据
}

const (
	SUCCESS = 0 //成功
	ERROR   = 1 //错误
)

func Result(c *app.RequestContext, code int, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}

func Success(c *app.RequestContext, data interface{}) {
	Result(c, SUCCESS, "操作成功", data)
}

func Error(c *app.RequestContext, msg string, data interface{}) {
	Result(c, ERROR, msg, data)
}
