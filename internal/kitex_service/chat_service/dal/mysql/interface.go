package mysql

import (
	"answer/internal/model"

	"gorm.io/gorm"
)

type gor struct {
	db *gorm.DB
}

func NewChatDao(db *gorm.DB) ChatDao {
	return &gor{db: db}
}

type ChatDao interface {
	CreateFile(sessionId uint, objectName string) error //创建文件
	DeleteFile(sessionId uint, id uint) error           //删除文件
	FindFile(sessionId uint) ([]model.File, error)      //查找上传文件
	UpdateFileStatus(sessionId uint, id uint, status string) error
	CreateSession(userId uint) (uint, error)                           //创建对话
	UpdateSessionTitle(sessionId uint, title string) error             //修改标题
	UpdateSessionRoleSetting(sessionId uint, roleSetting string) error //修改角色设定
	DeleteSession(sessionId uint) error                                //删除对话
	FindSessions(userId uint) ([]model.Session, error)                 //找寻对话
	FindSession(sessionId uint) (model.Session, error)                 //找寻对话细节
	CreateChat(sessionId uint, msg string) (uint, error)
	UpdateChat(sessionId uint, aiMsg string, chatId uint) error
}
