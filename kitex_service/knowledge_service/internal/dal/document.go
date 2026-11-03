package dal

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/model"
	"github.com/Airiseina/answer/pkg/tenant"

	"gorm.io/gorm"
)

type documentDao struct {
	db *gorm.DB
}

func (d *documentDao) Create(doc model.KbDocument) error {
	if err := d.db.Create(&doc).Error; err != nil {
		return fmt.Errorf("创建文档记录失败: %w", err)
	}
	return nil
}

func (d *documentDao) GetByID(tenantID, docID int64) (model.KbDocument, error) {
	var doc model.KbDocument
	err := tenant.Scope(d.db.Where("id = ?", docID), tenantID).First(&doc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.KbDocument{}, nil
		}
		return model.KbDocument{}, fmt.Errorf("查询文档失败: %w", err)
	}
	return doc, nil
}

func (d *documentDao) GetByKBID(tenantID, kbID int64) ([]model.KbDocument, error) {
	var docs []model.KbDocument
	err := tenant.Scope(d.db.Where("kb_id = ?", kbID), tenantID).Order("created_at DESC").Find(&docs).Error
	if err != nil {
		return nil, fmt.Errorf("查询知识库文档列表失败: %w", err)
	}
	return docs, nil
}

func (d *documentDao) UpdateStatus(tenantID, docID int64, status string, chunkCount int, errMsg string) error {
	updates := map[string]interface{}{
		"status":        status,
		"chunk_count":   chunkCount,
		"error_message": errMsg,
	}
	if err := tenant.Scope(d.db.Model(&model.KbDocument{}).Where("id = ?", docID), tenantID).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新文档状态失败: %w", err)
	}
	return nil
}

func (d *documentDao) DeleteByKBID(tenantID, kbID int64) error {
	if err := tenant.Scope(d.db.Where("kb_id = ?", kbID), tenantID).Delete(&model.KbDocument{}).Error; err != nil {
		return fmt.Errorf("删除知识库文档失败: %w", err)
	}
	return nil
}

func (d *documentDao) DeleteByID(tenantID, docID int64) error {
	if err := tenant.Scope(d.db.Where("id = ?", docID), tenantID).Delete(&model.KbDocument{}).Error; err != nil {
		return fmt.Errorf("删除文档失败: %w", err)
	}
	return nil
}

func (d *documentDao) GetPendingDocuments(limit int) ([]model.KbDocument, error) {
	var docs []model.KbDocument
	err := d.db.Where("status = ?", model.DocStatusPending).Limit(limit).Find(&docs).Error
	if err != nil {
		return nil, fmt.Errorf("查询待解析文档失败: %w", err)
	}
	return docs, nil
}

func (d *documentDao) ResetStuckDocuments() error {
	result := d.db.Model(&model.KbDocument{}).
		Where("status = ?", model.DocStatusParsing).
		Update("status", model.DocStatusPending)
	if result.Error != nil {
		return fmt.Errorf("重置卡住的文档状态失败: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		fmt.Printf("已重置%d个卡在parsing状态的文档为pending\n", result.RowsAffected)
	}
	return nil
}

func (d *documentDao) GetStuckDocuments() ([]model.KbDocument, error) {
	var docs []model.KbDocument
	err := d.db.Where("status = ?", model.DocStatusParsing).Find(&docs).Error
	if err != nil {
		return nil, fmt.Errorf("查询卡住的文档失败: %w", err)
	}
	return docs, nil
}

func (d *documentDao) GetParsedDocuments() ([]model.KbDocument, error) {
	var docs []model.KbDocument
	err := d.db.Where("status = ?", model.DocStatusParsed).Find(&docs).Error
	if err != nil {
		return nil, fmt.Errorf("查询已解析文档失败: %w", err)
	}
	return docs, nil
}
