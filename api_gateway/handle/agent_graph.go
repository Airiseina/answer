package handle

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"

	bot "github.com/Airiseina/answer/kitex_service/bot_service/kitex_gen/bot"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// agentGraphView 是编排图对外的视图；画布 JSON 直接以对象形式透出，便于前端直接喂给 React Flow。
type agentGraphView struct {
	GraphId   string          `json:"graph_id"`
	BotId     string          `json:"bot_id"`
	Name      string          `json:"name"`
	Graph     json.RawMessage `json:"graph"`
	Status    int16           `json:"status"`
	Version   int32           `json:"version"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

// safeRawJSON 把画布 JSON 字符串安全地转成可内嵌的 JSON 值；无法解析时返回 null。
func safeRawJSON(raw string) json.RawMessage {
	if raw == "" {
		return json.RawMessage("null")
	}
	if !json.Valid([]byte(raw)) {
		return json.RawMessage("null")
	}
	return json.RawMessage(raw)
}

func toAgentGraphView(info *bot.AgentGraphInfo) agentGraphView {
	return agentGraphView{
		GraphId:   fmt.Sprintf("%d", info.GraphId),
		BotId:     fmt.Sprintf("%d", info.BotId),
		Name:      info.Name,
		Graph:     safeRawJSON(info.GraphJson),
		Status:    info.Status,
		Version:   info.Version,
		CreatedAt: fmt.Sprintf("%d", info.CreatedAt),
		UpdatedAt: fmt.Sprintf("%d", info.UpdatedAt),
	}
}

// parseBotIDParam 从 query 中解析 bot_id；缺失或非法时返回错误。
func parseBotIDParam(c *app.RequestContext) (int64, error) {
	raw := c.Query("bot_id")
	if raw == "" {
		return 0, fmt.Errorf("缺少 bot_id")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("bot_id 非法")
	}
	return id, nil
}

// GetAgentGraph 查询某 Bot 的协作编排图（含草稿），仅限当前租户。
func GetAgentGraph(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	botId, err := parseBotIDParam(c)
	if err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	resp, err := rpc.GetAgentGraph(ctx, &bot.GetAgentGraphReq{TenantId: identity.TenantId, BotId: botId})
	if err != nil {
		hlog.CtxErrorf(ctx, "查询Bot[%d]编排图失败: %v", botId, err)
		response.Error(c, "查询失败", err.Error())
		return
	}
	if !resp.GetSuccess() || resp.GetGraph() == nil {
		response.Success(c, map[string]interface{}{"exists": false})
		return
	}
	view := toAgentGraphView(resp.GetGraph())
	response.Success(c, map[string]interface{}{
		"exists": true,
		"graph":  view,
	})
}

type saveAgentGraphReq struct {
	BotId     string `json:"bot_id" vd:"len($) > 0"`
	GraphId   string `json:"graph_id"`
	Name      string `json:"name"`
	GraphJson string `json:"graph_json" vd:"len($) > 0"`
}

// SaveAgentGraph 保存编排查（新建或更新为草稿），返回校验错误列表供画布提示。
func SaveAgentGraph(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var req saveAgentGraphReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	botId, err := strconv.ParseInt(req.BotId, 10, 64)
	if err != nil || botId <= 0 {
		response.Error(c, "参数错误", "bot_id 非法")
		return
	}
	rpcReq := &bot.SaveAgentGraphReq{
		TenantId:   identity.TenantId,
		BotId:      botId,
		OperatorId: identity.Id,
		Name:       req.Name,
		GraphJson:  req.GraphJson,
	}
	if req.GraphId != "" {
		graphId, gerr := strconv.ParseInt(req.GraphId, 10, 64)
		if gerr != nil || graphId <= 0 {
			response.Error(c, "参数错误", "graph_id 非法")
			return
		}
		rpcReq.GraphId = &graphId
	}
	resp, err := rpc.SaveAgentGraph(ctx, rpcReq)
	if err != nil {
		hlog.CtxErrorf(ctx, "保存Bot[%d]编排图失败: %v", botId, err)
		response.Error(c, "保存失败", err.Error())
		return
	}
	response.Success(c, map[string]interface{}{
		"success":  resp.GetSuccess(),
		"graph_id": fmt.Sprintf("%d", resp.GetGraphId()),
		"version":  resp.GetVersion(),
		"status":   resp.GetStatus(),
		"errors":   resp.GetErrors(),
	})
}

type validateAgentGraphReq struct {
	BotId     string `json:"bot_id" vd:"len($) > 0"`
	GraphJson string `json:"graph_json" vd:"len($) > 0"`
}

// ValidateAgentGraph 只做编排图结构与内容校验，不落库。
func ValidateAgentGraph(ctx context.Context, c *app.RequestContext) {
	var req validateAgentGraphReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	botId, err := strconv.ParseInt(req.BotId, 10, 64)
	if err != nil || botId <= 0 {
		response.Error(c, "参数错误", "bot_id 非法")
		return
	}
	resp, err := rpc.ValidateAgentGraph(ctx, &bot.ValidateAgentGraphReq{BotId: botId, GraphJson: req.GraphJson})
	if err != nil {
		hlog.CtxErrorf(ctx, "校验Bot[%d]编排图失败: %v", botId, err)
		response.Error(c, "校验失败", err.Error())
		return
	}
	response.Success(c, map[string]interface{}{
		"valid":  resp.GetValid(),
		"errors": resp.GetErrors(),
	})
}

type publishAgentGraphReq struct {
	BotId string `json:"bot_id" vd:"len($) > 0"`
}

// PublishAgentGraph 发布编排图，发布后该 Bot 的对话才会按图协作执行。
func PublishAgentGraph(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	var req publishAgentGraphReq
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	botId, err := strconv.ParseInt(req.BotId, 10, 64)
	if err != nil || botId <= 0 {
		response.Error(c, "参数错误", "bot_id 非法")
		return
	}
	resp, err := rpc.PublishAgentGraph(ctx, &bot.PublishAgentGraphReq{
		TenantId:   identity.TenantId,
		BotId:      botId,
		OperatorId: identity.Id,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "发布Bot[%d]编排图失败: %v", botId, err)
		response.Error(c, "发布失败", err.Error())
		return
	}
	response.Success(c, map[string]interface{}{"success": resp.GetSuccess()})
}

// ListAgentGraphHistory 查询编排图的历史版本快照。
func ListAgentGraphHistory(ctx context.Context, c *app.RequestContext) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		response.Error(c, "登录过期", "请重新登录")
		return
	}
	botId, err := parseBotIDParam(c)
	if err != nil {
		response.Error(c, "参数错误", err.Error())
		return
	}
	req := &bot.ListAgentGraphHistoryReq{TenantId: identity.TenantId, BotId: botId}
	if raw := c.Query("limit"); raw != "" {
		limit, lerr := strconv.ParseInt(raw, 10, 32)
		if lerr != nil || limit <= 0 {
			response.Error(c, "参数错误", "limit 非法")
			return
		}
		lim := int32(limit)
		req.Limit = &lim
	}
	resp, err := rpc.ListAgentGraphHistory(ctx, req)
	if err != nil {
		hlog.CtxErrorf(ctx, "查询Bot[%d]编排图历史失败: %v", botId, err)
		response.Error(c, "查询失败", err.Error())
		return
	}
	items := make([]map[string]interface{}, 0, len(resp.GetItems()))
	for _, it := range resp.GetItems() {
		items = append(items, map[string]interface{}{
			"id":          fmt.Sprintf("%d", it.GetId()),
			"graph_id":    fmt.Sprintf("%d", it.GetGraphId()),
			"version":     it.GetVersion(),
			"graph":       safeRawJSON(it.GetGraphJson()),
			"operator_id": fmt.Sprintf("%d", it.GetOperatorId()),
			"created_at":  fmt.Sprintf("%d", it.GetCreatedAt()),
		})
	}
	response.Success(c, map[string]interface{}{"items": items})
}
