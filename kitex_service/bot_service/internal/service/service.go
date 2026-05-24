package service

import (
	"bot_service/internal/dal"
	"bot_service/internal/model"
	"fmt"

	"answer_pkg/snowflake"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type BotService struct {
	dao      dal.BotDao
	snowNode *snowflake.Node
}

func NewBotService(dao dal.BotDao) *BotService {
	return &BotService{
		dao:      dao,
		snowNode: snowflake.NewNode(5),
	}
}

func (svc *BotService) CreateBot(creatorId int64, name, avatarUrl, systemPrompt, apiKey, model_ string) (int64, error) {
	botId := svc.snowNode.Generate()
	bot := model.Bot{
		ID:           botId,
		CreatorID:    creatorId,
		Name:         name,
		AvatarURL:    avatarUrl,
		SystemPrompt: systemPrompt,
		ApiKey:       apiKey,
		Model:        model_,
		IsSystem:     false,
	}
	err := svc.dao.CreateBot(bot)
	if err != nil {
		return 0, fmt.Errorf("创建Bot失败: %w", err)
	}
	return botId, nil
}

func (svc *BotService) GetBot(botId int64) (model.Bot, error) {
	return svc.dao.GetBot(botId)
}

func (svc *BotService) GetSystemBot() (model.Bot, error) {
	return svc.dao.GetSystemBot()
}

func (svc *BotService) GetUserBots(creatorId int64) ([]model.Bot, error) {
	return svc.dao.GetUserBots(creatorId)
}

func (svc *BotService) UpdateBot(botId, operatorId int64, updates map[string]interface{}) (bool, error) {
	bot, err := svc.dao.GetBot(botId)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	if bot.IsSystem {
		return false, nil
	}
	if bot.CreatorID != operatorId {
		return false, nil
	}
	err = svc.dao.UpdateBot(botId, updates)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (svc *BotService) DeleteBot(botId, operatorId int64) (bool, error) {
	bot, err := svc.dao.GetBot(botId)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	if bot.IsSystem {
		return false, nil
	}
	if bot.CreatorID != operatorId {
		return false, nil
	}
	err = svc.dao.DeleteBot(botId)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (svc *BotService) IsBot(userId int64) (bool, error) {
	return svc.dao.IsBot(userId)
}

func (svc *BotService) InitSystemBot() (int64, error) {
	bot, err := svc.dao.GetSystemBot()
	if err == nil && bot.ID != 0 {
		klog.Infof("系统Bot已存在, ID: %d", bot.ID)
		return bot.ID, nil
	}

	botId := svc.snowNode.Generate()
	systemBot := model.Bot{
		ID:           botId,
		CreatorID:    0,
		Name:         viper.GetString("ai.system_bot_name"),
		AvatarURL:    "",
		SystemPrompt: viper.GetString("ai.system_bot_prompt"),
		ApiKey:       viper.GetString("ai.system_bot_api_key"),
		Model:        viper.GetString("ai.system_bot_model"),
		IsSystem:     true,
	}
	err = svc.dao.CreateBot(systemBot)
	if err != nil {
		return 0, fmt.Errorf("初始化系统Bot失败: %w", err)
	}
	klog.Infof("系统Bot创建成功, ID: %d", botId)
	return botId, nil
}
