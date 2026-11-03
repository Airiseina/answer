package model

import "time"

// 编排图状态机：0=草稿 1=已发布 2=已停用。
//
// 只有「已发布」的编排图会被 work_service 用于实际对话，草稿仅用于画布编辑与预览。
const (
	AgentGraphStatusDraft     int16 = 0
	AgentGraphStatusPublished int16 = 1
	AgentGraphStatusOffline   int16 = 2
)

// ValidAgentGraphStatus 判断状态取值是否在状态机范围内。
func ValidAgentGraphStatus(status int16) bool {
	switch status {
	case AgentGraphStatusDraft, AgentGraphStatusPublished, AgentGraphStatusOffline:
		return true
	default:
		return false
	}
}

// AgentGraph 是某个 Bot 的多 Agent 协作编排图（一个 Bot 至多一张图）。
//
// 图结构由前端画布产出并序列化为 JSON 存入 GraphJSON；服务端只做结构与内容校验，
// 具体的执行调度由 work_service 侧按 DAG 分层完成，双方共用 pkg/agentgraph 的校验规则。
type AgentGraph struct {
	ID        int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	TenantID  int64     `gorm:"not null;default:0;index" json:"tenant_id"`
	BotID     int64     `gorm:"not null;index" json:"bot_id"`
	Name      string    `gorm:"type:varchar(64);default:''" json:"name"`
	GraphJSON string    `gorm:"type:text;not null" json:"graph_json"`
	Status    int16     `gorm:"not null;default:0;index" json:"status"`
	Version   int32     `gorm:"not null;default:1" json:"version"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 固定表名，避免 GORM 复数化差异。
func (AgentGraph) TableName() string {
	return "bot_agent_graph"
}

// AgentGraphHistory 记录编排图每次保存的版本快照，供画布回滚与追溯。
type AgentGraphHistory struct {
	ID         int64     `gorm:"primaryKey;autoIncrement:false" json:"id"`
	GraphID    int64     `gorm:"not null;index" json:"graph_id"`
	TenantID   int64     `gorm:"not null;default:0;index" json:"tenant_id"`
	BotID      int64     `gorm:"not null;index" json:"bot_id"`
	Version    int32     `gorm:"not null;default:1" json:"version"`
	GraphJSON  string    `gorm:"type:text;not null" json:"graph_json"`
	OperatorID int64     `gorm:"not null;default:0" json:"operator_id"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 固定表名，避免 GORM 复数化差异。
func (AgentGraphHistory) TableName() string {
	return "bot_agent_graph_history"
}
