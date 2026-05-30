package dal

import (
	"errors"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/model"

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

func (d *documentDao) GetByID(docID int64) (model.KbDocument, error) {
	var doc model.KbDocument
	err := d.db.Where("id = ?", docID).First(&doc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.KbDocument{}, nil
		}
		return model.KbDocument{}, fmt.Errorf("查询文档失败: %w", err)
	}
	return doc, nil
}

func (d *documentDao) GetByKBID(kbID int64) ([]model.KbDocument, error) {
	var docs []model.KbDocument
	err := d.db.Where("kb_id = ?", kbID).Order("created_at DESC").Find(&docs).Error
	if err != nil {
		return nil, fmt.Errorf("查询知识库文档列表失败: %w", err)
	}
	return docs, nil
}

func (d *documentDao) UpdateStatus(docID int64, status string, chunkCount int, errMsg string) error {
	updates := map[string]interface{}{
		"status":        status,
		"chunk_count":   chunkCount,
		"error_message": errMsg,
	}
	if err := d.db.Model(&model.KbDocument{}).Where("id = ?", docID).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新文档状态失败: %w", err)
	}
	return nil
}

func (d *documentDao) DeleteByKBID(kbID int64) error {
	if err := d.db.Where("kb_id = ?", kbID).Delete(&model.KbDocument{}).Error; err != nil {
		return fmt.Errorf("删除知识库文档失败: %w", err)
	}
	return nil
}

func (d *documentDao) DeleteByID(docID int64) error {
	if err := d.db.Where("id = ?", docID).Delete(&model.KbDocument{}).Error; err != nil {
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
