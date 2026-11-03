package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/Airiseina/answer/kitex_service/work_service/internal/llm"
	"github.com/Airiseina/answer/kitex_service/work_service/internal/mcp"
	"github.com/Airiseina/answer/pkg/agentgraph"

	"github.com/cloudwego/kitex/pkg/klog"
)

// GraphRunConfig 是一次多 Agent 协作编排执行的入参。
//
// CommonPrompt 是所有节点共享的上下文（安全提示词、知识库说明、可用工具引导、记忆等），
// 每个 agent 节点在此基础上再叠加自己的 SystemPrompt，从而既保留平台约束又允许节点各司其职。
type GraphRunConfig struct {
	APIKey         string
	BaseURL        string
	Model          string
	CommonPrompt   string
	UserContent    string
	ImageData      *llm.ImageData
	KBIDs          []int64
	BotID          int64
	MemoryServers  []mcp.ServerConfig // 供声明了 memory 工具的节点使用
	DefaultServers []mcp.ServerConfig // 未声明工具时继承的默认工具集
	GraphJSON      string
	MaxAgentNodes  int // 单次执行允许的 agent 节点上限；<=0 时使用 agentgraph.MaxNodes
}

// GraphRunResult 是一次编排执行的汇总结果。
type GraphRunResult struct {
	Content string
	Usage   TokenUsage
	Nodes   int // 实际执行的 agent 节点数，用于观测与限流
}

// RunGraph 按编排图分层调度执行：开始节点透传用户输入，agent 节点复用单 Agent 的 ReAct 执行器，
// 结束节点汇聚（或综合）上游输出作为最终回复。
//
// 采用「同层节点串行」而非并发：一是保证多节点共享同一 api_key 时不会被上游限流打爆，
// 二是让 token 记账与日志顺序稳定可复现。任一节点失败即整体失败，由调用方决定是否回退单 Agent。
func (a *Agent) RunGraph(ctx context.Context, cfg GraphRunConfig) (*GraphRunResult, error) {
	graph, err := agentgraph.Parse(cfg.GraphJSON)
	if err != nil {
		return nil, err
	}
	if errs := graph.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("编排图校验失败: %s", strings.Join(errs, "; "))
	}
	layers, err := graph.TopoLayers()
	if err != nil {
		return nil, err
	}
	maxAgents := cfg.MaxAgentNodes
	if maxAgents <= 0 {
		maxAgents = agentgraph.MaxNodes
	}

	outputs := make(map[string]string, len(graph.Nodes))
	var usage TokenUsage
	agentNodes := 0

	for _, layer := range layers {
		for _, node := range layer {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			switch node.Type {
			case agentgraph.NodeTypeStart:
				outputs[node.ID] = cfg.UserContent
			case agentgraph.NodeTypeAgent:
				agentNodes++
				if agentNodes > maxAgents {
					return nil, fmt.Errorf("编排图 agent 节点数超过上限 %d", maxAgents)
				}
				node := node
				res, runErr := a.runGraphNode(ctx, cfg, graph, outputs, node)
				if runErr != nil {
					return nil, runErr
				}
				outputs[node.ID] = res.Content
				addUsage(&usage, res.Usage)
			case agentgraph.NodeTypeEnd:
				endOutput, endUsage, endErr := a.runEndNode(ctx, cfg, graph, outputs, node)
				if endErr != nil {
					return nil, endErr
				}
				outputs[node.ID] = endOutput
				addUsage(&usage, endUsage)
			}
		}
	}

	final := collectFinal(graph, outputs)
	if strings.TrimSpace(final) == "" {
		return nil, fmt.Errorf("编排图未产生有效输出")
	}
	return &GraphRunResult{Content: final, Usage: usage, Nodes: agentNodes}, nil
}

// runGraphNode 执行一个 agent 节点：拼接上游输出、叠加节点提示词、按节点声明装配工具。
func (a *Agent) runGraphNode(ctx context.Context, cfg GraphRunConfig, graph *agentgraph.Graph,
	outputs map[string]string, node *agentgraph.Node) (*AgentResult, error) {
	systemPrompt := strings.TrimSpace(cfg.CommonPrompt)
	if sp := strings.TrimSpace(node.SystemPrompt); sp != "" {
		if systemPrompt != "" {
			systemPrompt += "\n\n"
		}
		systemPrompt += sp
	}
	if name := strings.TrimSpace(node.Name); name != "" {
		systemPrompt += fmt.Sprintf("\n\n你在本次协作中的角色是「%s」。", name)
	}
	kbIDs, servers := resolveNodeTools(node, cfg)
	model := strings.TrimSpace(node.Model)
	if model == "" {
		model = cfg.Model
	}
	res, err := a.Run(ctx, AgentRunConfig{
		APIKey:       cfg.APIKey,
		BaseURL:      cfg.BaseURL,
		Model:        model,
		SystemPrompt: systemPrompt,
		McpServers:   servers,
		UserContent:  buildNodeInput(cfg.UserContent, graph, outputs, node.ID),
		ImageData:    imageForNode(cfg.ImageData, graph, node.ID),
		KBIDs:        kbIDs,
		BotID:        cfg.BotID,
		MaxSteps:     node.MaxSteps,
	})
	if err != nil {
		return nil, fmt.Errorf("节点「%s」执行失败: %w", displayName(node), err)
	}
	klog.Infof("编排节点[%s]执行完成, 输出长度=%d", displayName(node), len(res.Content))
	return res, nil
}

