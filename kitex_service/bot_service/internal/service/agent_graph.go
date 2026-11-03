package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"
	"github.com/Airiseina/answer/pkg/agentgraph"
	"github.com/Airiseina/answer/pkg/audit"
	"github.com/Airiseina/answer/pkg/snowflake"
	"github.com/Airiseina/answer/pkg/tenant"

	"github.com/cloudwego/kitex/pkg/klog"
)

const (
	agentGraphMaxNameLen     = 64
	agentGraphDefaultHistory = 20
	agentGraphMaxHistory     = 50
)

// AgentGraphService 负责 Bot 多 Agent 协作编排图的保存、校验、发布与版本历史。
//
// 图结构校验统一走 pkg/agentgraph，与 work_service 运行前的校验共用同一套规则；
// 保存允许「结构可解析但校验不通过」的草稿，发布则要求完全校验通过。
type AgentGraphService struct {
	dao      dal.AgentGraphDao
	botDao   dal.BotDao
	audit    audit.Recorder
	snowNode *snowflake.Node
}

func NewAgentGraphService(dao dal.AgentGraphDao, botDao dal.BotDao, rec audit.Recorder) *AgentGraphService {
	if rec == nil {
		rec = audit.NoopRecorder{}
	}
	return &AgentGraphService{
		dao:      dao,
		botDao:   botDao,
		audit:    rec,
		snowNode: snowflake.NewNode(snowflake.ShardBot),
	}
}

// AgentGraphDTO 是编排图的对外视图。
type AgentGraphDTO struct {
	GraphID   int64
	BotID     int64
	Name      string
	GraphJSON string
	Status    int16
	Version   int32
	CreatedAt int64
	UpdatedAt int64
}

// AgentGraphHistoryDTO 是历史版本快照的对外视图。
type AgentGraphHistoryDTO struct {
	ID         int64
	GraphID    int64
	Version    int32
	GraphJSON  string
	OperatorID int64
	CreatedAt  int64
}

// SaveGraphResult 是保存编排图的结果；Errors 非空表示校验未全部通过。
type SaveGraphResult struct {
	GraphID int64
	Version int32
	Status  int16
	Errors  []string
}

// Save 新建或更新编排图：写入主表 + 落一条历史快照 + 记录审计。
//
// 任何一次保存都会把状态重置为草稿并递增版本，需重新发布才对对话生效；
// JSON 无法解析时视为致命错误（不落库，Success=false），仅校验不通过时仍保存草稿。
func (svc *AgentGraphService) Save(ctx context.Context, tenantId, botId, operatorId int64, graphId *int64,
	name, graphJSON string) (SaveGraphResult, error) {
	if !tenant.IsValid(tenantId) {
		return SaveGraphResult{}, fmt.Errorf("缺少有效的租户上下文")
	}
	b, err := svc.botDao.GetBot(tenantId, botId)
	if err != nil {
		return SaveGraphResult{}, err
	}
	if b.ID == 0 {
		return SaveGraphResult{}, fmt.Errorf("Bot[%d]不存在或不属于当前租户", botId)
	}

	parsed, perr := agentgraph.Parse(graphJSON)
	if perr != nil {
		return SaveGraphResult{Errors: []string{perr.Error()}}, nil
	}
	validateErrs := parsed.Validate()
	name = truncateRunes(strings.TrimSpace(name), agentGraphMaxNameLen)
	if name == "" {
		name = "未命名编排"
	}
	now := time.Now()

	// 已存在的图：仅当 graphId 归属当前租户时才允许更新。
	existing, err := svc.dao.GetAgentGraph(tenantId, botId)
	if err != nil {
		return SaveGraphResult{}, err
	}
	if graphId != nil && *graphId != 0 && existing.ID != 0 && existing.ID != *graphId {
		return SaveGraphResult{}, fmt.Errorf("Bot[%d]已存在编排图，不能另建", botId)
	}

	if existing.ID == 0 {
		graph := model.AgentGraph{
			ID:        svc.snowNode.Generate(),
			TenantID:  tenantId,
			BotID:     botId,
			Name:      name,
			GraphJSON: graphJSON,
			Status:    model.AgentGraphStatusDraft,
			Version:   1,
		}
		if err := svc.dao.CreateAgentGraph(graph); err != nil {
			svc.record(ctx, operatorId, tenantId, "agent_graph_save", 0, "创建编排图失败", audit.ResultFailure)
			return SaveGraphResult{}, err
		}
		if err := svc.writeHistory(tenantId, botId, graph.ID, graph.Version, graphJSON, operatorId); err != nil {
			klog.CtxWarnf(ctx, "编排图[%d]写入历史快照失败: %v", graph.ID, err)
		}
		svc.record(ctx, operatorId, tenantId, "agent_graph_save", graph.ID, "新建编排图", audit.ResultSuccess)
		return SaveGraphResult{GraphID: graph.ID, Version: graph.Version, Status: graph.Status, Errors: validateErrs}, nil
	}

	version := existing.Version + 1
	updates := map[string]interface{}{
		"name":       name,
		"graph_json": graphJSON,
		"status":     model.AgentGraphStatusDraft,
		"version":    version,
		"updated_at": now,
	}
	if err := svc.dao.UpdateAgentGraph(tenantId, existing.ID, updates); err != nil {
		svc.record(ctx, operatorId, tenantId, "agent_graph_save", existing.ID, "更新编排图失败", audit.ResultFailure)
		return SaveGraphResult{}, err
	}
	if err := svc.writeHistory(tenantId, botId, existing.ID, version, graphJSON, operatorId); err != nil {
		klog.CtxWarnf(ctx, "编排图[%d]写入历史快照失败: %v", existing.ID, err)
	}
	svc.record(ctx, operatorId, tenantId, "agent_graph_save", existing.ID, "更新编排图", audit.ResultSuccess)
	return SaveGraphResult{GraphID: existing.ID, Version: version, Status: model.AgentGraphStatusDraft, Errors: validateErrs}, nil
}

