// Package agentgraph 定义「多 Agent 协作编排图」的 schema 与校验规则。
//
// 编排图由前端画布产出，序列化为 JSON 存入 bot_service；work_service 在执行时
// 反序列化并按有向无环图（DAG）分层调度，每个 agent 节点复用单 Agent 的 ReAct 执行器。
// 校验逻辑集中在此包，保证「保存校验」与「运行前校验」使用同一套规则。
package agentgraph

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NodeType 是编排节点的类型。
type NodeType string

const (
	NodeTypeStart NodeType = "start" // 入口，只有一个，透传用户输入
	NodeTypeAgent NodeType = "agent" // 一个子 Agent（独立提示词/模型/工具）
	NodeTypeEnd   NodeType = "end"   // 出口，汇聚前驱输出并生成最终回复
)

// 编排图规模与内容上限，防止画布被滥用为资源放大器。
const (
	MaxNodes           = 12
	MaxPromptLength    = 4000
	MaxNameLength      = 64
	MaxStepsPerNode    = 6
	MaxEdges           = 40
	DefaultMaxStepsPad = 1
)

// AllowTools 是 agent 节点可声明的工具白名单。
// 只放行平台受控能力，避免画布直接指定任意 MCP 地址。
var AllowTools = map[string]bool{
	"":          true,
	"none":      true,
	"knowledge": true,
	"memory":    true,
}

// Node 是一个编排节点。
type Node struct {
	ID           string   `json:"id"`
	Type         NodeType `json:"type"`
	Name         string   `json:"name"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
	Model        string   `json:"model,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	MaxSteps     int      `json:"max_steps,omitempty"`
}

// Edge 是一条有向边，表示 from 的输出会作为 to 的上下文。
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Graph 是一张完整的编排图。
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Parse 反序列化编排图 JSON。
func Parse(raw string) (*Graph, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("编排图为空")
	}
	var g Graph
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		return nil, fmt.Errorf("编排图 JSON 解析失败: %w", err)
	}
	return &g, nil
}

// Validate 校验编排图，返回人类可读的错误列表；空列表表示合法。
func (g *Graph) Validate() []string {
	var errs []string
	if g == nil {
		return []string{"编排图为空"}
	}
	if len(g.Nodes) == 0 {
		errs = append(errs, "编排图至少需要一个节点")
	}
	if len(g.Nodes) > MaxNodes {
		errs = append(errs, fmt.Sprintf("节点数超过上限 %d", MaxNodes))
	}
	if len(g.Edges) > MaxEdges {
		errs = append(errs, fmt.Sprintf("连线数超过上限 %d", MaxEdges))
	}

	byID := make(map[string]*Node, len(g.Nodes))
	startCount := 0
	endCount := 0
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.ID == "" {
			errs = append(errs, "存在缺少 id 的节点")
			continue
		}
		if _, dup := byID[n.ID]; dup {
			errs = append(errs, fmt.Sprintf("节点 id 重复: %s", n.ID))
			continue
		}
		byID[n.ID] = n

		switch n.Type {
		case NodeTypeStart:
			startCount++
		case NodeTypeEnd:
			endCount++
		case NodeTypeAgent:
			if len([]rune(n.Name)) > MaxNameLength {
				errs = append(errs, fmt.Sprintf("节点[%s]名称过长", n.ID))
			}
			if len([]rune(n.SystemPrompt)) > MaxPromptLength {
				errs = append(errs, fmt.Sprintf("节点[%s]提示词过长（上限 %d 字）", n.ID, MaxPromptLength))
			}
			for _, t := range n.Tools {
				if !AllowTools[t] {
					errs = append(errs, fmt.Sprintf("节点[%s]引用了不允许的工具: %s", n.ID, t))
				}
			}
			if n.MaxSteps < 0 || n.MaxSteps > MaxStepsPerNode {
				errs = append(errs, fmt.Sprintf("节点[%s]步数上限需在 0~%d 之间", n.ID, MaxStepsPerNode))
			}
		default:
			errs = append(errs, fmt.Sprintf("节点[%s]类型非法: %q", n.ID, n.Type))
		}
	}

	if startCount != 1 {
		errs = append(errs, "编排图必须且只能有一个开始节点")
	}
	if endCount == 0 {
		errs = append(errs, "编排图至少需要一个结束节点")
	}

	inDeg := make(map[string]int, len(byID))
	outDeg := make(map[string]int, len(byID))
	adj := make(map[string][]string, len(byID))
	seen := make(map[string]bool, len(g.Edges))
	for _, e := range g.Edges {
		from, okFrom := byID[e.From]
		to, okTo := byID[e.To]
		if !okFrom {
			errs = append(errs, fmt.Sprintf("连线引用了不存在的起点: %s", e.From))
			continue
		}
		if !okTo {
			errs = append(errs, fmt.Sprintf("连线引用了不存在的终点: %s", e.To))
			continue
		}
		if e.From == e.To {
			errs = append(errs, fmt.Sprintf("节点[%s]不能连接自身", e.From))
			continue
		}
		key := e.From + "->" + e.To
		if seen[key] {
			continue
		}
		seen[key] = true
		adj[e.From] = append(adj[e.From], e.To)
		inDeg[e.To]++
		outDeg[e.From]++
		if from.Type == NodeTypeEnd {
			errs = append(errs, fmt.Sprintf("结束节点[%s]不能有出边", e.From))
		}
		if to.Type == NodeTypeStart {
			errs = append(errs, fmt.Sprintf("开始节点[%s]不能有入边", e.To))
		}
	}

	if startCount == 1 && endCount > 0 {
		errs = append(errs, checkReachability(g, byID, startCount)...)
	}

	if _, err := g.TopoLayers(); err != nil {
		errs = append(errs, err.Error())
	}
	return errs
}

