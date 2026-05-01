package mysql

import (
	"answer/internal/kitex_service/chat_service/model"
	"answer/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func NewServiceDao(g *gorm.DB) ServiceDao {
	return &gor{g}
}

func (db *gor) CreateFile(sessionId uint, objectName string) (uint, error) {
	file := model.File{
		ObjectName: objectName,
		SessionID:  sessionId,
		Status:     0,
	}
	err := db.db.Create(&file).Error
	if err != nil {
		logger.Error("创建文件失败", zap.Error(err))
		return 0, err
	}
	return file.ID, nil
}

func (db *gor) UpdateFileStatus(sessionId uint, id uint, status int) error {
	err := db.db.Model(&model.File{}).Where("id = ? AND session_id=?", id, sessionId).Update("status", status).Error
	if err != nil {
		logger.Error("更新文件状态失败", zap.Error(err))
		return err
	}
	return nil
}
