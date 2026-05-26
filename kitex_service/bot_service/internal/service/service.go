package service

import (
	"context"
	"fmt"
	"os"

	"github.com/Airiseina/answer/kitex_service/bot_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/bot_service/rpc"
	"github.com/Airiseina/answer/pkg/snowflake"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/spf13/viper"
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

func (svc *BotService) CreateBot(ctx context.Context, creatorId int64, name, avatarUrl, systemPrompt, apiKey, model_, baseURL string) (int64, error) {
	botId := svc.snowNode.Generate()
	bot := model.Bot{
		ID:           botId,
		CreatorID:    creatorId,
		Name:         name,
		AvatarURL:    avatarUrl,
		SystemPrompt: systemPrompt,
		ApiKey:       apiKey,
		Model:        model_,
		BaseURL:      baseURL,
		IsSystem:     false,
	}
	err := svc.dao.CreateBot(bot)
	if err != nil {
		return 0, err
	}
	userID, rpcErr := rpc.CreateBotUser(ctx, name, avatarUrl)
	if rpcErr != nil {
		delErr := svc.dao.DeleteBot(botId)
		if delErr != nil {
			klog.Errorf("Bot[%d]创建用户记录失败后回滚删除Bot也失败: %v", botId, delErr)
		}
		return 0, fmt.Errorf("创建Bot用户记录失败: %w", rpcErr)
	}
	err = svc.dao.UpdateBot(botId, map[string]interface{}{"user_id": userID})
	if err != nil {
		klog.Errorf("Bot[%d]更新user_id失败: %v", botId, err)
		delErr := svc.dao.DeleteBot(botId)
		if delErr != nil {
			klog.Errorf("Bot[%d]创建用户记录失败后回滚删除Bot也失败: %v", botId, delErr)
		}
		return 0, fmt.Errorf("创建Bot用户记录失败: %w", rpcErr)
	}
	return botId, nil
}

type BotInfoDTO struct {
	BotId        int64
	UserId       int64
	CreatorId    int64
	Name         string
	AvatarUrl    string
	ApiKey       string
	SystemPrompt string
	Model        string
	BaseURL      string
	IsSystem     bool
	CreatedAt    int64
}

func (svc *BotService) GetBot(botId int64) (BotInfoDTO, error) {
	info, err := svc.dao.GetBot(botId)
	if err != nil {
		return BotInfoDTO{}, err
	}
	dto := BotInfoDTO{
		BotId:        info.ID,
		UserId:       info.UserID,
		CreatorId:    info.CreatorID,
		Name:         info.Name,
		AvatarUrl:    info.AvatarURL,
		SystemPrompt: info.SystemPrompt,
		Model:        info.Model,
		BaseURL:      info.BaseURL,
		IsSystem:     info.IsSystem,
		CreatedAt:    info.CreatedAt.UnixMilli(),
	}
	return dto, nil
}

func (svc *BotService) GetSystemBot() (int64, error) {
	info, err := svc.dao.GetSystemBot()
	if err != nil {
		return 0, err
	}
	return info.ID, nil
}

func (svc *BotService) GetUserBots(creatorId int64) ([]BotInfoDTO, error) {
	infos, err := svc.dao.GetUserBots(creatorId)
	if err != nil {
		return nil, err
	}
	var dtos []BotInfoDTO
	for _, info := range infos {
		dtos = append(dtos, BotInfoDTO{
			BotId:        info.ID,
			CreatorId:    info.CreatorID,
			Name:         info.Name,
			AvatarUrl:    info.AvatarURL,
			SystemPrompt: info.SystemPrompt,
			Model:        info.Model,
			BaseURL:      info.BaseURL,
			IsSystem:     info.IsSystem,
			CreatedAt:    info.CreatedAt.UnixMilli(),
		})
	}
	return dtos, nil
}

func (svc *BotService) UpdateBot(botId, operatorId int64, updates map[string]interface{}) (bool, error) {
	bot, err := svc.dao.GetBot(botId)
	if err != nil {
		return false, err
	}
	if bot.ID == 0 {
		return false, nil
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
		return false, err
	}
	if bot.ID == 0 {
		return false, nil
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

func (svc *BotService) IsBot(userId int64) (bool, int64, error) {
	bot, err := svc.dao.GetBotByUserId(userId)
	if err != nil {
		return false, 0, err
	}
	if bot.ID == 0 {
		return false, 0, nil
	}
	return true, bot.ID, nil
}

const (
	ConvTypePrivate int16 = 1
	ConvTypeGroup   int16 = 2
)

func (svc *BotService) AddBotToConversation(ctx context.Context, operatorId, botId, conversationId int64, convType int16) (int64, error) {
	bot, err := svc.dao.GetBot(botId)
	if err != nil {
		return 0, fmt.Errorf("查询Bot失败: %w", err)
	}
	if bot.ID == 0 {
		return 0, fmt.Errorf("bot不存在")
	}
	if bot.CreatorID != operatorId {
		return 0, fmt.Errorf("只有Bot创建者才能将Bot拉入会话")
	}
	if bot.UserID == 0 {
		return 0, fmt.Errorf("bot用户记录异常，缺少user_id")
	}
	switch convType {
	case ConvTypeGroup:
		err = rpc.AddConversationMembers(ctx, conversationId, []int64{bot.UserID})
		if err != nil {
			return 0, fmt.Errorf("将Bot加入群聊会话失败: %w", err)
		}
		return conversationId, nil
	case ConvTypePrivate:
		if conversationId == 0 {
			convID, err := rpc.GetOrCreatePrivateConversation(ctx, operatorId, bot.UserID)
			if err != nil {
				return 0, fmt.Errorf("创建Bot单聊会话失败: %w", err)
			}
			return convID, nil
		}
		err = rpc.AddConversationMembers(ctx, conversationId, []int64{bot.UserID})
		if err != nil {
			return 0, fmt.Errorf("将Bot加入私聊会话失败: %w", err)
		}
		return conversationId, nil
	default:
		return 0, fmt.Errorf("不支持的会话类型: %d", convType)
	}
}

func (svc *BotService) InitSystemBot(ctx context.Context) (int64, error) {
	bot, err := svc.dao.GetSystemBot()
	if err == nil && bot.ID != 0 {
		klog.Infof("系统Bot已存在, ID: %d", bot.ID)
		return bot.ID, nil
	}
	if err != nil {
		return 0, err
	}
	botId := svc.snowNode.Generate()
	name := viper.GetString("ai.system.bot_name")
	systemPrompt := viper.GetString("ai.system.bot_prompt")
	promptFile := viper.GetString("ai.system.bot_prompt_file")
	if promptFile != "" {
		data, err := os.ReadFile(promptFile)
		if err != nil {
			klog.Warnf("读取系统Bot prompt文件失败: %v, 使用默认prompt", err)
		} else {
			systemPrompt = string(data)
		}
	}
	systemBot := model.Bot{
		ID:           botId,
		CreatorID:    0,
		Name:         name,
		AvatarURL:    "",
		SystemPrompt: systemPrompt,
		ApiKey:       viper.GetString("ai.system.bot_api_key"),
		Model:        viper.GetString("ai.system.bot_model"),
		BaseURL:      viper.GetString("ai.system.bot_base_url"),
		IsSystem:     true,
	}
	err = svc.dao.CreateBot(systemBot)
	if err != nil {
		return 0, fmt.Errorf("初始化系统Bot失败: %w", err)
	}
	userID, rpcErr := rpc.CreateBotUser(ctx, name, "")
	if rpcErr != nil {
		delErr := svc.dao.DeleteBot(botId)
		if delErr != nil {
			klog.Errorf("系统Bot[%d]创建用户记录失败后回滚删除也失败: %v", botId, delErr)
		}
		return 0, fmt.Errorf("系统Bot创建用户记录失败: %w", rpcErr)
	}
	err = svc.dao.UpdateBot(botId, map[string]interface{}{"user_id": userID})
	if err != nil {
		klog.Errorf("系统Bot[%d]更新user_id失败: %v", botId, err)
	}
	klog.Infof("系统Bot创建成功, ID: %d, UserID: %d", botId, userID)
	return botId, nil
}
