package dal

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/model"
	"github.com/Airiseina/answer/pkg/tenant"

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

func (d *knowledgeBaseDao) GetByID(tenantID, kbID int64) (model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	err := tenant.Scope(d.db.Where("id = ?", kbID), tenantID).First(&kb).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.KnowledgeBase{}, nil
		}
		return model.KnowledgeBase{}, fmt.Errorf("查询知识库失败: %w", err)
	}
	return kb, nil
}

func (d *knowledgeBaseDao) GetByOwner(tenantID, ownerID int64) ([]model.KnowledgeBase, error) {
	var kbs []model.KnowledgeBase
	err := tenant.Scope(d.db.Where("owner_id = ?", ownerID), tenantID).Find(&kbs).Error
	if err != nil {
		return nil, fmt.Errorf("查询用户知识库列表失败: %w", err)
	}
	return kbs, nil
}

func (d *knowledgeBaseDao) Update(tenantID, kbID int64, updates map[string]interface{}) error {
	if err := tenant.Scope(d.db.Model(&model.KnowledgeBase{}).Where("id = ?", kbID), tenantID).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新知识库失败: %w", err)
	}
	return nil
}

func (d *knowledgeBaseDao) Delete(tenantID, kbID int64) error {
	if err := tenant.Scope(d.db.Where("id = ?", kbID), tenantID).Delete(&model.KnowledgeBase{}).Error; err != nil {
		return fmt.Errorf("删除知识库失败: %w", err)
	}
	return nil
}

func (d *knowledgeBaseDao) IncrDocCount(tenantID, kbID int64, delta int) error {
	err := tenant.Scope(d.db.Model(&model.KnowledgeBase{}).Where("id = ?", kbID), tenantID).
		Update("doc_count", gorm.Expr("doc_count + ?", delta)).Error
	if err != nil {
		return fmt.Errorf("更新知识库文档计数失败: %w", err)
	}
	return nil
}

func (d *knowledgeBaseDao) IncrChunkCount(tenantID, kbID int64, delta int) error {
	err := tenant.Scope(d.db.Model(&model.KnowledgeBase{}).Where("id = ?", kbID), tenantID).
		Update("chunk_count", gorm.Expr("chunk_count + ?", delta)).Error
	if err != nil {
		return fmt.Errorf("更新知识库分块计数失败: %w", err)
	}
	return nil
}