// runEndNode 处理结束节点：默认直接汇聚上游输出；
// 若显式配置了提示词或模型，则再调一次模型把上游结论综合成最终回复。
func (a *Agent) runEndNode(ctx context.Context, cfg GraphRunConfig, graph *agentgraph.Graph,
	outputs map[string]string, node *agentgraph.Node) (string, TokenUsage, error) {
	aggregated := aggregatePredecessors(graph, outputs, node.ID)
	if strings.TrimSpace(node.SystemPrompt) == "" && strings.TrimSpace(node.Model) == "" {
		return aggregated, TokenUsage{}, nil
	}
	systemPrompt := strings.TrimSpace(node.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = "你是协作流程的汇总者。请把上游各节点的结论整合成一段面向用户的最终回复，" +
			"去掉重复与冲突，只保留有用信息，不要提及内部节点名称。"
	}
	model := strings.TrimSpace(node.Model)
	if model == "" {
		model = cfg.Model
	}
	res, err := a.Run(ctx, AgentRunConfig{
		APIKey:       cfg.APIKey,
		BaseURL:      cfg.BaseURL,
		Model:        model,
		SystemPrompt: systemPrompt,
		UserContent:  aggregated,
		KBIDs:        nil,
		BotID:        cfg.BotID,
		MaxSteps:     node.MaxSteps,
	})
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("汇总节点「%s」执行失败: %w", displayName(node), err)
	}
	return res.Content, res.Usage, nil
}

// resolveNodeTools 依据节点声明的工具白名单装配工具集。
//
// 约定：未声明 → 继承 Bot 默认工具（含知识库与 MCP）；声明 none → 关闭全部工具；
// 声明 knowledge → 只开放知识库检索；声明 memory → 只开放记忆检索。
func resolveNodeTools(node *agentgraph.Node, cfg GraphRunConfig) ([]int64, []mcp.ServerConfig) {
	if len(node.Tools) == 0 {
		return cfg.KBIDs, cfg.DefaultServers
	}
	var kbIDs []int64
	var servers []mcp.ServerConfig
	for _, t := range node.Tools {
		switch t {
		case "none":
			return nil, nil
		case "knowledge":
			kbIDs = cfg.KBIDs
		case "memory":
			servers = append(servers, cfg.MemoryServers...)
		}
	}
	return kbIDs, servers
}

// buildNodeInput 把用户输入与所有上游节点的输出拼成当前节点的输入。
func buildNodeInput(userContent string, graph *agentgraph.Graph, outputs map[string]string, id string) string {
	preds := graph.Predecessors(id)
	if len(preds) == 0 {
		return userContent
	}
	var sb strings.Builder
	sb.WriteString(userContent)
	for _, pid := range preds {
		out := strings.TrimSpace(outputs[pid])
		if out == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n\n[上游节点「%s」的输出]\n%s", nodeLabel(graph, pid), out))
	}
	return sb.String()
}

// aggregatePredecessors 汇聚结束节点的所有上游输出；单上游时直接返回其内容。
func aggregatePredecessors(graph *agentgraph.Graph, outputs map[string]string, id string) string {
	preds := graph.Predecessors(id)
	if len(preds) == 0 {
		return ""
	}
	if len(preds) == 1 {
		return strings.TrimSpace(outputs[preds[0]])
	}
	parts := make([]string, 0, len(preds))
	for _, pid := range preds {
		out := strings.TrimSpace(outputs[pid])
		if out == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("【%s】\n%s", nodeLabel(graph, pid), out))
	}
	return strings.Join(parts, "\n\n")
}

// collectFinal 按节点声明顺序拼接所有结束节点的输出，作为最终回复。
func collectFinal(graph *agentgraph.Graph, outputs map[string]string) string {
	var parts []string
	for i := range graph.Nodes {
		n := &graph.Nodes[i]
		if n.Type != agentgraph.NodeTypeEnd {
			continue
		}
		if out := strings.TrimSpace(outputs[n.ID]); out != "" {
			parts = append(parts, out)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

// imageForNode 只在「紧邻开始节点」的 agent 节点上传图片数据，避免同一张图被反复编码传递。
func imageForNode(img *llm.ImageData, graph *agentgraph.Graph, id string) *llm.ImageData {
	if img == nil {
		return nil
	}
	preds := graph.Predecessors(id)
	if len(preds) == 0 {
		return img
	}
	for _, pid := range preds {
		n := graph.NodeByID(pid)
		if n == nil || n.Type != agentgraph.NodeTypeStart {
			return nil
		}
	}
	return img
}

func nodeLabel(graph *agentgraph.Graph, id string) string {
	if n := graph.NodeByID(id); n != nil && strings.TrimSpace(n.Name) != "" {
		return strings.TrimSpace(n.Name)
	}
	return id
}

func displayName(node *agentgraph.Node) string {
	if strings.TrimSpace(node.Name) != "" {
		return strings.TrimSpace(node.Name)
	}
	return node.ID
}

func addUsage(dst *TokenUsage, src TokenUsage) {
	dst.PromptTokens += src.PromptTokens
	dst.CompletionTokens += src.CompletionTokens
	dst.TotalTokens += src.TotalTokens
}
