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

// SummarizeConversation implements the WorkServiceImpl interface.
func (s *WorkServiceImpl) SummarizeConversation(ctx context.Context, req *work.SummarizeConversationReq) (resp *work.SummarizeConversationRes, err error) {
	summary, err := s.workService.SummarizeConversation(ctx, req.ConversationId, req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "总结会话[%d]失败: %v", req.ConversationId, err)
		return &work.SummarizeConversationRes{Success: false}, err
	}
	return &work.SummarizeConversationRes{Success: true, Summary: summary}, nil
}

func (s *WorkServiceImpl) SuggestReplies(ctx context.Context, req *work.SuggestRepliesReq) (resp *work.SuggestRepliesRes, err error) {
	replies, err := s.workService.SuggestReplies(ctx, req.ConversationId, req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "生成回复候选失败: %v", err)
		return &work.SuggestRepliesRes{Success: false}, err
	}
	return &work.SuggestRepliesRes{Success: true, Replies: replies}, nil
}
