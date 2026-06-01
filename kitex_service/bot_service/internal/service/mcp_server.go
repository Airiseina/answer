package service

import (
	"fmt"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"
	"github.com/Airiseina/answer/pkg/snowflake"
)

type McpServerService struct {
	dao      dal.McpServerDao
	botDao   dal.BotDao
	snowNode *snowflake.Node
}

func NewMcpServerService(dao dal.McpServerDao, botDao dal.BotDao) *McpServerService {
	return &McpServerService{
		dao:      dao,
		botDao:   botDao,
		snowNode: snowflake.NewNode(6),
	}
}

type McpServerInfoDTO struct {
	ID          int64
	BotID       int64
	Name        string
	Description string
	Transport   string
	URL         string
	AuthType    string
	AuthToken   string
	Enabled     bool
	CreatedAt   int64
}

func (svc *McpServerService) CreateMcpServer(operatorId int64, botId int64, name, url, description, transport, authType, authToken string) (int64, error) {
	bot, err := svc.botDao.GetBot(botId)
	if err != nil {
		return 0, fmt.Errorf("查询Bot失败: %w", err)
	}
	if bot.ID == 0 {
		return 0, fmt.Errorf("bot不存在")
	}
	if bot.IsSystem {
		return 0, fmt.Errorf("系统Bot不支持添加MCP Server")
	}
	if bot.CreatorID != operatorId {
		return 0, fmt.Errorf("只有Bot创建者才能添加MCP Server")
	}
	if transport == "" {
		transport = "sse"
	}
	if authType == "" {
		authType = "none"
	}
	server := model.McpServer{
		ID:          svc.snowNode.Generate(),
		BotID:       botId,
		Name:        name,
		Description: description,
		Transport:   transport,
		URL:         url,
		AuthType:    authType,
		AuthToken:   authToken,
		Enabled:     true,
	}
	err = svc.dao.CreateMcpServer(server)
	if err != nil {
		return 0, err
	}
	return server.ID, nil
}

func (svc *McpServerService) GetBotMcpServers(botId int64) ([]McpServerInfoDTO, error) {
	servers, err := svc.dao.GetBotMcpServers(botId)
	if err != nil {
		return nil, err
	}
	var dtos []McpServerInfoDTO
	for _, s := range servers {
		dtos = append(dtos, svc.toDTO(s))
	}
	return dtos, nil
}

func (svc *McpServerService) UpdateMcpServer(id, operatorId int64, updates map[string]interface{}) (bool, error) {
	server, err := svc.dao.GetMcpServer(id)
	if err != nil {
		return false, err
	}
	if server.ID == 0 {
		return false, nil
	}
	bot, err := svc.botDao.GetBot(server.BotID)
	if err != nil {
		return false, err
	}
	if bot.IsSystem {
		return false, fmt.Errorf("系统Bot不支持修改MCP Server")
	}
	if bot.CreatorID != operatorId {
		return false, fmt.Errorf("只有Bot创建者才能修改MCP Server")
	}
	err = svc.dao.UpdateMcpServer(id, updates)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (svc *McpServerService) DeleteMcpServer(id, operatorId int64) (bool, error) {
	server, err := svc.dao.GetMcpServer(id)
	if err != nil {
		return false, err
	}
	if server.ID == 0 {
		return false, nil
	}
	bot, err := svc.botDao.GetBot(server.BotID)
	if err != nil {
		return false, err
	}
	if bot.IsSystem {
		return false, fmt.Errorf("系统Bot不支持删除MCP Server")
	}
	if bot.CreatorID != operatorId {
		return false, fmt.Errorf("只有Bot创建者才能删除MCP Server")
	}
	err = svc.dao.DeleteMcpServer(id)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (svc *McpServerService) toDTO(s model.McpServer) McpServerInfoDTO {
	return McpServerInfoDTO{
		ID:          s.ID,
		BotID:       s.BotID,
		Name:        s.Name,
		Description: s.Description,
		Transport:   s.Transport,
		URL:         s.URL,
		AuthType:    s.AuthType,
		AuthToken:   s.AuthToken,
		Enabled:     s.Enabled,
		CreatedAt:   s.CreatedAt.UnixMilli(),
	}
}
