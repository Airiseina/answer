package main

import (
	"chat_service/internal/model"
	"chat_service/internal/service"
	chat "chat_service/kitex_gen/chat"
	"context"
	"fmt"

	"github.com/cloudwego/kitex/pkg/klog"
)

// ChatServiceImpl Kitex RPC 服务实现，作为 Thrift IDL 定义接口的服务端
// 职责：协议转换（Thrift ↔ 内部模型）、参数校验、调用 Service 层
// 边界：不包含业务逻辑，仅做请求/响应的编解码和错误处理
type ChatServiceImpl struct {
	chatService *service.ChatService
}

// SendMessage 处理消息发送 RPC 请求
// 支持单聊（隐式创建会话）和群聊两种场景
// 成功时返回消息ID、时间戳、会话ID和成员列表
// 失败时返回 Success=false（不抛出 RPC 错误，保持与 IDL 定义一致）
func (s *ChatServiceImpl) SendMessage(ctx context.Context, req *chat.SendMessageReq) (resp *chat.SendMessageRes, err error) {
	result, err := s.chatService.SendMessage(ctx, req.SenderId, req.ConversationId, req.PeerId, req.Content, req.ClientSeq)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]发送消息失败: %v", req.SenderId, err)
		return &chat.SendMessageRes{Success: false}, nil
	}

	return &chat.SendMessageRes{
		Success:        true,
		MsgId:          result.MsgID,
		Timestamp:      result.Timestamp,
		ConversationId: result.ConversationID,
		MemberIds:      result.MemberIDs,
	}, nil
}

// GetHistory 处理历史消息拉取 RPC 请求
// 基于 conversation_id 查询，支持 before_msg_id 游标翻页
// 新增成员身份校验：请求者必须是会话成员才能拉取历史消息
func (s *ChatServiceImpl) GetHistory(ctx context.Context, req *chat.GetHistoryReq) (resp *chat.GetHistoryRes, err error) {
	messages, err := s.chatService.GetHistory(ctx, req.UserId, req.ConversationId, req.BeforeMsgId, req.Limit)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]拉取会话[%d]历史消息失败: %v", req.UserId, req.ConversationId, err)
		return &chat.GetHistoryRes{Success: false}, nil
	}

	// 将内部 DTO 转换为 Thrift 消息对象
	var list []*chat.Message
	for _, m := range messages {
		list = append(list, &chat.Message{
			MsgId:          m.MsgID,
			ClientSeq:      m.ClientSeq,
			SenderId:       m.SenderID,
			ConversationId: m.ConversationID,
			Content:        m.Content,
			Timestamp:      m.Timestamp,
		})
	}

	return &chat.GetHistoryRes{
		Success:  true,
		Messages: list,
	}, nil
}

// CreateConversation 处理创建会话 RPC 请求
// 仅用于群聊场景；单聊会话由 SendMessage 隐式创建，无需调用此接口
// 会话类型硬编码为群聊（type=2），不再从请求中读取
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

// GetConversations 处理查询用户会话列表 RPC 请求
// 返回用户参与的所有会话，单聊会话名称为空时自动生成占位名称
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
		// 单聊会话名称为空时，生成默认名称（前端可根据成员信息覆盖展示）
		if convType == model.ConvTypePrivate && name == "" {
			name = fmt.Sprintf("单聊 %d", c.ID)
		}
		list = append(list, &chat.ConversationInfo{
			ConversationId: c.ID,
			Type:           convType,
			Name:           name,
			MemberIds:      c.MemberIDs,
			GroupId:        &c.GroupID,
		})
	}
	return &chat.GetConversationsRes{
		Success:       true,
		Conversations: list,
	}, nil
}

// SetOnline 处理用户上线 RPC 请求
// 由 msg_gateway 在 WebSocket 连接建立时调用，将用户在线状态注册到 Redis
func (s *ChatServiceImpl) SetOnline(ctx context.Context, req *chat.SetOnlineReq) (resp *chat.CommonRes, err error) {
	err = s.chatService.SetOnline(ctx, req.UserId, req.GatewayAddr)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]上线注册失败: %v", req.UserId, err)
		return &chat.CommonRes{Success: false}, nil
	}
	return &chat.CommonRes{Success: true}, nil
}

// SetOffline 处理用户下线 RPC 请求
// 由 msg_gateway 在 WebSocket 连接断开时调用，清除用户在线状态
func (s *ChatServiceImpl) SetOffline(ctx context.Context, req *chat.SetOfflineReq) (resp *chat.CommonRes, err error) {
	err = s.chatService.SetOffline(ctx, req.UserId)
	if err != nil {
		klog.CtxErrorf(ctx, "用户[%d]下线注销失败: %v", req.UserId, err)
		return &chat.CommonRes{Success: false}, nil
	}
	return &chat.CommonRes{Success: true}, nil
}

// GetOnlineStatus 处理批量查询在线状态 RPC 请求
// 返回每个用户的在线/离线状态及所连接的网关地址
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

// AddConversationMembers 处理添加会话成员 RPC 请求
// 由 group_service 在邀请成员入群后调用，同步 conversation_member 数据
// 这是跨服务数据一致性的关键环节：group_service 完成群组治理后，通知 chat_service 更新消息路由数据
func (s *ChatServiceImpl) AddConversationMembers(ctx context.Context, req *chat.AddConversationMembersReq) (resp *chat.AddConversationMembersRes, err error) {
	err = s.chatService.AddConversationMembers(ctx, req.ConversationId, req.MemberIds)
	if err != nil {
		klog.CtxErrorf(ctx, "添加会话[%d]成员失败: %v", req.ConversationId, err)
		return &chat.AddConversationMembersRes{Success: false}, nil
	}
	return &chat.AddConversationMembersRes{Success: true}, nil
}

// RemoveConversationMembers 处理移除会话成员 RPC 请求
// 由 group_service 在踢出成员后调用，同步 conversation_member 数据
// 确保被踢出的成员不再收到该会话的消息推送
func (s *ChatServiceImpl) RemoveConversationMembers(ctx context.Context, req *chat.RemoveConversationMembersReq) (resp *chat.RemoveConversationMembersRes, err error) {
	err = s.chatService.RemoveConversationMembers(ctx, req.ConversationId, req.MemberIds)
	if err != nil {
		klog.CtxErrorf(ctx, "移除会话[%d]成员失败: %v", req.ConversationId, err)
		return &chat.RemoveConversationMembersRes{Success: false}, nil
	}
	return &chat.RemoveConversationMembersRes{Success: true}, nil
}
