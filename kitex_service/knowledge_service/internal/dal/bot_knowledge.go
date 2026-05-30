package dal

import (
	"fmt"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/model"

	"gorm.io/gorm"
)

type botKnowledgeDao struct {
	db *gorm.DB
}

func (d *botKnowledgeDao) Create(bk model.BotKnowledge) error {
	if err := d.db.Create(&bk).Error; err != nil {
		return fmt.Errorf("创建Bot-KB绑定失败: %w", err)
	}
	return nil
}

func (d *botKnowledgeDao) Delete(botID, kbID int64) error {
	if err := d.db.Where("bot_id = ? AND kb_id = ?", botID, kbID).Delete(&model.BotKnowledge{}).Error; err != nil {
		return fmt.Errorf("删除Bot-KB绑定失败: %w", err)
	}
	return nil
}

func (d *botKnowledgeDao) GetByBotID(botID int64) ([]model.BotKnowledge, error) {
	var bks []model.BotKnowledge
	err := d.db.Where("bot_id = ?", botID).Find(&bks).Error
	if err != nil {
		return nil, fmt.Errorf("查询Bot-KB绑定失败: %w", err)
	}
	return bks, nil
}

func (d *botKnowledgeDao) GetByKBID(kbID int64) ([]model.BotKnowledge, error) {
	var bks []model.BotKnowledge
	err := d.db.Where("kb_id = ?", kbID).Find(&bks).Error
	if err != nil {
		return nil, fmt.Errorf("查询KB-Bot绑定失败: %w", err)
	}
	return bks, nil
}

func (d *botKnowledgeDao) GetKnowledgeBasesByBotID(botID int64) ([]model.KnowledgeBase, error) {
	var kbs []model.KnowledgeBase
	err := d.db.Joins("JOIN bot_knowledge ON bot_knowledge.kb_id = knowledge_base.id").
		Where("bot_knowledge.bot_id = ?", botID).
		Find(&kbs).Error
	if err != nil {
		return nil, fmt.Errorf("查询Bot关联的知识库失败: %w", err)
	}
	return kbs, nil
}
