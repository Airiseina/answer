package dal

import (
	"github.com/Airiseina/answer/kitex_service/knowledge_service/internal/model"

	"gorm.io/gorm"
)

type KnowledgeBaseDao interface {
	Create(kb model.KnowledgeBase) error
	GetByID(kbID int64) (model.KnowledgeBase, error)
	GetByOwner(ownerID int64) ([]model.KnowledgeBase, error)
	Update(kbID int64, updates map[string]interface{}) error
	Delete(kbID int64) error
	IncrDocCount(kbID int64, delta int) error
	IncrChunkCount(kbID int64, delta int) error
}

type DocumentDao interface {
	Create(doc model.KbDocument) error
	GetByID(docID int64) (model.KbDocument, error)
	GetByKBID(kbID int64) ([]model.KbDocument, error)
	UpdateStatus(docID int64, status string, chunkCount int, errMsg string) error
	DeleteByKBID(kbID int64) error
	DeleteByID(docID int64) error
	GetPendingDocuments(limit int) ([]model.KbDocument, error)
	ResetStuckDocuments() error
	GetStuckDocuments() ([]model.KbDocument, error)
	GetParsedDocuments() ([]model.KbDocument, error)
}

type BotKnowledgeDao interface {
	Create(bk model.BotKnowledge) error
	Delete(botID, kbID int64) error
	GetByBotID(botID int64) ([]model.BotKnowledge, error)
	GetByKBID(kbID int64) ([]model.BotKnowledge, error)
	GetKnowledgeBasesByBotID(botID int64) ([]model.KnowledgeBase, error)
}

func NewKnowledgeBaseDao(db *gorm.DB) KnowledgeBaseDao {
	return &knowledgeBaseDao{db: db}
}

func NewDocumentDao(db *gorm.DB) DocumentDao {
	return &documentDao{db: db}
}

func NewBotKnowledgeDao(db *gorm.DB) BotKnowledgeDao {
	return &botKnowledgeDao{db: db}
}
