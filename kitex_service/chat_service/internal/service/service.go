package service

import (
	"answer_pkg/snowflake"
	"chat_service/internal/dal"
	"chat_service/internal/model"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// 会话ID的 Snowflake 节点（节点编号 4），由 Service 层统一管理
// 与消息ID的节点编号 3 区分，确保全局唯一
var (
	convSnowNode     *snowflake.Node
	convSnowNodeOnce sync.Once
)

// getConvSnowNode 获取会话专用的 Snowflake 节点（懒初始化）
func getConvSnowNode() *snowflake.Node {
	convSnowNodeOnce.Do(func() {
		convSnowNode = snowflake.NewNode(4)
	})
	return convSnowNode
}

// ChatService 聊天业务逻辑层
// 职责：消息收发、会话管理、在线状态管理、ID 生成
// 边界：仅处理业务逻辑，不直接操作数据库/缓存（通过 DAO 接口），不处理协议转换（由 handler 层负责）
type ChatService struct {
	dao             dal.ChatDao         // 消息数据访问
	onlineDao       dal.OnlineDao       // 在线状态数据访问
	conversationDao dal.ConversationDao // 会话数据访问
	msgSnowNode     *snowflake.Node     // 消息ID的 Snowflake 节点（节点编号 3）
}

// NewChatService 创建 ChatService 实例
func NewChatService(dao dal.ChatDao, onlineDao dal.OnlineDao, conversationDao dal.ConversationDao) *ChatService {
	return &ChatService{
		dao:             dao,
		onlineDao:       onlineDao,
		conversationDao: conversationDao,
		msgSnowNode:     snowflake.NewNode(3),
	}
}

// SendMessageResult 发送消息的返回结果
type SendMessageResult struct {
	MsgID          int64   // 消息唯一ID（Snowflake 生成）
	Timestamp      int64   // 消息发送时间戳（毫秒）
	ConversationID int64   // 消息所属会话ID（可能是隐式创建的新会话）
	MemberIDs      []int64 // 会话成员列表，供 msg_gateway 推送使用
}

// SendMessage 发送消息（统一入口，单聊和群聊共用）
// 核心逻辑：
//  1. 确定会话ID：若 conversationID 为 0 且 peerID 不为 0，则隐式创建单聊会话
//  2. 校验发送者是否为会话成员
//  3. 生成消息ID，写入数据库
//  4. 返回消息信息和成员列表（供推送使用）
//
// 参数:
//   - senderID: 发送者用户ID
//   - conversationID: 会话ID，已有会话时传入；首次单聊时传 0
//   - peerID: 对端用户ID，仅单聊首次发消息时使用（conversationID=0 时生效）
//   - content: 消息文本内容
//   - clientSeq: 客户端序列号，用于去重
//
// 返回值: 发送结果（含消息ID、时间戳、会话ID、成员列表），或错误信息
func (svc *ChatService) SendMessage(ctx context.Context, senderID int64, conversationID int64, peerID int64, content string, clientSeq int64) (*SendMessageResult, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}
	// 步骤1：确定会话ID
	var convID int64
	if conversationID == 0 && peerID != 0 {
		// 单聊首次发消息：隐式创建会话
		id, err := svc.conversationDao.GetOrCreatePrivateConversation(ctx, senderID, peerID, func() int64 {
			return getConvSnowNode().Generate()
		})
		if err != nil {
			return nil, fmt.Errorf("获取或创建单聊会话失败: %w", err)
		}
		convID = id
	} else if conversationID != 0 {
		// 已有会话：直接使用
		convID = conversationID
	} else {
		// 两者都为空：参数错误
		return nil, fmt.Errorf("conversation_id和peer_id不能同时为空") //该处修改，并不能为系统繁忙
	}
	// 步骤2：校验发送者是否为会话成员
	members, err := svc.conversationDao.GetConversationMembers(ctx, convID)
	if err != nil {
		return nil, fmt.Errorf("查询会话成员失败: %w", err)
	}
	isMember := false
	for _, m := range members {
		if m == senderID {
			isMember = true
			break
		}
	}
	if !isMember {
		return nil, fmt.Errorf("发送者不在该会话中")
	}

	// 步骤3：生成消息ID，写入数据库
	msgID := svc.msgSnowNode.Generate()
	now := time.Now().UnixMilli()
	msg := &model.Message{
		MsgID:          msgID,
		ClientSeq:      clientSeq,
		SenderID:       senderID,
		ConversationID: convID,
		Content:        content,
		Timestamp:      now,
	}
	err = svc.dao.CreateMessage(msg)
	if err != nil {
		return nil, err
	}

	// 步骤4：返回结果
	return &SendMessageResult{
		MsgID:          msgID,
		Timestamp:      now,
		ConversationID: convID,
		MemberIDs:      members,
	}, nil
}

