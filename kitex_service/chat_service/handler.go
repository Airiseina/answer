package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Airiseina/answer/kitex_service/chat_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/chat_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/chat_service/internal/service"
	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat"

	"github.com/Airiseina/answer/pkg/observability/meter"

	"github.com/cloudwego/kitex/pkg/klog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type ChatServiceImpl struct {
	chatService *service.ChatService
}

func (s *ChatServiceImpl) SendMessage(ctx context.Context, req *chat.SendMessageReq) (resp *chat.SendMessageRes, err error) {
	start := time.Now()
	result, err := s.chatService.SendMessage(ctx, req.SenderId, req.ConversationId, req.PeerId, req.Content, req.ClientSeq, req.GetQuoteMsgId())
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]发送消息失败: %v", req.SenderId, err)
		meter.M.MessageSentTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
		return &chat.SendMessageRes{Success: false}, nil
	}
	meter.M.MessageSentTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "success")))
	meter.M.MessageLatency.Record(ctx, float64(time.Since(start).Milliseconds()))
	return &chat.SendMessageRes{
		Success:          true,
		MsgId:            result.MsgID,
		Timestamp:        result.Timestamp,
		ConversationId:   result.ConversationID,
		MemberIds:        result.MemberIDs,
		Content:          &result.Content,
		ConversationType: &result.ConversationType,
		Seq:              &result.Seq,
	}, nil
}

func (s *ChatServiceImpl) GetHistory(ctx context.Context, req *chat.GetHistoryReq) (resp *chat.GetHistoryRes, err error) {
	messages, err := s.chatService.GetHistory(ctx, req.UserId, req.ConversationId, req.BeforeMsgId, req.Limit)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]拉取会话[%d]历史消息失败: %v", req.UserId, req.ConversationId, err)
		return &chat.GetHistoryRes{Success: false}, nil
	}
	var list []*chat.Message
	for _, m := range messages {
		msg := &chat.Message{
			MsgId:          m.MsgID,
			ClientSeq:      m.ClientSeq,
			SenderId:       m.SenderID,
			ConversationId: m.ConversationID,
			Content:        m.Content,
			Timestamp:      m.Timestamp,
			Seq:            &m.Seq,
			Status:         &m.Status,
			IsEdited:       &m.IsEdited,
		}
		if m.QuoteMsgID != 0 {
			msg.QuoteMsgId = &m.QuoteMsgID
		}
		list = append(list, msg)
	}

	return &chat.GetHistoryRes{
		Success:  true,
		Messages: list,
	}, nil
}

func (s *ChatServiceImpl) CreateConversation(ctx context.Context, req *chat.CreateConversationReq) (resp *chat.CreateConversationRes, err error) {
	var groupID int64
	if req.GroupId != nil {
		groupID = *req.GroupId
	}
	convID, err := s.chatService.CreateConversation(ctx, req.Name, req.MemberIds, groupID)
	if err != nil {
		klog.CtxErrorf(ctx, "创建会话失败: %v", err)
		return &chat.CreateConversationRes{Success: false}, nil
	}
	return &chat.CreateConversationRes{
		Success:        true,
		ConversationId: convID,
	}, nil
}

func (s *ChatServiceImpl) GetConversations(ctx context.Context, req *chat.GetConversationsReq) (resp *chat.GetConversationsRes, err error) {
	conversations, err := s.chatService.GetConversations(ctx, req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询用户[%d]会话列表失败: %v", req.UserId, err)
		return nil, fmt.Errorf("查询会话列表失败: %w", err)
	}
	var list []*chat.ConversationInfo
	for _, c := range conversations {
		convType := c.Type
		name := c.Name
		if convType == model.ConvTypePrivate && name == "" {
			name = fmt.Sprintf("单聊 %d", c.ID)
		}
		list = append(list, &chat.ConversationInfo{
			ConversationId: c.ID,
			Type:           convType,
			Name:           name,
			MemberIds:      c.MemberIDs,
			GroupId:        &c.GroupID,
			MaxSeq:         &c.MaxSeq,
			MaxReadSeq:     &c.MaxReadSeq,
			UnreadCount:    &c.UnreadCount,
		})
	}
	return &chat.GetConversationsRes{
		Success:       true,
		Conversations: list,
	}, nil
}