// Get 读取某 Bot 的编排图；不存在时返回零值 DTO（GraphID=0）。
func (svc *AgentGraphService) Get(ctx context.Context, tenantId, botId int64) (AgentGraphDTO, error) {
	if !tenant.IsValid(tenantId) {
		return AgentGraphDTO{}, fmt.Errorf("缺少有效的租户上下文")
	}
	graph, err := svc.dao.GetAgentGraph(tenantId, botId)
	if err != nil {
		return AgentGraphDTO{}, err
	}
	if graph.ID == 0 {
		return AgentGraphDTO{}, nil
	}
	return toAgentGraphDTO(graph), nil
}

// Validate 只做结构与内容校验，不落库，供画布实时提示。
func (svc *AgentGraphService) Validate(ctx context.Context, botId int64, graphJSON string) (bool, []string) {
	parsed, err := agentgraph.Parse(graphJSON)
	if err != nil {
		return false, []string{err.Error()}
	}
	errs := parsed.Validate()
	return len(errs) == 0, errs
}

// Publish 发布编排图：必须是完全校验通过的图，发布后 work_service 才会按图执行。
func (svc *AgentGraphService) Publish(ctx context.Context, tenantId, botId, operatorId int64) (bool, []string, error) {
	if !tenant.IsValid(tenantId) {
		return false, nil, fmt.Errorf("缺少有效的租户上下文")
	}
	graph, err := svc.dao.GetAgentGraph(tenantId, botId)
	if err != nil {
		return false, nil, err
	}
	if graph.ID == 0 {
		return false, []string{"编排图不存在，请先保存"}, nil
	}
	parsed, perr := agentgraph.Parse(graph.GraphJSON)
	if perr != nil {
		return false, []string{perr.Error()}, nil
	}
	if errs := parsed.Validate(); len(errs) > 0 {
		return false, errs, nil
	}
	if graph.Status != model.AgentGraphStatusPublished {
		if err := svc.dao.UpdateAgentGraph(tenantId, graph.ID, map[string]interface{}{
			"status":     model.AgentGraphStatusPublished,
			"updated_at": time.Now(),
		}); err != nil {
			return false, nil, err
		}
	}
	svc.record(ctx, operatorId, tenantId, "agent_graph_publish", graph.ID,
		fmt.Sprintf("发布编排图版本=%d", graph.Version), audit.ResultSuccess)
	return true, nil, nil
}

// ListHistory 返回编排图的历史版本快照，供画布回滚。
func (svc *AgentGraphService) ListHistory(ctx context.Context, tenantId, botId int64, limit int32) ([]AgentGraphHistoryDTO, error) {
	if !tenant.IsValid(tenantId) {
		return nil, fmt.Errorf("缺少有效的租户上下文")
	}
	n := int(limit)
	if n <= 0 {
		n = agentGraphDefaultHistory
	}
	if n > agentGraphMaxHistory {
		n = agentGraphMaxHistory
	}
	items, err := svc.dao.ListHistory(tenantId, botId, n)
	if err != nil {
		return nil, err
	}
	dtos := make([]AgentGraphHistoryDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, AgentGraphHistoryDTO{
			ID:         it.ID,
			GraphID:    it.GraphID,
			Version:    it.Version,
			GraphJSON:  it.GraphJSON,
			OperatorID: it.OperatorID,
			CreatedAt:  it.CreatedAt.UnixMilli(),
		})
	}
	return dtos, nil
}

func (svc *AgentGraphService) writeHistory(tenantId, botId, graphId int64, version int32, graphJSON string, operatorId int64) error {
	return svc.dao.CreateHistory(model.AgentGraphHistory{
		ID:         svc.snowNode.Generate(),
		GraphID:    graphId,
		TenantID:   tenantId,
		BotID:      botId,
		Version:    version,
		GraphJSON:  graphJSON,
		OperatorID: operatorId,
	})
}

func toAgentGraphDTO(graph model.AgentGraph) AgentGraphDTO {
	return AgentGraphDTO{
		GraphID:   graph.ID,
		BotID:     graph.BotID,
		Name:      graph.Name,
		GraphJSON: graph.GraphJSON,
		Status:    graph.Status,
		Version:   graph.Version,
		CreatedAt: graph.CreatedAt.UnixMilli(),
		UpdatedAt: graph.UpdatedAt.UnixMilli(),
	}
}

// record 统一写入编排图审计日志，失败只告警不影响主流程。
func (svc *AgentGraphService) record(ctx context.Context, actorId, tenantId int64, action string,
	targetId int64, detail string, result int16) {
	event := audit.Event{
		EventType:  audit.EventBotPublish,
		ActorType:  audit.ActorUser,
		ActorID:    actorId,
		TenantID:   tenantId,
		Action:     action,
		TargetType: "agent_graph",
		TargetID:   strconv.FormatInt(targetId, 10),
		Detail:     detail,
		Result:     result,
	}
	if err := svc.audit.Record(ctx, event); err != nil {
		klog.CtxWarnf(ctx, "写入编排图审计日志失败: %v", err)
	}
}