// checkReachability 校验所有节点都能从开始节点到达。
func checkReachability(g *Graph, byID map[string]*Node, startCount int) []string {
	if startCount != 1 {
		return nil
	}
	var startID string
	for id, n := range byID {
		if n.Type == NodeTypeStart {
			startID = id
			break
		}
	}
	adj := make(map[string][]string)
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	visited := map[string]bool{startID: true}
	queue := []string{startID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adj[cur] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	var errs []string
	for id := range byID {
		if !visited[id] {
			errs = append(errs, fmt.Sprintf("节点[%s]无法从开始节点到达", id))
		}
	}
	return errs
}

// StartNode 返回唯一的开始节点；不存在时返回 nil。
func (g *Graph) StartNode() *Node {
	for i := range g.Nodes {
		if g.Nodes[i].Type == NodeTypeStart {
			return &g.Nodes[i]
		}
	}
	return nil
}

// NodeByID 按 id 查找节点。
func (g *Graph) NodeByID(id string) *Node {
	for i := range g.Nodes {
		if g.Nodes[i].ID == id {
			return &g.Nodes[i]
		}
	}
	return nil
}

// Predecessors 返回指向 id 的所有节点 id。
func (g *Graph) Predecessors(id string) []string {
	var preds []string
	for _, e := range g.Edges {
		if e.To == id {
			preds = append(preds, e.From)
		}
	}
	return preds
}

// TopoLayers 按 DAG 拓扑序分层：同一层内的节点互不依赖，可并发执行。
// 图中存在环时返回错误。
func (g *Graph) TopoLayers() ([][]*Node, error) {
	if g == nil || len(g.Nodes) == 0 {
		return nil, fmt.Errorf("编排图为空")
	}
	byID := make(map[string]*Node, len(g.Nodes))
	inDeg := make(map[string]int, len(g.Nodes))
	adj := make(map[string][]string, len(g.Nodes))
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.ID == "" {
			return nil, fmt.Errorf("存在缺少 id 的节点")
		}
		byID[n.ID] = n
		inDeg[n.ID] = 0
	}
	for _, e := range g.Edges {
		if _, ok := byID[e.From]; !ok {
			return nil, fmt.Errorf("连线引用了不存在的节点: %s", e.From)
		}
		if _, ok := byID[e.To]; !ok {
			return nil, fmt.Errorf("连线引用了不存在的节点: %s", e.To)
		}
		adj[e.From] = append(adj[e.From], e.To)
		inDeg[e.To]++
	}

	var current []*Node
	for i := range g.Nodes {
		if inDeg[g.Nodes[i].ID] == 0 {
			current = append(current, &g.Nodes[i])
		}
	}
	if len(current) == 0 {
		return nil, fmt.Errorf("编排图存在环，无法确定执行顺序")
	}

	var layers [][]*Node
	processed := 0
	for len(current) > 0 {
		layers = append(layers, current)
		processed += len(current)
		var next []*Node
		for _, n := range current {
			for _, to := range adj[n.ID] {
				inDeg[to]--
				if inDeg[to] == 0 {
					next = append(next, byID[to])
				}
			}
		}
		current = next
	}
	if processed != len(g.Nodes) {
		return nil, fmt.Errorf("编排图存在环，无法确定执行顺序")
	}
	return layers, nil
}
