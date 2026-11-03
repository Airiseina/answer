package agentgraph

import (
	"strings"
	"testing"
)

func TestValidateLinearGraph(t *testing.T) {
	g := &Graph{
		Nodes: []Node{
			{ID: "start", Type: NodeTypeStart, Name: "开始"},
			{ID: "a", Type: NodeTypeAgent, Name: "研究员", SystemPrompt: "你负责检索"},
			{ID: "end", Type: NodeTypeEnd, Name: "汇总"},
		},
		Edges: []Edge{{From: "start", To: "a"}, {From: "a", To: "end"}},
	}
	if errs := g.Validate(); len(errs) != 0 {
		t.Fatalf("期望校验通过，实际: %v", errs)
	}
	layers, err := g.TopoLayers()
	if err != nil {
		t.Fatalf("分层失败: %v", err)
	}
	if len(layers) != 3 {
		t.Fatalf("期望 3 层，实际 %d", len(layers))
	}
}

func TestValidateParallelGraph(t *testing.T) {
	g := &Graph{
		Nodes: []Node{
			{ID: "start", Type: NodeTypeStart, Name: "开始"},
			{ID: "a", Type: NodeTypeAgent, Name: "A"},
			{ID: "b", Type: NodeTypeAgent, Name: "B"},
			{ID: "end", Type: NodeTypeEnd, Name: "汇总"},
		},
		Edges: []Edge{
			{From: "start", To: "a"}, {From: "start", To: "b"},
			{From: "a", To: "end"}, {From: "b", To: "end"},
		},
	}
	if errs := g.Validate(); len(errs) != 0 {
		t.Fatalf("期望校验通过，实际: %v", errs)
	}
	layers, err := g.TopoLayers()
	if err != nil {
		t.Fatalf("分层失败: %v", err)
	}
	if len(layers) != 3 || len(layers[1]) != 2 {
		t.Fatalf("期望中间层 2 个节点，实际 %d 层 %d 节点", len(layers), len(layers[1]))
	}
}

func TestValidateRejectsCycle(t *testing.T) {
	g := &Graph{
		Nodes: []Node{
			{ID: "start", Type: NodeTypeStart, Name: "开始"},
			{ID: "a", Type: NodeTypeAgent, Name: "A"},
			{ID: "b", Type: NodeTypeAgent, Name: "B"},
			{ID: "end", Type: NodeTypeEnd, Name: "汇总"},
		},
		Edges: []Edge{
			{From: "start", To: "a"}, {From: "a", To: "b"},
			{From: "b", To: "a"}, {From: "b", To: "end"},
		},
	}
	errs := g.Validate()
	if len(errs) == 0 {
		t.Fatal("期望检测到环，实际通过")
	}
}

func TestValidateRejectsUnreachable(t *testing.T) {
	g := &Graph{
		Nodes: []Node{
			{ID: "start", Type: NodeTypeStart, Name: "开始"},
			{ID: "a", Type: NodeTypeAgent, Name: "A"},
			{ID: "orphan", Type: NodeTypeAgent, Name: "孤立"},
			{ID: "end", Type: NodeTypeEnd, Name: "汇总"},
		},
		Edges: []Edge{{From: "start", To: "a"}, {From: "a", To: "end"}},
	}
	errs := g.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e, "无法从开始节点到达") {
			found = true
		}
	}
	if !found {
		t.Fatal("期望检测到不可达节点")
	}
}
