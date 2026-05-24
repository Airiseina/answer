package dal

import (
	"bot_service/internal/model"

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
	GetBot(botId int64) (model.Bot, error)
	GetSystemBot() (model.Bot, error)
	GetUserBots(creatorId int64) ([]model.Bot, error)
	UpdateBot(botId int64, updates map[string]interface{}) error
	DeleteBot(botId int64) error
	IsBot(userId int64) (bool, error)
}