// MessageDTO 消息数据传输对象，用于 Service → Handler 层的数据传递
type MessageDTO struct {
	ClientSeq      int64  // 客户端序列号
	MsgID          int64  // 消息唯一ID
	SenderID       int64  // 发送者用户ID
	ConversationID int64  // 所属会话ID
	Content        string // 消息内容
	Timestamp      int64  // 发送时间戳（毫秒）
}

// GetHistory 拉取会话历史消息
// 新增成员身份校验：请求者必须是会话成员才能拉取历史消息
// 防止非成员通过猜测 conversation_id 读取他人聊天记录
//
// 参数:
//   - userID: 请求者用户ID，用于身份校验
//   - conversationID: 会话ID
//   - beforeMsgID: 游标，返回 msg_id < beforeMsgID 的消息；传 0 从最新开始
//   - limit: 返回条数，范围 [1, 100]，默认 20
//
// 返回值: 消息DTO列表（可能为 nil 表示无消息），或错误信息
func (svc *ChatService) GetHistory(ctx context.Context, userID int64, conversationID int64, beforeMsgID int64, limit int16) ([]MessageDTO, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// 身份校验：请求者必须是会话成员
	members, err := svc.conversationDao.GetConversationMembers(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("查询会话成员失败: %w", err)
	}
	isMember := false
	for _, m := range members {
		if m == userID {
			isMember = true
			break
		}
	}
	if !isMember {
		return nil, fmt.Errorf("无权查看该会话消息")
	}

	var msgs []MessageDTO
	messages, err := svc.dao.GetHistory(conversationID, beforeMsgID, limit)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, nil
	}
	for _, msg := range messages {
		msgs = append(msgs, MessageDTO{
			ClientSeq:      msg.ClientSeq,
			MsgID:          msg.MsgID,
			SenderID:       msg.SenderID,
			ConversationID: msg.ConversationID,
			Content:        msg.Content,
			Timestamp:      msg.Timestamp,
		})
	}
	return msgs, nil
}

// ConversationDTO 会话数据传输对象，用于 Service → Handler 层的数据传递
type ConversationDTO struct {
	ID        int64   // 会话ID
	Type      int16   // 会话类型：1=单聊，2=群聊
	Name      string  // 会话名称
	GroupID   int64   // 群聊关联的群组ID，单聊时为0。前端通过此字段将会话与群组关联
	MemberIDs []int64 // 成员用户ID列表
}

// CreateConversation 创建群聊会话（显式创建）
// 单聊会话无需调用此方法，由 SendMessage 隐式创建
// 此接口仅用于群聊场景，会话类型硬编码为 ConvTypeGroup（2）
// 移除 convType 参数的原因：
//   - 单聊会话由 GetOrCreatePrivateConversation 隐式创建，不应走此接口
//   - 允许外部传入 convType 存在误用风险（如传 type=1 绕过隐式创建逻辑）
//   - 显式创建 = 群聊，这是业务语义的硬约束
//
// 参数:
//   - name: 会话名称，群聊必填
//   - memberIDs: 成员用户ID列表，第一个元素为创建者
//   - groupID: 关联的群组ID，由 group_service 传入，用于前端将会话与群组关联
//
// 返回值: 新创建的会话ID，或错误信息
func (svc *ChatService) CreateConversation(ctx context.Context, name string, memberIDs []int64, groupID int64) (int64, error) {
	convID := getConvSnowNode().Generate()
	conv := &model.Conversation{
		ID:        convID,
		Type:      model.ConvTypeGroup,
		Name:      name,
		GroupID:   groupID,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}
	createdID, err := svc.conversationDao.CreateConversation(ctx, conv, memberIDs)
	if err != nil {
		return 0, err
	}
	return createdID, nil
}