func (s *ChatServiceImpl) SetOnline(ctx context.Context, req *chat.SetOnlineReq) (resp *chat.CommonRes, err error) {
	err = s.chatService.SetOnline(ctx, req.UserId, req.GatewayAddr)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]上线注册失败: %v", req.UserId, err)
		return &chat.CommonRes{Success: false}, nil
	}
	return &chat.CommonRes{Success: true}, nil
}

func (s *ChatServiceImpl) SetOffline(ctx context.Context, req *chat.SetOfflineReq) (resp *chat.CommonRes, err error) {
	err = s.chatService.SetOffline(ctx, req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]下线注销失败: %v", req.UserId, err)
		return &chat.CommonRes{Success: false}, nil
	}
	return &chat.CommonRes{Success: true}, nil
}

func (s *ChatServiceImpl) RenewOnline(ctx context.Context, req *chat.RenewOnlineReq) (resp *chat.CommonRes, err error) {
	err = s.chatService.RenewOnline(ctx, req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]在线状态续期失败: %v", req.UserId, err)
		return &chat.CommonRes{Success: false}, nil
	}
	return &chat.CommonRes{Success: true}, nil
}

func (s *ChatServiceImpl) GetOnlineStatus(ctx context.Context, req *chat.GetOnlineStatusReq) (resp *chat.GetOnlineStatusRes, err error) {
	infos, err := s.chatService.GetOnlineStatus(ctx, req.UserIds)
	if err != nil {
		klog.CtxErrorf(ctx, "查询在线状态失败: %v", err)
		return nil, fmt.Errorf("查询在线状态失败: %w", err)
	}
	var statuses []*chat.OnlineStatus
	for _, info := range infos {
		statuses = append(statuses, &chat.OnlineStatus{
			UserId:      info.UserID,
			Online:      info.Online,
			GatewayAddr: info.GatewayAddr,
		})
	}
	return &chat.GetOnlineStatusRes{Statuses: statuses}, nil
}

func (s *ChatServiceImpl) AddConversationMembers(ctx context.Context, req *chat.AddConversationMembersReq) (resp *chat.AddConversationMembersRes, err error) {
	err = s.chatService.AddConversationMembers(ctx, req.ConversationId, req.MemberIds)
	if err != nil {
		klog.CtxErrorf(ctx, "添加会话[%d]成员失败: %v", req.ConversationId, err)
		return &chat.AddConversationMembersRes{Success: false}, nil
	}
	return &chat.AddConversationMembersRes{Success: true}, nil
}

func (s *ChatServiceImpl) RemoveConversationMembers(ctx context.Context, req *chat.RemoveConversationMembersReq) (resp *chat.RemoveConversationMembersRes, err error) {
	err = s.chatService.RemoveConversationMembers(ctx, req.ConversationId, req.MemberIds)
	if err != nil {
		klog.CtxErrorf(ctx, "移除会话[%d]成员失败: %v", req.ConversationId, err)
		return &chat.RemoveConversationMembersRes{Success: false}, nil
	}
	return &chat.RemoveConversationMembersRes{Success: true}, nil
}

func (s *ChatServiceImpl) DeleteConversation(ctx context.Context, req *chat.DeleteConversationReq) (resp *chat.DeleteConversationRes, err error) {
	err = s.chatService.DeleteConversation(ctx, req.ConversationId)
	if err != nil {
		klog.CtxErrorf(ctx, "删除会话[%d]失败: %v", req.ConversationId, err)
		return &chat.DeleteConversationRes{Success: false}, nil
	}
	return &chat.DeleteConversationRes{Success: true}, nil
}

func (s *ChatServiceImpl) MarkRead(ctx context.Context, req *chat.MarkReadReq) (resp *chat.MarkReadRes, err error) {
	maxReadSeq, err := s.chatService.MarkRead(ctx, req.UserId, req.ConversationId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]标记会话[%d]已读失败: %v", req.UserId, req.ConversationId, err)
		return &chat.MarkReadRes{Success: false}, nil
	}
	return &chat.MarkReadRes{
		Success:    true,
		MaxReadSeq: &maxReadSeq,
	}, nil
}

func (s *ChatServiceImpl) GetConversationMembers(ctx context.Context, req *chat.GetConversationMembersReq) (resp *chat.GetConversationMembersRes, err error) {
	members, err := s.chatService.GetConversationMembers(ctx, req.ConversationId)
	if err != nil {
		klog.CtxErrorf(ctx, "查询会话[%d]成员列表失败: %v", req.ConversationId, err)
		return &chat.GetConversationMembersRes{Success: false}, nil
	}
	return &chat.GetConversationMembersRes{
		Success:   true,
		MemberIds: members,
	}, nil
}

