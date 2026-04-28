package service

import (
	"answer/internal/kitex_service/chat_service/dal/mysql"
)

type ChatService struct {
	dao mysql.ChatDao
}

func NewChatService(dao mysql.ChatDao) *ChatService {
	return &ChatService{dao}
}

type SessionDTO struct {
	SessionId uint
	Title     string
	Role      string
	Chats     []ChatsDTO
}
type ChatsDTO struct {
	Question string
	AiMsg    string
}

func (dao ChatService) isPermission(sessionId uint, userId uint) (bool, error) {
	session, err := dao.dao.FindSession(sessionId)
	if err != nil {
		return false, err
	}
	if session.UserID != userId {
		return false, nil
	}
	return true, nil
}

func (dao ChatService) OpenSession(sessionId uint, userId uint) (SessionDTO, error) {
	if sessionId == 0 {
		sesId, err := dao.dao.CreateSession(sessionId)
		if err != nil {
			return SessionDTO{}, err
		}
		return SessionDTO{SessionId: sesId}, nil
	}
	session, err := dao.dao.FindSession(sessionId)
	if err != nil {
		return SessionDTO{}, err
	}
	if session.UserID != userId {
		return SessionDTO{}, nil
	}
	var chats []ChatsDTO
	for _, chat := range session.Chats {
		c := ChatsDTO{
			Question: chat.Question,
			AiMsg:    chat.AiMessage,
		}
		chats = append(chats, c)
	}
	return SessionDTO{SessionId: session.ID,
		Title: session.Title,
		Role:  session.RoleSetting,
		Chats: chats}, nil
}

func (dao ChatService) FindSessions(userId uint) ([]SessionDTO, error) {
	sessions, err := dao.dao.FindSessions(userId)
	if err != nil {
		return []SessionDTO{}, err
	}
	var sessionsDTO []SessionDTO
	for _, session := range sessions {
		s := SessionDTO{
			SessionId: session.ID,
			Title:     session.Title,
		}
		sessionsDTO = append(sessionsDTO, s)
	}
	return sessionsDTO, nil
}

func (dao ChatService) UpdateSessionTitle(sessionId uint, userId uint, title string) (bool, error) {
	flag, err := dao.isPermission(sessionId, userId)
	if err != nil || !flag {
		return flag, err
	}
	return flag, dao.dao.UpdateSessionTitle(sessionId, title)
}

func (dao ChatService) UpdateSessionRoleSetting(sessionId uint, userId uint, roleSetting string) (bool, error) {
	flag, err := dao.isPermission(sessionId, userId)
	if err != nil || !flag {
		return flag, err
	}
	return flag, dao.dao.UpdateSessionRoleSetting(sessionId, roleSetting)
}

func (dao ChatService) DeleteSession(sessionId uint, userId uint) (bool, error) {
	flag, err := dao.isPermission(sessionId, userId)
	if err != nil || !flag {
		return flag, err
	}
	return flag, dao.dao.DeleteSession(sessionId)
}

func (dao ChatService) CreateFile(userId, sessionId uint, objectName string) (bool, error) {
	flag, err := dao.isPermission(sessionId, userId)
	if err != nil || !flag {
		return flag, err
	}
	return flag, dao.dao.CreateFile(sessionId, objectName)
}

type FileDTO struct {
	FileId    uint
	SessionId uint
}

func (dao ChatService) FindFile(sessionId uint, userId uint) (bool, []FileDTO, error) {
	flag, err := dao.isPermission(sessionId, userId)
	if err != nil || !flag {
		return flag, []FileDTO{}, err
	}
	files, err := dao.dao.FindFile(sessionId)
	if err != nil {
		return flag, []FileDTO{}, err
	}
	var fs []FileDTO
	for _, file := range files {
		f := FileDTO{
			FileId:    file.ID,
			SessionId: file.SessionID,
		}
		fs = append(fs, f)
	}
	return flag, fs, nil
}

func (dao ChatService) DeleteFile(fileId uint, sessionId, userId uint) (bool, error) {
	flag, err := dao.isPermission(sessionId, userId)
	if err != nil || !flag {
		return flag, err
	}
	return flag, dao.dao.DeleteFile(sessionId, fileId)
}

func (dao ChatService) CreateChat(sessionId, userId uint, msg string) (uint, error, bool) {
	flag, err := dao.isPermission(sessionId, userId)
	if err != nil || !flag {
		return 0, err, flag
	}
	chatId, err := dao.dao.CreateChat(sessionId, msg)
	if err != nil {
		return 0, err, flag
	}

}
