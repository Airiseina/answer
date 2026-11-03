package dal

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"

	"gorm.io/gorm"
)

type agentGraphDao struct {
	db *gorm.DB
}

func NewAgentGraphDao(db *gorm.DB) AgentGraphDao {
	return &agentGraphDao{db}
}

type AgentGraphDao interface {
	GetAgentGraph(tenantId, botId int64) (model.AgentGraph, error)
	GetAgentGraphByID(tenantId, graphId int64) (model.AgentGraph, error)
	CreateAgentGraph(graph model.AgentGraph) error
	UpdateAgentGraph(tenantId, id int64, updates map[string]interface{}) error
	CreateHistory(history model.AgentGraphHistory) error
	ListHistory(tenantId, botId int64, limit int) ([]model.AgentGraphHistory, error)
}

// GetAgentGraph 按「租户 + Bot」读取编排图，一个 Bot 至多一张图。
func (d *agentGraphDao) GetAgentGraph(tenantId, botId int64) (model.AgentGraph, error) {
	var graph model.AgentGraph
	err := d.db.Where("tenant_id = ? AND bot_id = ?", tenantId, botId).First(&graph).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.AgentGraph{}, nil
		}
		return model.AgentGraph{}, fmt.Errorf("查询编排图失败: %w", err)
	}
	return graph, nil
}

// GetAgentGraphByID 供更新前校验归属，强制限定租户，避免跨租户篡改。
func (d *agentGraphDao) GetAgentGraphByID(tenantId, graphId int64) (model.AgentGraph, error) {
	var graph model.AgentGraph
	err := d.db.Where("id = ? AND tenant_id = ?", graphId, tenantId).First(&graph).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.AgentGraph{}, nil
		}
		return model.AgentGraph{}, fmt.Errorf("查询编排图失败: %w", err)
	}
	return graph, nil
}

func (d *agentGraphDao) CreateAgentGraph(graph model.AgentGraph) error {
	if err := d.db.Create(&graph).Error; err != nil {
		return fmt.Errorf("创建编排图失败: %w", err)
	}
	return nil
}

func (d *agentGraphDao) UpdateAgentGraph(tenantId, id int64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	err := d.db.Model(&model.AgentGraph{}).Where("id = ? AND tenant_id = ?", id, tenantId).
		Updates(updates).Error
	if err != nil {
		return fmt.Errorf("更新编排图失败: %w", err)
	}
	return nil
}

func (d *agentGraphDao) CreateHistory(history model.AgentGraphHistory) error {
	if err := d.db.Create(&history).Error; err != nil {
		return fmt.Errorf("写入编排图历史失败: %w", err)
	}
	return nil
}

// ListHistory 返回某 Bot 的历史版本快照，按版本倒序，便于画布回滚到最近一次。
func (d *agentGraphDao) ListHistory(tenantId, botId int64, limit int) ([]model.AgentGraphHistory, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []model.AgentGraphHistory
	err := d.db.Where("tenant_id = ? AND bot_id = ?", tenantId, botId).
		Order("created_at DESC, id DESC").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("查询编排图历史失败: %w", err)
	}
	return items, nil
}
