package dal

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"

	"gorm.io/gorm"
)

type mcpServerDao struct {
	db *gorm.DB
}

func NewMcpServerDao(db *gorm.DB) McpServerDao {
	return &mcpServerDao{db}
}

type McpServerDao interface {
	CreateMcpServer(server model.McpServer) error
	GetMcpServer(id int64) (model.McpServer, error)
	GetBotMcpServers(botId int64) ([]model.McpServer, error)
	UpdateMcpServer(id int64, updates map[string]interface{}) error
	DeleteMcpServer(id int64) error
}

func (d *mcpServerDao) CreateMcpServer(server model.McpServer) error {
	err := d.db.Create(&server).Error
	if err != nil {
		return fmt.Errorf("创建McpServer失败: %w", err)
	}
	return nil
}

func (d *mcpServerDao) GetMcpServer(id int64) (model.McpServer, error) {
	var server model.McpServer
	err := d.db.Where("id = ?", id).First(&server).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.McpServer{}, nil
		}
		return model.McpServer{}, fmt.Errorf("查询McpServer失败: %w", err)
	}
	return server, nil
}

func (d *mcpServerDao) GetBotMcpServers(botId int64) ([]model.McpServer, error) {
	var servers []model.McpServer
	err := d.db.Where("bot_id = ?", botId).Find(&servers).Error
	if err != nil {
		return nil, fmt.Errorf("查询Bot[%d]的McpServer列表失败: %w", botId, err)
	}
	return servers, nil
}

func (d *mcpServerDao) UpdateMcpServer(id int64, updates map[string]interface{}) error {
	err := d.db.Model(&model.McpServer{}).Where("id = ?", id).Updates(updates).Error
	if err != nil {
		return fmt.Errorf("更新McpServer失败: %w", err)
	}
	return nil
}

func (d *mcpServerDao) DeleteMcpServer(id int64) error {
	err := d.db.Where("id = ?", id).Delete(&model.McpServer{}).Error
	if err != nil {
		return fmt.Errorf("删除McpServer失败: %w", err)
	}
	return nil
}
