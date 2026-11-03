package dal

import (
	"fmt"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/model"
	"github.com/Airiseina/answer/pkg/tenant"

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

func (d *botKnowledgeDao) Delete(tenantID, botID, kbID int64) error {
	q := tenant.Scope(d.db.Where("bot_id = ? AND kb_id = ?", botID, kbID), tenantID)
	if err := q.Delete(&model.BotKnowledge{}).Error; err != nil {
		return fmt.Errorf("删除Bot-KB绑定失败: %w", err)
	}
	return nil
}

func (d *botKnowledgeDao) GetByBotID(tenantID, botID int64) ([]model.BotKnowledge, error) {
	var bks []model.BotKnowledge
	err := tenant.Scope(d.db.Where("bot_id = ?", botID), tenantID).Find(&bks).Error
	if err != nil {
		return nil, fmt.Errorf("查询Bot-KB绑定失败: %w", err)
	}
	return bks, nil
}

func (d *botKnowledgeDao) GetByKBID(tenantID, kbID int64) ([]model.BotKnowledge, error) {
	var bks []model.BotKnowledge
	err := tenant.Scope(d.db.Where("kb_id = ?", kbID), tenantID).Find(&bks).Error
	if err != nil {
		return nil, fmt.Errorf("查询KB-Bot绑定失败: %w", err)
	}
	return bks, nil
}

func (d *botKnowledgeDao) GetKnowledgeBasesByBotID(tenantID, botID int64) ([]model.KnowledgeBase, error) {
	var kbs []model.KnowledgeBase
	q := d.db.Joins("JOIN bot_knowledge ON bot_knowledge.kb_id = knowledge_base.id AND bot_knowledge.tenant_id = knowledge_base.tenant_id").
		Where("bot_knowledge.bot_id = ?", botID)
	q = tenant.ColumnScope(q, "knowledge_base.tenant_id", tenantID)
	err := q.
		Find(&kbs).Error
	if err != nil {
		return nil, fmt.Errorf("查询Bot关联的知识库失败: %w", err)
	}
	return kbs, nil
}
