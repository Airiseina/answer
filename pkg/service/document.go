package service

import (
	"answer_pkg/ai"
	"answer_pkg/dal/mysql"
	"answer_pkg/dal/qdrant"
	"answer_pkg/storage"
	"answer_pkg/until"
	"context"
)

type Service struct {
	dao      mysql.ServiceDao
	qdrant   qdrant.ServiceQdrant
	objStore storage.Storage
}

func NewServiceQdrant(dao mysql.ServiceDao, qdrant qdrant.ServiceQdrant, objStore storage.Storage) *Service {
	return &Service{dao: dao, qdrant: qdrant, objStore: objStore}
}

func (s *Service) FileProcess(ctx context.Context, sessionId uint, fileId uint, objectName string) error { //先保存，再获取文件，在分割后进行向量化
	err := s.dao.UpdateFileStatus(sessionId, fileId, 1) //修改一下位置
	if err != nil {
		return err
	}
	fileByte, err := s.objStore.GetObject(ctx, "", objectName)
	if err != nil {
		return err
	}
	file := string(fileByte)
	sen := until.ChunkText(file, 500, 110)
	err = s.saveQdrant(ctx, sessionId, sen)
	if err != nil {
		return err
	}
	return s.dao.UpdateFileStatus(sessionId, fileId, 2)
}

func (s *Service) saveQdrant(ctx context.Context, sessionId uint, sen []string) error {
	res, err := ai.GetEmbedding(ctx, sen[0])
	if err != nil {
		return err
	}
	vectorSize := len(res)
	err = s.qdrant.CreateVectors(ctx, vectorSize)
	if err != nil {
		return err
	}
	ress, err := ai.GetEmbeddings(ctx, sen)
	if err != nil {
		return err
	}
	return s.qdrant.InsertVectors(ctx, sessionId, sen, ress)
}

func (s *Service) FailQdrant(sessionId, fileId uint) error {
	return s.dao.UpdateFileStatus(sessionId, fileId, 3)
}
