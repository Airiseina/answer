package dal

import (
	"github.com/Airiseina/answer/kitex_service/bot_service/internal/model"

	"gorm.io/gorm"
)

type botDao struct {
	db *gorm.DB
}

func NewBotDao(db *gorm.DB) BotDao {
	return &botDao{db}
}

type BotDao interface {
	CreateBot(bot model.Bot) error
	GetBot(tenantId, botId int64) (model.Bot, error)
	GetSystemBot() (model.Bot, error)
	GetUserBots(tenantId, creatorId int64) ([]model.Bot, error)
	UpdateBot(tenantId, botId int64, updates map[string]interface{}) error
	DeleteBot(tenantId, botId int64) error
	IsBot(userId int64) (bool, error) //未用上，是废代码
	GetBotByUserId(userId int64) (model.Bot, error)
}
