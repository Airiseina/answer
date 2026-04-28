package mysql

import (
	"answer/internal/model"
	"answer/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (db *gor) CreateFile(sessionId uint, objectName string) error {
	file := model.File{
		ObjectName: objectName,
		SessionID:  sessionId,
		Status:     0,
	}
	err := db.db.Create(&file).Error
	if err != nil {
		logger.Error("创建文件失败", zap.Error(err))
		return err
	}
	return nil
}

func (db *gor) DeleteFile(sessionId uint, id uint) error {
	file := model.File{
		Model: gorm.Model{
			ID: id,
		},
		SessionID: sessionId,
	}
	err := db.db.Unscoped().Delete(&file).Error
	if err != nil {
		logger.Error("删除文件失败", zap.Error(err))
		return err
	}
	return nil
}

func (db *gor) FindFile(sessionId uint) ([]model.File, error) { //服务查找文件地址
	var file []model.File
	err := db.db.Where("session_id = ?", sessionId).Find(&file).Error
	if err != nil {
		logger.Error("查找文件失败", zap.Error(err))
		return []model.File{}, err
	}
	return file, nil
}

func (db *gor) UpdateFileStatus(sessionId uint, id uint, status string) error {
	err := db.db.Model(&model.File{}).Where("id = ? AND session_id=?", id, sessionId).Update("status", status).Error
	if err != nil {
		logger.Error("更新文件状态失败", zap.Error(err))
		return err
	}
	return nil
}

func (db *gor) CreateSession(userId uint) (uint, error) {
	session := model.Session{
		UserID: userId,
	}
	err := db.db.Create(&session).Error
	if err != nil {
		logger.Error("创建对话失败", zap.Error(err))
		return 0, err
	}
	return session.ID, nil
}

func (db *gor) UpdateSessionTitle(sessionId uint, title string) error {
	result := db.db.Model(&model.Session{}).Where("id = ?", sessionId).Update("title", title)
	if result.Error != nil || result.RowsAffected == 0 {
		logger.Error("创建标题失败", zap.Error(result.Error))
		return result.Error
	}
	return nil
}

func (db *gor) UpdateSessionRoleSetting(sessionId uint, roleSetting string) error {
	result := db.db.Model(&model.Session{}).Where("id = ?", sessionId).Update("roleSetting", roleSetting)
	if result.Error != nil || result.RowsAffected == 0 {
		logger.Error("创建对话角色设定失败", zap.Error(result.Error))
		return result.Error
	}
	return nil
}

func (db *gor) DeleteSession(sessionId uint) error {
	session := model.Session{
		Model: gorm.Model{
			ID: sessionId,
		},
	}
	err := db.db.Unscoped().Delete(&session).Error
	if err != nil {
		logger.Error("删除对话失败", zap.Error(err))
		return err
	}
	return nil
}

func (db *gor) FindSessions(userId uint) ([]model.Session, error) {
	var sessions []model.Session
	err := db.db.Where("user_id = ?", userId).Find(&sessions).Error
	if err != nil {
		logger.Error("查找用户对话失败", zap.Error(err))
		return nil, err
	}
	return sessions, nil
}

func (db *gor) FindSession(sessionId uint) (model.Session, error) { //对话的所有信息（包括历史消息）
	session := model.Session{
		Model: gorm.Model{
			ID: sessionId,
		},
	}
	err := db.db.Preload("Chats").Preload("Files").First(&session).Error
	if err != nil {
		logger.Error("查找对话失败", zap.Error(err))
		return model.Session{}, err
	}
	return session, nil
}

func (db *gor) CreateChat(sessionId uint, msg string) (uint, error) {
	chat := model.Chat{
		SessionID: sessionId,
		AiMessage: "",
		Question:  msg,
	}
	err := db.db.Create(&chat).Error
	if err != nil {
		logger.Error("创建用户对话失败", zap.Error(err))
		return 0, err
	}
	return chat.ID, nil
}

func (db *gor) UpdateChat(sessionId uint, aiMsg string, chatId uint) error {
	result := db.db.Model(&model.Chat{}).Where("session_id=? AND chat_id=?", sessionId, chatId).Update("ai_message", aiMsg)
	if result.Error != nil || result.RowsAffected == 0 {
		logger.Error("更新对话失败", zap.Error(result.Error))
		return result.Error
	}
	return nil
}
