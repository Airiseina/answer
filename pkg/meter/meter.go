package meter

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

const MeterName = "aim"

var meter metric.Meter

func InitMeter(serviceName string) {
	meter = otel.GetMeterProvider().Meter(
		MeterName,
		metric.WithInstrumentationVersion("1.0.0"),
	)
	registerAll(serviceName)
}

func Meter() metric.Meter {
	if meter == nil {
		panic("meter 未初始化，请先调用 InitMeter()")
	}
	return meter
}

type Metrics struct {
	OnlineUsers          metric.Int64ObservableGauge
	WsConnectTotal       metric.Int64Counter
	WsDisconnectTotal    metric.Int64Counter
	MessageSentTotal     metric.Int64Counter
	MessageReceivedTotal metric.Int64Counter
	MessageLatency       metric.Float64Histogram
	BotRequestTotal      metric.Int64Counter
	BotResponseLatency   metric.Float64Histogram
	BotTokenUsage        metric.Int64Counter
	McpCallTotal         metric.Int64Counter
	McpCallLatency       metric.Float64Histogram
	McpCallErrors        metric.Int64Counter
	McpConnectTotal      metric.Int64Counter
	McpConnectErrors     metric.Int64Counter
	McpReconnectTotal    metric.Int64Counter
	GroupOpTotal         metric.Int64Counter
	UserRegisterTotal    metric.Int64Counter
	UserLoginTotal       metric.Int64Counter
	FriendOpTotal        metric.Int64Counter
}

var M *Metrics

func registerAll(serviceName string) {
	M = &Metrics{}
	var err error

	M.WsConnectTotal, err = Meter().Int64Counter(
		"aim.ws.connect.total",
		metric.WithDescription("WebSocket 连接总数"),
	)
	mustRegister(err, "aim.ws.connect.total")

	M.WsDisconnectTotal, err = Meter().Int64Counter(
		"aim.ws.disconnect.total",
		metric.WithDescription("WebSocket 断开连接总数"),
	)
	mustRegister(err, "aim.ws.disconnect.total")

	M.MessageSentTotal, err = Meter().Int64Counter(
		"aim.message.sent.total",
		metric.WithDescription("发送消息总数"),
	)
	mustRegister(err, "aim.message.sent.total")

	M.MessageReceivedTotal, err = Meter().Int64Counter(
		"aim.message.received.total",
		metric.WithDescription("接收消息总数"),
	)
	mustRegister(err, "aim.message.received.total")

	M.MessageLatency, err = Meter().Float64Histogram(
		"aim.message.latency",
		metric.WithDescription("消息投递延迟(毫秒)"),
		metric.WithUnit("ms"),
	)
	mustRegister(err, "aim.message.latency")

	M.BotRequestTotal, err = Meter().Int64Counter(
		"aim.bot.request.total",
		metric.WithDescription("Bot 请求总数"),
	)
	mustRegister(err, "aim.bot.request.total")

	M.BotResponseLatency, err = Meter().Float64Histogram(
		"aim.bot.response.latency",
		metric.WithDescription("Bot 响应延迟(毫秒)"),
		metric.WithUnit("ms"),
	)
	mustRegister(err, "aim.bot.response.latency")

	M.BotTokenUsage, err = Meter().Int64Counter(
		"aim.bot.token.usage",
		metric.WithDescription("Bot Token 消耗量"),
	)
	mustRegister(err, "aim.bot.token.usage")

	M.McpCallTotal, err = Meter().Int64Counter(
		"aim.mcp.call.total",
		metric.WithDescription("MCP 工具调用总数"),
	)
	mustRegister(err, "aim.mcp.call.total")

	M.McpCallLatency, err = Meter().Float64Histogram(
		"aim.mcp.call.latency",
		metric.WithDescription("MCP 工具调用延迟(毫秒)"),
		metric.WithUnit("ms"),
	)
	mustRegister(err, "aim.mcp.call.latency")

	M.McpCallErrors, err = Meter().Int64Counter(
		"aim.mcp.call.errors",
		metric.WithDescription("MCP 工具调用错误数"),
	)
	mustRegister(err, "aim.mcp.call.errors")

	M.McpConnectTotal, err = Meter().Int64Counter(
		"aim.mcp.connect.total",
		metric.WithDescription("MCP 连接总数"),
	)
	mustRegister(err, "aim.mcp.connect.total")

	M.McpConnectErrors, err = Meter().Int64Counter(
		"aim.mcp.connect.errors",
		metric.WithDescription("MCP 连接错误数"),
	)
	mustRegister(err, "aim.mcp.connect.errors")

	M.McpReconnectTotal, err = Meter().Int64Counter(
		"aim.mcp.reconnect.total",
		metric.WithDescription("MCP 重连总数"),
	)
	mustRegister(err, "aim.mcp.reconnect.total")

	M.GroupOpTotal, err = Meter().Int64Counter(
		"aim.group.operation.total",
		metric.WithDescription("群组操作总数"),
	)
	mustRegister(err, "aim.group.operation.total")

	M.UserRegisterTotal, err = Meter().Int64Counter(
		"aim.user.register.total",
		metric.WithDescription("用户注册总数"),
	)
	mustRegister(err, "aim.user.register.total")

	M.UserLoginTotal, err = Meter().Int64Counter(
		"aim.user.login.total",
		metric.WithDescription("用户登录总数"),
	)
	mustRegister(err, "aim.user.login.total")

	M.FriendOpTotal, err = Meter().Int64Counter(
		"aim.friend.operation.total",
		metric.WithDescription("好友操作总数"),
	)
	mustRegister(err, "aim.friend.operation.total")

	fmt.Printf("[%s] 指标注册完成\n", serviceName)
}

func mustRegister(err error, name string) {
	if err != nil {
		panic(fmt.Sprintf("注册指标 %s 失败: %v", name, err))
	}
}

func RegisterOnlineUsers(callback func(ctx context.Context, observer metric.Int64Observer) error) {
	_, err := Meter().Int64ObservableGauge(
		"aim.online.users",
		metric.WithDescription("当前在线用户数"),
		metric.WithInt64Callback(callback),
	)
	mustRegister(err, "aim.online.users")
}
