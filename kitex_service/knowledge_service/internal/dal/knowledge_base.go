package dal

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/model"

	"gorm.io/gorm"
)

type knowledgeBaseDao struct {
	db *gorm.DB
}

func (d *knowledgeBaseDao) Create(kb model.KnowledgeBase) error {
	if err := d.db.Create(&kb).Error; err != nil {
		return fmt.Errorf("创建知识库失败: %w", err)
	}
	return nil
}

func (d *knowledgeBaseDao) GetByID(kbID int64) (model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	err := d.db.Where("id = ?", kbID).First(&kb).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.KnowledgeBase{}, nil
		}
		return model.KnowledgeBase{}, fmt.Errorf("查询知识库失败: %w", err)
	}
	return kb, nil
}

func (d *knowledgeBaseDao) GetByOwner(ownerID int64) ([]model.KnowledgeBase, error) {
	var kbs []model.KnowledgeBase
	err := d.db.Where("owner_id = ?", ownerID).Find(&kbs).Error
	if err != nil {
		return nil, fmt.Errorf("查询用户知识库列表失败: %w", err)
	}
	return kbs, nil
}

func (d *knowledgeBaseDao) Update(kbID int64, updates map[string]interface{}) error {
	if err := d.db.Model(&model.KnowledgeBase{}).Where("id = ?", kbID).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新知识库失败: %w", err)
	}
	return nil
}

func (d *knowledgeBaseDao) Delete(kbID int64) error {
	if err := d.db.Where("id = ?", kbID).Delete(&model.KnowledgeBase{}).Error; err != nil {
		return fmt.Errorf("删除知识库失败: %w", err)
	}
	return nil
}

func (d *knowledgeBaseDao) IncrDocCount(kbID int64, delta int) error {
	err := d.db.Model(&model.KnowledgeBase{}).Where("id = ?", kbID).
		Update("doc_count", gorm.Expr("doc_count + ?", delta)).Error
	if err != nil {
		return fmt.Errorf("更新知识库文档计数失败: %w", err)
	}
	return nil
}

func (d *knowledgeBaseDao) IncrChunkCount(kbID int64, delta int) error {
	err := d.db.Model(&model.KnowledgeBase{}).Where("id = ?", kbID).
		Update("chunk_count", gorm.Expr("chunk_count + ?", delta)).Error
	if err != nil {
		return fmt.Errorf("更新知识库分块计数失败: %w", err)
	}
	return nil
}