func (s *ChatServiceImpl) GetOrCreatePrivateConversation(ctx context.Context, req *chat.GetOrCreatePrivateConversationReq) (resp *chat.GetOrCreatePrivateConversationRes, err error) {
	convID, err := s.chatService.GetOrCreatePrivateConversation(ctx, req.UserIdA, req.UserIdB)
	if err != nil {
		klog.CtxErrorf(ctx, "获取或创建单聊会话失败, userA=%d, userB=%d: %v", req.UserIdA, req.UserIdB, err)
		return &chat.GetOrCreatePrivateConversationRes{Success: false}, nil
	}
	return &chat.GetOrCreatePrivateConversationRes{
		Success:        true,
		ConversationId: convID,
	}, nil
}

func (s *ChatServiceImpl) RecallMessage(ctx context.Context, req *chat.RecallMessageReq) (resp *chat.RecallMessageRes, err error) {
	result, err := s.chatService.RecallMessage(ctx, req.UserId, req.MsgId, req.ConversationId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]撤回消息[%d]失败: %v", req.UserId, req.MsgId, err)
		return nil, err
	}
	return &chat.RecallMessageRes{
		Success:        true,
		ConversationId: &result.ConversationID,
		MemberIds:      result.MemberIDs,
	}, nil
}

func (s *ChatServiceImpl) EditMessage(ctx context.Context, req *chat.EditMessageReq) (resp *chat.EditMessageRes, err error) {
	result, err := s.chatService.EditMessage(ctx, req.UserId, req.MsgId, req.ConversationId, req.NewContent_)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]编辑消息[%d]失败: %v", req.UserId, req.MsgId, err)
		return &chat.EditMessageRes{Success: false}, nil
	}
	return &chat.EditMessageRes{
		Success:        true,
		ConversationId: &result.ConversationID,
		MemberIds:      result.MemberIDs,
	}, nil
}

func (s *ChatServiceImpl) GetEditHistory(ctx context.Context, req *chat.GetEditHistoryReq) (resp *chat.GetEditHistoryRes, err error) {
	histories, err := s.chatService.GetEditHistory(ctx, req.UserId, req.MsgId, req.ConversationId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]查询消息[%d]编辑历史失败: %v", req.UserId, req.MsgId, err)
		return &chat.GetEditHistoryRes{Success: false}, nil
	}
	var items []*chat.EditHistoryItem
	for _, h := range histories {
		items = append(items, &chat.EditHistoryItem{
			Id:         h.ID,
			MsgId:      h.MsgID,
			Version:    h.Version,
			OldContent: h.OldContent,
			EditorId:   h.EditorID,
			EditedAt:   h.EditedAt,
		})
	}
	return &chat.GetEditHistoryRes{
		Success:   true,
		Histories: items,
	}, nil
}

func (s *ChatServiceImpl) SyncMessages(ctx context.Context, req *chat.SyncMessagesReq) (resp *chat.SyncMessagesRes, err error) {
	var convSeqs []dal.ConvSeqPair
	for _, pair := range req.ConvSeqs {
		convSeqs = append(convSeqs, dal.ConvSeqPair{
			ConversationID: pair.ConversationId,
			LastSeq:        pair.LastSeq,
		})
	}
	results, err := s.chatService.SyncMessages(ctx, req.UserId, convSeqs, req.Limit)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]同步消息失败: %v", req.UserId, err)
		return &chat.SyncMessagesRes{Success: false}, nil
	}
	var convMessages []*chat.ConvMessages
	for _, result := range results {
		var msgs []*chat.Message
		for _, m := range result.Messages {
			msgs = append(msgs, &chat.Message{
				MsgId:          m.MsgID,
				ClientSeq:      m.ClientSeq,
				SenderId:       m.SenderID,
				ConversationId: m.ConversationID,
				Content:        m.Content,
				Timestamp:      m.Timestamp,
				Seq:            &m.Seq,
				Status:         &m.Status,
				IsEdited:       &m.IsEdited,
			})
		}
		convMessages = append(convMessages, &chat.ConvMessages{
			ConversationId: result.ConversationID,
			Messages:       msgs,
		})
	}
	return &chat.SyncMessagesRes{
		Success:      true,
		ConvMessages: convMessages,
	}, nil
}