// GetConversations 查询用户参与的所有会话
// 返回值包含每个会话的成员列表，供前端展示会话信息
func (svc *ChatService) GetConversations(ctx context.Context, userID int64) ([]ConversationDTO, error) {
	conversations, err := svc.conversationDao.GetUserConversations(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(conversations) == 0 {
		return nil, nil
	}
	convIDs := make([]int64, len(conversations))
	for i, conv := range conversations {
		convIDs[i] = conv.ID
	}
	membersMap, err := svc.conversationDao.BatchGetConversationMembers(ctx, convIDs)
	if err != nil {
		return nil, err
	}
	var result []ConversationDTO
	for _, conv := range conversations {
		result = append(result, ConversationDTO{
			ID:        conv.ID,
			Type:      conv.Type,
			Name:      conv.Name,
			GroupID:   conv.GroupID,
			MemberIDs: membersMap[conv.ID],
		})
	}
	return result, nil
}

// SetOnline 将用户标记为在线
// 参数 gatewayAddr: 用户连接的 msg_gateway 地址，用于跨网关推送
func (svc *ChatService) SetOnline(ctx context.Context, userID int64, gatewayAddr string) error {
	return svc.onlineDao.SetOnline(ctx, userID, gatewayAddr)
}

// SetOffline 将用户标记为离线
func (svc *ChatService) SetOffline(ctx context.Context, userID int64) error {
	return svc.onlineDao.SetOffline(ctx, userID)
}

// GetOnlineStatus 批量查询用户在线状态
func (svc *ChatService) GetOnlineStatus(ctx context.Context, userIDs []int64) ([]dal.OnlineInfo, error) {
	return svc.onlineDao.GetOnlineStatus(ctx, userIDs)
}

// AddConversationMembers 向已有会话中添加成员
// 由 group_service 通过 RPC 调用，在邀请成员入群后同步会话成员数据
//
// 为什么由 group_service 驱动而非 chat_service 主动同步：
//   - group_service 是群组治理的权威源（Source of Truth），负责权限校验和审批
//   - chat_service 是被动方，只负责根据 group_service 的决策更新消息路由数据
//   - 这种"命令式同步"比"事件式同步"更简单可靠，避免最终一致性的延迟问题
//
// 安全校验：
//   - 校验会话是否存在且为群聊类型，防止对单聊会话操作或操作不存在的会话
func (svc *ChatService) AddConversationMembers(ctx context.Context, conversationID int64, memberIDs []int64) error {
	conv, err := svc.conversationDao.GetConversationInfo(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("查询会话信息失败: %w", err)
	}
	if conv == nil || conv.ID == 0 {
		return fmt.Errorf("会话不存在")
	}
	if conv.Type != model.ConvTypeGroup {
		return fmt.Errorf("仅群聊会话支持添加成员")
	}
	return svc.conversationDao.AddMembers(ctx, conversationID, memberIDs)
}

// RemoveConversationMembers 从已有会话中移除成员
// 由 group_service 通过 RPC 调用，在踢出成员后同步会话成员数据
//
// 同步的必要性：
//   - 被踢出的成员必须从 conversation_member 中移除
//   - 否则 GetConversationMembers 仍会返回该用户，导致消息推送给已退群的人
//
// 安全校验：
//   - 校验会话是否存在且为群聊类型，防止对单聊会话操作或操作不存在的会话
func (svc *ChatService) RemoveConversationMembers(ctx context.Context, conversationID int64, memberIDs []int64) error {
	conv, err := svc.conversationDao.GetConversationInfo(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("查询会话信息失败: %w", err)
	}
	if conv == nil || conv.ID == 0 {
		return fmt.Errorf("会话不存在")
	}
	if conv.Type != model.ConvTypeGroup {
		return fmt.Errorf("仅群聊会话支持移除成员")
	}
	return svc.conversationDao.RemoveMembers(ctx, conversationID, memberIDs)
}
