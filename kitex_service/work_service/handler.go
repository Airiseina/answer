package main

import (
	"context"

	"github.com/Airiseina/answer/kitex_service/work_service/internal/service"
	work "github.com/Airiseina/answer/kitex_service/work_service/kitex_gen/work"

	"github.com/cloudwego/kitex/pkg/klog"
)

type WorkServiceImpl struct {
	workService *service.WorkService
}

func (s *WorkServiceImpl) HandleMessage(ctx context.Context, req *work.HandleMessageReq) (resp *work.HandleMessageRes, err error) {
	success, err := s.workService.HandleMessage(ctx, req.BotId, req.ConversationId, req.SenderId, req.Content, req.History)
	if err != nil {
		klog.CtxErrorf(ctx, "Bot[%d]处理消息失败: %v", req.BotId, err)
		return &work.HandleMessageRes{Success: false}, err
	}
	return &work.HandleMessageRes{Success: success}, nil
}
