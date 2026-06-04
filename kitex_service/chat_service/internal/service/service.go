package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Airiseina/answer/kitex_service/chat_service/internal/dal"
	"github.com/Airiseina/answer/kitex_service/chat_service/internal/model"
	"github.com/Airiseina/answer/kitex_service/chat_service/rpc"
	"github.com/Airiseina/answer/pkg/snowflake"
	"gorm.io/gorm"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/redis/go-redis/v9"
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
// 职责：消息收发、会话管理、在线状态管理、ID 生成、冷热路由、写扩散/读扩散
// 边界：仅处理业务逻辑，不直接操作数据库/缓存（通过 DAO 接口），不处理协议转换（由 handler 层负责）
type ChatService struct {
	dao             dal.ChatDao         // 消息数据访问（热库 PostgreSQL + Redis 缓存）
	onlineDao       dal.OnlineDao       // 在线状态数据访问
	conversationDao dal.ConversationDao // 会话数据访问
	inboxDao        dal.InboxDao        // 写扩散收件箱数据访问（小群写扩散）
	coldStorageDao  dal.ColdStorageDao  // 冷库数据访问（ClickHouse）
	msgSnowNode     *snowflake.Node     // 消息ID的 Snowflake 节点（节点编号 3）
	rdb             *redis.Client       // Redis 客户端，用于 client_seq 去重
	coldEnabled     bool                // 是否启用冷库归档
	hotMonths       int                 // 热数据保留月数
}

func NewChatService(dao dal.ChatDao, onlineDao dal.OnlineDao, conversationDao dal.ConversationDao, inboxDao dal.InboxDao, coldStorageDao dal.ColdStorageDao, rdb *redis.Client, coldEnabled bool, hotMonths int) *ChatService {
	return &ChatService{
		dao:             dao,
		onlineDao:       onlineDao,
		conversationDao: conversationDao,
		inboxDao:        inboxDao,
		coldStorageDao:  coldStorageDao,
		msgSnowNode:     snowflake.NewNode(3),
		rdb:             rdb,
		coldEnabled:     coldEnabled,
		hotMonths:       hotMonths,
	}
}

// SendMessageResult 发送消息的返回结果
type SendMessageResult struct {
	MsgID            int64
	Seq              int64
	Timestamp        int64
	ConversationID   int64
	ConversationType int16
	MemberIDs        []int64
	Content          string
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
func (svc *ChatService) SendMessage(ctx context.Context, senderID int64, conversationID int64, peerID int64, content string, clientSeq int64, quoteMsgID int64) (*SendMessageResult, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}
	normalizedContent := model.NormalizeContent(content)
	// 步骤1：确定会话ID
	var convID int64
	if conversationID == 0 && peerID != 0 {
		if peerID == senderID {
			return nil, fmt.Errorf("不能给自己发消息")
		}
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
	convInfo, err := svc.conversationDao.GetConversationInfo(ctx, convID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) && peerID != 0 {
			newConvID, createErr := svc.conversationDao.GetOrCreatePrivateConversation(ctx, senderID, peerID, func() int64 {
				return getConvSnowNode().Generate()
			})
			if createErr != nil {
				return nil, fmt.Errorf("会话不存在且重建失败: %w", createErr)
			}
			convID = newConvID
			convInfo, err = svc.conversationDao.GetConversationInfo(ctx, convID)
			if err != nil {
				return nil, fmt.Errorf("查询重建会话信息失败: %w", err)
			}
			members, err = svc.conversationDao.GetConversationMembers(ctx, convID)
			if err != nil {
				return nil, fmt.Errorf("查询重建会话成员失败: %w", err)
			}
			isMember = false
			for _, m := range members {
				if m == senderID {
					isMember = true
					break
				}
			}
			if !isMember {
				return nil, fmt.Errorf("发送者不在该会话中")
			}
		} else {
			return nil, fmt.Errorf("查询会话信息失败: %w", err)
		}
	}
	var convType int16
	if convInfo != nil {
		convType = convInfo.Type
	}
	if convType == model.ConvTypeGroup && convInfo.GroupID != 0 {
		muted, muteErr := rpc.CheckMuted(ctx, convInfo.GroupID, senderID)
		if muteErr != nil {
			klog.CtxWarnf(ctx, "查询禁言状态失败, group_id=%d, user_id=%d: %v", convInfo.GroupID, senderID, muteErr)
		} else if muted {
			return nil, fmt.Errorf("你已被禁言")
		}
	}
	if clientSeq > 0 {
		dedupeKey := fmt.Sprintf("msg:dedup:%d:%d", senderID, clientSeq)
		locked, err := svc.rdb.SetNX(ctx, dedupeKey, 1, 5*time.Minute).Result()
		if err != nil {
			return nil, fmt.Errorf("消息去重检查失败: %w", err)
		}
		if !locked {
			return nil, fmt.Errorf("重复消息")
		}
	}
	// 步骤3：生成消息ID，写入数据库
	msgID := svc.msgSnowNode.Generate()
	// 原子递增会话的 max_seq，生成该消息在会话内的单调递增序号
	seq, seqErr := svc.conversationDao.IncrConvMaxSeq(ctx, convID)
	if seqErr != nil {
		return nil, fmt.Errorf("生成消息序号失败: %w", seqErr)
	}
	now := time.Now().UnixMilli()
	msg := &model.Message{
		MsgID:          msgID,
		ClientSeq:      clientSeq,
		SenderID:       senderID,
		ConversationID: convID,
		Seq:            seq,
		Content:        normalizedContent,
		Timestamp:      now,
		QuoteMsgID:     quoteMsgID,
	}
	err = svc.dao.CreateMessage(msg)
	if err != nil {
		return nil, err
	}
	// 写扩散：小群（≤100人）为每个成员写入收件箱记录
	// 小群写扩散优势：拉取消息时直接查个人收件箱，无需实时计算，读取快
	// 大群（>100人）采用读扩散：只写群信箱（message_table），拉取时实时计算，节省写压力
	if len(members) <= model.WriteDiffusionThreshold {
		inboxMessages := make([]model.InboxMessage, 0, len(members))
		for _, memberID := range members {
			inboxMessages = append(inboxMessages, model.InboxMessage{
				UserID:         memberID,
				ConversationID: convID,
				MsgID:          msgID,
				SenderID:       senderID,
				Seq:            seq,
				Content:        normalizedContent,
				Timestamp:      now,
				QuoteMsgID:     quoteMsgID,
			})
		}
		if inboxErr := svc.inboxDao.BatchCreateInboxMessages(ctx, inboxMessages); inboxErr != nil {
			klog.CtxWarnf(ctx, "写扩散写入收件箱失败: %v", inboxErr)
		}
	}
	// 消息缓存：将消息写入 Redis，加速首屏加载
	svc.dao.CacheMessage(ctx, msg)
	// 设置撤回窗口键：recall:msg:{msgID}，TTL=2min
	// 撤回时通过 EXISTS 判断该键是否存在，存在则可撤回，不存在则超时
	recallKey := fmt.Sprintf("recall:msg:%d", msgID)
	if err := svc.rdb.Set(ctx, recallKey, 1, 2*time.Minute).Err(); err != nil {
		klog.CtxWarnf(ctx, "设置撤回窗口键失败: %v", err)
	}
	return &SendMessageResult{
		MsgID:            msgID,
		Seq:              seq,
		Timestamp:        now,
		ConversationID:   convID,
		ConversationType: convType,
		MemberIDs:        members,
		Content:          normalizedContent,
	}, nil
}

// MessageDTO 消息数据传输对象，用于 Service → Handler 层的数据传递
type MessageDTO struct {
	ClientSeq      int64  // 客户端序列号
	MsgID          int64  // 消息唯一ID
	SenderID       int64  // 发送者用户ID
	ConversationID int64  // 所属会话ID
	Seq            int64  // 会话内消息序号，用于未读数计算和消息同步
	Content        string // 消息内容
	Timestamp      int64  // 发送时间戳（毫秒）
	Status         int16  // 消息状态：0=正常，1=已撤回
	IsEdited       bool   // 是否已编辑
	QuoteMsgID     int64  // 引用消息ID
}

// GetHistory 拉取会话历史消息（支持冷热路由）
// 查询优先级：Redis 缓存 → PostgreSQL 热库 → ClickHouse 冷库
// 冷热分界：近 hotMonths 个月的消息在热库，更早的消息在冷库
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
	if beforeMsgID < 0 {
		beforeMsgID = 0
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

	// 判断是否使用写扩散路径（小群从收件箱读取）
	useWriteDiffusion := len(members) <= model.WriteDiffusionThreshold

	var msgs []MessageDTO

	// 1. 首屏加载（beforeMsgID == 0）：优先从 Redis 缓存读取
	if beforeMsgID == 0 {
		cachedMessages, hit, _ := svc.dao.GetCachedMessages(ctx, conversationID, limit)
		if hit && len(cachedMessages) > 0 {
			for _, msg := range cachedMessages {
				content := msg.Content
				if msg.Status == model.MsgStatusRecalled {
					content = "该消息已撤回"
				}
				msgs = append(msgs, MessageDTO{
					ClientSeq:      msg.ClientSeq,
					MsgID:          msg.MsgID,
					SenderID:       msg.SenderID,
					ConversationID: msg.ConversationID,
					Seq:            msg.Seq,
					Content:        content,
					Timestamp:      msg.Timestamp,
					Status:         msg.Status,
					IsEdited:       msg.IsEdited,
					QuoteMsgID:     msg.QuoteMsgID,
				})
			}
			return msgs, nil
		}
	}

	// 2. 写扩散路径：从收件箱读取（小群）
	// 收件箱的游标是 seq，客户端传的是 beforeMsgID，需要先转换为 beforeSeq
	inboxHit := false
	if useWriteDiffusion {
		var beforeSeq int64
		if beforeMsgID > 0 {
			// 将 beforeMsgID 转换为收件箱中的 beforeSeq
			seq, seqErr := svc.inboxDao.GetInboxSeqByMsgID(ctx, userID, conversationID, beforeMsgID)
			if seqErr != nil {
				klog.CtxWarnf(ctx, "查询收件箱seq失败，回退到群信箱: %v", seqErr)
			} else {
				beforeSeq = seq
			}
		}
		inboxMsgs, err := svc.inboxDao.GetInboxMessages(ctx, userID, conversationID, beforeSeq, limit)
		if err != nil {
			klog.CtxWarnf(ctx, "从收件箱读取消息失败，回退到群信箱: %v", err)
		} else if len(inboxMsgs) > 0 {
			inboxHit = true
			for _, msg := range inboxMsgs {
				content := msg.Content
				if msg.Status == model.MsgStatusRecalled {
					content = "该消息已撤回"
				}
				msgs = append(msgs, MessageDTO{
					MsgID:          msg.MsgID,
					SenderID:       msg.SenderID,
					ConversationID: msg.ConversationID,
					Seq:            msg.Seq,
					Content:        content,
					Timestamp:      msg.Timestamp,
					Status:         msg.Status,
					IsEdited:       msg.IsEdited,
					QuoteMsgID:     msg.QuoteMsgID,
				})
			}
			// 写扩散路径数据充足时直接返回，不足时跳过读扩散，直接走冷库补充
			if len(msgs) >= int(limit) {
				return msgs, nil
			}
		}
	}

	// 3. 读扩散路径：从群信箱（热库 PostgreSQL）读取
	// 仅在写扩散未命中时执行，避免与收件箱数据重复
	if !inboxHit {
		messages, err := svc.dao.GetHistory(conversationID, beforeMsgID, limit)
		if err != nil {
			return nil, err
		}
		for _, msg := range messages {
			content := msg.Content
			if msg.Status == model.MsgStatusRecalled {
				content = "该消息已撤回"
			}
			msgs = append(msgs, MessageDTO{
				ClientSeq:      msg.ClientSeq,
				MsgID:          msg.MsgID,
				SenderID:       msg.SenderID,
				ConversationID: msg.ConversationID,
				Seq:            msg.Seq,
				Content:        content,
				Timestamp:      msg.Timestamp,
				Status:         msg.Status,
				IsEdited:       msg.IsEdited,
				QuoteMsgID:     msg.QuoteMsgID,
			})
		}
	}

	// 4. 冷库路由：热库数据不足且启用冷库时，查询 ClickHouse
	if svc.coldEnabled && len(msgs) < int(limit) && svc.coldStorageDao != nil {
		hotBoundary := time.Now().AddDate(0, -svc.hotMonths, 0).UnixMilli()
		var beforeTimestamp int64
		if len(msgs) > 0 {
			beforeTimestamp = msgs[len(msgs)-1].Timestamp
		} else {
			beforeTimestamp = hotBoundary
		}
		remaining := limit - int16(len(msgs))
		coldMessages, coldErr := svc.coldStorageDao.GetColdHistory(ctx, conversationID, beforeTimestamp, remaining)
		if coldErr != nil {
			klog.CtxWarnf(ctx, "查询冷库消息失败: %v", coldErr)
		} else {
			for _, msg := range coldMessages {
				content := msg.Content
				if msg.Status == model.MsgStatusRecalled {
					content = "该消息已撤回"
				}
				msgs = append(msgs, MessageDTO{
					MsgID:          msg.MsgID,
					ClientSeq:      msg.ClientSeq,
					SenderID:       msg.SenderID,
					ConversationID: msg.ConversationID,
					Seq:            msg.Seq,
					Content:        content,
					Timestamp:      msg.Timestamp,
					Status:         msg.Status,
					IsEdited:       msg.IsEdited,
					QuoteMsgID:     msg.QuoteMsgID,
				})
			}
		}
	}

	if len(msgs) == 0 {
		return nil, nil
	}
	return msgs, nil
}

// ConversationDTO 会话数据传输对象，用于 Service → Handler 层的数据传递
type ConversationDTO struct {
	ID          int64   // 会话ID
	Type        int16   // 会话类型：1=单聊，2=群聊
	Name        string  // 会话名称
	GroupID     int64   // 群聊关联的群组ID，单聊时为0。前端通过此字段将会话与群组关联
	MemberIDs   []int64 // 成员用户ID列表
	MaxSeq      int64   // 会话当前最大消息序号，用于客户端同步
	MaxReadSeq  int64   // 用户在该会话中的已读序号，用于客户端计算未读数
	UnreadCount int64   // 未读消息数，由 MaxSeq - MaxReadSeq 计算
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
// 返回值包含每个会话的成员列表、最大消息序号、已读序号和未读数
// 未读数计算公式：unread = conv:max_seq - member:max_read_seq
// 单聊和群聊使用完全一致的计算逻辑，这是统一会话模型的核心优势
func (svc *ChatService) GetConversations(ctx context.Context, userID int64) ([]ConversationDTO, error) {
	conversations, err := svc.conversationDao.GetUserConversations(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(conversations) == 0 {
		return []ConversationDTO{}, nil
	}
	convIDs := make([]int64, len(conversations))
	for i, conv := range conversations {
		convIDs[i] = conv.ID
	}
	membersMap, err := svc.conversationDao.BatchGetConversationMembers(ctx, convIDs)
	if err != nil {
		return nil, err
	}
	// 批量获取所有会话的最大消息序号
	maxSeqMap, err := svc.conversationDao.BatchGetConvMaxSeq(ctx, convIDs)
	if err != nil {
		return nil, fmt.Errorf("批量获取会话最大序号失败: %w", err)
	}
	// 批量获取用户在所有会话中的已读序号
	readSeqMap, err := svc.conversationDao.BatchGetMemberReadSeq(ctx, userID, convIDs)
	if err != nil {
		return nil, fmt.Errorf("批量获取成员已读序号失败: %w", err)
	}
	var result []ConversationDTO
	for _, conv := range conversations {
		maxSeq := maxSeqMap[conv.ID]
		readSeq := readSeqMap[conv.ID]
		unreadCount := maxSeq - readSeq
		if unreadCount < 0 {
			unreadCount = 0
		}
		result = append(result, ConversationDTO{
			ID:          conv.ID,
			Type:        conv.Type,
			Name:        conv.Name,
			GroupID:     conv.GroupID,
			MemberIDs:   membersMap[conv.ID],
			MaxSeq:      maxSeq,
			MaxReadSeq:  readSeq,
			UnreadCount: unreadCount,
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

// RenewOnline 续期用户在线状态
func (svc *ChatService) RenewOnline(ctx context.Context, userID int64) error {
	return svc.onlineDao.RenewOnline(ctx, userID)
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
	if conv.Type != model.ConvTypeGroup && conv.Type != model.ConvTypePrivate {
		return fmt.Errorf("仅群聊和私聊会话支持添加成员")
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

func (svc *ChatService) DeleteConversation(ctx context.Context, conversationID int64) error {
	return svc.conversationDao.DeleteConversation(ctx, conversationID)
}

// MarkRead 标记会话已读
// 写扩散模型的核心读取操作：将用户在该会话中的已读位置更新到当前最大消息序号
//
// 流程：
//  1. 校验用户是否为会话成员（防止越权标记）
//  2. 获取会话当前的最大消息序号（conv:max_seq）
//  3. 更新 PostgreSQL 的 conversation_member.max_read_seq（持久化真相）
//  4. 同步更新 Redis 缓存（加速后续未读数计算）
//
// 参数:
//   - userID: 用户ID
//   - conversationID: 会话ID
//
// 返回值: 更新后的已读序号、错误信息
func (svc *ChatService) MarkRead(ctx context.Context, userID int64, conversationID int64) (int64, error) {
	// 校验用户是否为会话成员
	members, err := svc.conversationDao.GetConversationMembers(ctx, conversationID)
	if err != nil {
		return 0, fmt.Errorf("查询会话成员失败: %w", err)
	}
	isMember := false
	for _, m := range members {
		if m == userID {
			isMember = true
			break
		}
	}
	if !isMember {
		return 0, fmt.Errorf("用户不在该会话中")
	}
	// 获取会话当前最大消息序号
	maxSeq, err := svc.conversationDao.GetConvMaxSeq(ctx, conversationID)
	if err != nil {
		return 0, fmt.Errorf("获取会话最大序号失败: %w", err)
	}
	// 如果当前没有消息，无需标记
	if maxSeq == 0 {
		return 0, nil
	}
	// 更新用户的已读序号（PG + Redis 双写）
	err = svc.conversationDao.UpdateMemberReadSeq(ctx, conversationID, userID, maxSeq)
	if err != nil {
		return 0, fmt.Errorf("更新已读序号失败: %w", err)
	}
	return maxSeq, nil
}

// GetConversationMembers 查询会话的所有成员ID
// 用于 typing 等场景下获取推送目标用户列表
func (svc *ChatService) GetConversationMembers(ctx context.Context, conversationID int64) ([]int64, error) {
	members, err := svc.conversationDao.GetConversationMembers(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("查询会话成员失败: %w", err)
	}
	return members, nil
}

func (svc *ChatService) GetOrCreatePrivateConversation(ctx context.Context, userIDA, userIDB int64) (int64, error) {
	if userIDA == userIDB {
		return 0, fmt.Errorf("不能与自己创建会话")
	}
	convID, err := svc.conversationDao.GetOrCreatePrivateConversation(ctx, userIDA, userIDB, func() int64 {
		return getConvSnowNode().Generate()
	})
	if err != nil {
		return 0, fmt.Errorf("获取或创建单聊会话失败: %w", err)
	}
	return convID, nil
}

// RecallMessageResult 撤回消息的返回结果
type RecallMessageResult struct {
	ConversationID int64
	MemberIDs      []int64
}

// RecallMessage 撤回消息
// 核心逻辑：
//  1. 查询消息，校验消息是否存在
//  2. 权限校验：
//     - 私聊：仅发送者可撤回
//     - 群聊：群主可撤回任何消息，管理员可撤回非群主消息，普通成员仅可撤回自己的消息
//  3. 校验消息是否已被撤回
//  4. 校验消息是否在 2 分钟内（使用 Redis TTL 快速判断）
//  5. 更新 PostgreSQL SET status='recalled'
//  6. 返回会话信息和成员列表（供推送使用）
func (svc *ChatService) RecallMessage(ctx context.Context, userID int64, msgID int64, conversationID int64) (*RecallMessageResult, error) {
	msg, err := svc.dao.GetMessage(msgID)
	if err != nil {
		return nil, fmt.Errorf("查询消息失败: %w", err)
	}
	if msg == nil {
		return nil, fmt.Errorf("消息不存在")
	}
	if msg.ConversationID != conversationID {
		return nil, fmt.Errorf("消息不属于该会话")
	}
	if msg.Status == model.MsgStatusRecalled {
		return nil, fmt.Errorf("消息已被撤回")
	}

	canRecall := false
	convInfo, convErr := svc.conversationDao.GetConversationInfo(ctx, conversationID)
	if convErr != nil || convInfo == nil {
		if msg.SenderID != userID {
			return nil, fmt.Errorf("只能撤回自己发送的消息")
		}
		canRecall = true
	} else if convInfo.Type == model.ConvTypeGroup && convInfo.GroupID != 0 {
		role, roleErr := rpc.GetMemberRole(ctx, convInfo.GroupID, userID)
		if roleErr != nil {
			if msg.SenderID != userID {
				return nil, fmt.Errorf("只能撤回自己发送的消息")
			}
			canRecall = true
		} else {
			switch role {
			case 2:
				canRecall = true
			case 1:
				senderRole, senderRoleErr := rpc.GetMemberRole(ctx, convInfo.GroupID, msg.SenderID)
				if senderRoleErr != nil || senderRole == 2 {
					return nil, fmt.Errorf("管理员无法撤回群主的消息")
				}
				canRecall = true
			default:
				if msg.SenderID != userID {
					return nil, fmt.Errorf("只能撤回自己发送的消息")
				}
				canRecall = true
			}
		}
	} else {
		if msg.SenderID != userID {
			return nil, fmt.Errorf("只能撤回自己发送的消息")
		}
		canRecall = true
	}

	if !canRecall {
		return nil, fmt.Errorf("无权撤回该消息")
	}

	recallKey := fmt.Sprintf("recall:msg:%d", msgID)
	exists, err := svc.rdb.Exists(ctx, recallKey).Result()
	if err != nil {
		return nil, fmt.Errorf("撤回时限校验失败: %w", err)
	}
	if exists == 0 {
		return nil, fmt.Errorf("消息已超过2分钟，无法撤回")
	}
	err = svc.dao.RecallMessage(msgID)
	if err != nil {
		return nil, fmt.Errorf("撤回消息失败: %w", err)
	}
	// 同步更新收件箱中的消息撤回状态（写扩散场景）
	if inboxErr := svc.inboxDao.RecallInboxMessage(ctx, msgID); inboxErr != nil {
		klog.CtxWarnf(ctx, "同步收件箱撤回状态失败: %v", inboxErr)
	}
	members, err := svc.conversationDao.GetConversationMembers(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("查询会话成员失败: %w", err)
	}
	return &RecallMessageResult{
		ConversationID: conversationID,
		MemberIDs:      members,
	}, nil
}

// EditMessageResult 编辑消息的返回结果
type EditMessageResult struct {
	ConversationID int64
	MemberIDs      []int64
	NewContent     string
}

// EditMessage 编辑消息
// 核心逻辑：
//  1. 查询消息，校验消息是否存在
//  2. 校验请求者是否为消息发送者
//  3. 校验消息是否已被撤回（已撤回的消息不可编辑）
//  4. 更新 PostgreSQL SET content=new_content, is_edited=true
//  5. 返回会话信息和成员列表（供推送使用）
func (svc *ChatService) EditMessage(ctx context.Context, userID int64, msgID int64, conversationID int64, newContent string) (*EditMessageResult, error) {
	if strings.TrimSpace(newContent) == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}
	normalizedContent := model.NormalizeContent(newContent)
	msg, err := svc.dao.GetMessage(msgID)
	if err != nil {
		return nil, fmt.Errorf("查询消息失败: %w", err)
	}
	if msg == nil {
		return nil, fmt.Errorf("消息不存在")
	}
	if msg.ConversationID != conversationID {
		return nil, fmt.Errorf("消息不属于该会话")
	}
	if msg.SenderID != userID {
		return nil, fmt.Errorf("只能编辑自己发送的消息")
	}
	if msg.Status == model.MsgStatusRecalled {
		return nil, fmt.Errorf("已撤回的消息不可编辑")
	}
	// 保存旧内容到编辑历史表（版本控制）
	latestVersion, err := svc.dao.GetLatestEditVersion(msgID)
	if err != nil {
		return nil, fmt.Errorf("查询编辑版本失败: %w", err)
	}
	history := &model.MessageEditHistory{
		MsgID:      msgID,
		Version:    latestVersion + 1,
		OldContent: msg.Content,
		EditorID:   userID,
		EditedAt:   time.Now().UnixMilli(),
	}
	if err := svc.dao.SaveEditHistory(history); err != nil {
		return nil, fmt.Errorf("保存编辑历史失败: %w", err)
	}
	err = svc.dao.EditMessage(msgID, normalizedContent)
	if err != nil {
		return nil, fmt.Errorf("编辑消息失败: %w", err)
	}
	// 同步更新收件箱中的消息编辑内容（写扩散场景）
	if inboxErr := svc.inboxDao.EditInboxMessage(ctx, msgID, normalizedContent); inboxErr != nil {
		klog.CtxWarnf(ctx, "同步收件箱编辑内容失败: %v", inboxErr)
	}
	members, err := svc.conversationDao.GetConversationMembers(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("查询会话成员失败: %w", err)
	}
	return &EditMessageResult{
		ConversationID: conversationID,
		MemberIDs:      members,
		NewContent:     normalizedContent,
	}, nil
}

// EditHistoryDTO 编辑历史数据传输对象
type EditHistoryDTO struct {
	ID         int64  `json:"id"`
	MsgID      int64  `json:"msg_id"`
	Version    int32  `json:"version"`
	OldContent string `json:"old_content"`
	EditorID   int64  `json:"editor_id"`
	EditedAt   int64  `json:"edited_at"`
}

// GetEditHistory 查询消息的编辑历史
// 校验请求者是否为会话成员，防止非成员查看编辑历史
func (svc *ChatService) GetEditHistory(ctx context.Context, userID int64, msgID int64, conversationID int64) ([]EditHistoryDTO, error) {
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
		return nil, fmt.Errorf("无权查看该消息的编辑历史")
	}
	histories, err := svc.dao.GetEditHistory(msgID)
	if err != nil {
		return nil, fmt.Errorf("查询编辑历史失败: %w", err)
	}
	var dtos []EditHistoryDTO
	for _, h := range histories {
		dtos = append(dtos, EditHistoryDTO{
			ID:         h.ID,
			MsgID:      h.MsgID,
			Version:    h.Version,
			OldContent: h.OldContent,
			EditorID:   h.EditorID,
			EditedAt:   h.EditedAt,
		})
	}
	return dtos, nil
}

// ConvSyncResult 单个会话的同步结果
type ConvSyncResult struct {
	ConversationID int64
	Messages       []MessageDTO
}

// SyncMessages 上线同步：按会话维度拉取增量消息
// 客户端断线重连后，遍历本地所有会话，携带每个会话的本地最大 seq
// 服务端对每个会话拉取 seq > last_seq 的消息返回
//
// 流程：
//  1. 参数校验：limit 范围 [1, 200]，默认 50
//  2. 遍历 conv_seqs，对每个会话：
//     a. 校验用户是否为会话成员（防止越权拉取）
//     b. 调用 DAO 查询 seq > last_seq 的消息
//     c. 转换为 DTO 返回
//
// 参数:
//   - userID: 请求者用户ID，用于成员身份校验
//   - convSeqs: 各会话的本地最大seq列表
//   - limit: 每个会话最多返回的消息条数
//
// 返回值: 各会话的同步消息结果列表
func (svc *ChatService) SyncMessages(ctx context.Context, userID int64, convSeqs []dal.ConvSeqPair, limit int16) ([]ConvSyncResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var results []ConvSyncResult
	for _, pair := range convSeqs {
		members, err := svc.conversationDao.GetConversationMembers(ctx, pair.ConversationID)
		if err != nil {
			klog.CtxWarnf(ctx, "SyncMessages: 查询会话%d成员失败: %v", pair.ConversationID, err)
			continue
		}
		isMember := false
		for _, m := range members {
			if m == userID {
				isMember = true
				break
			}
		}
		if !isMember {
			klog.CtxWarnf(ctx, "SyncMessages: 用户%d不在会话%d中", userID, pair.ConversationID)
			continue
		}

		var dtos []MessageDTO

		// 写扩散路径：小群（≤100人）从收件箱读取增量消息
		// 与 GetHistory 保持一致，小群优先查个人收件箱
		if len(members) <= model.WriteDiffusionThreshold {
			inboxMsgs, inboxErr := svc.inboxDao.GetInboxMessagesAfterSeq(ctx, userID, pair.ConversationID, pair.LastSeq, limit)
			if inboxErr != nil {
				klog.CtxWarnf(ctx, "SyncMessages: 从收件箱读取会话%d增量消息失败，回退到群信箱: %v", pair.ConversationID, inboxErr)
			} else if len(inboxMsgs) > 0 {
				for _, msg := range inboxMsgs {
					content := msg.Content
					if msg.Status == model.MsgStatusRecalled {
						content = "该消息已撤回"
					}
					dtos = append(dtos, MessageDTO{
						MsgID:          msg.MsgID,
						SenderID:       msg.SenderID,
						ConversationID: msg.ConversationID,
						Seq:            msg.Seq,
						Content:        content,
						Timestamp:      msg.Timestamp,
						Status:         msg.Status,
						IsEdited:       msg.IsEdited,
						QuoteMsgID:     msg.QuoteMsgID,
					})
				}
			}
		}

		// 读扩散路径：大群或收件箱无数据时，从群信箱读取
		if len(dtos) == 0 {
			messages, err := svc.dao.GetMessagesAfterSeq(pair.ConversationID, pair.LastSeq, limit)
			if err != nil {
				klog.CtxWarnf(ctx, "SyncMessages: 查询会话%d增量消息失败: %v", pair.ConversationID, err)
				continue
			}
			if len(messages) == 0 {
				continue
			}
			for _, msg := range messages {
				content := msg.Content
				if msg.Status == model.MsgStatusRecalled {
					content = "该消息已撤回"
				}
				dtos = append(dtos, MessageDTO{
					ClientSeq:      msg.ClientSeq,
					MsgID:          msg.MsgID,
					SenderID:       msg.SenderID,
					ConversationID: msg.ConversationID,
					Seq:            msg.Seq,
					Content:        content,
					Timestamp:      msg.Timestamp,
					Status:         msg.Status,
					IsEdited:       msg.IsEdited,
					QuoteMsgID:     msg.QuoteMsgID,
				})
			}
		}

		if len(dtos) == 0 {
			continue
		}
		results = append(results, ConvSyncResult{
			ConversationID: pair.ConversationID,
			Messages:       dtos,
		})
	}
	return results, nil
}

// ArchiveColdMessages 归档冷数据：将热库中超过 hotMonths 的消息迁移到 ClickHouse 冷库
// 流程：
//  1. 计算冷热分界时间戳（当前时间 - hotMonths 个月）
//  2. 从热库查询分界时间之前的消息（按会话维度批量处理）
//  3. 批量写入 ClickHouse 冷库
//  4. 归档成功后删除热库中已归档的消息
//
// 此方法应由定时任务调用（如每天凌晨3点），避免在业务高峰期执行
func (svc *ChatService) ArchiveColdMessages(ctx context.Context) error {
	if !svc.coldEnabled || svc.coldStorageDao == nil {
		return nil
	}
	hotBoundary := time.Now().AddDate(0, -svc.hotMonths, 0).UnixMilli()
	totalArchived := 0
	batchSize := int16(500)
	// 按会话维度归档：每次处理一个会话的过期消息
	// 先查询会话0的过期消息作为入口，然后逐会话处理
	for {
		messages, err := svc.dao.GetHotMessagesBeforeTime(0, hotBoundary, batchSize)
		if err != nil {
			klog.CtxWarnf(ctx, "查询热库过期消息失败: %v", err)
			break
		}
		if len(messages) == 0 {
			break
		}
		// 转换为冷库消息格式
		coldMessages := make([]model.ColdMessage, 0, len(messages))
		for _, msg := range messages {
			coldMessages = append(coldMessages, model.ColdMessage{
				MsgID:          msg.MsgID,
				ClientSeq:      msg.ClientSeq,
				SenderID:       msg.SenderID,
				ConversationID: msg.ConversationID,
				Seq:            msg.Seq,
				Content:        msg.Content,
				Status:         msg.Status,
				IsEdited:       msg.IsEdited,
				Timestamp:      msg.Timestamp,
				QuoteMsgID:     msg.QuoteMsgID,
				ArchivedAt:     time.Now().UnixMilli(),
			})
		}
		// 写入冷库
		if err := svc.coldStorageDao.ArchiveMessages(ctx, coldMessages); err != nil {
			klog.CtxWarnf(ctx, "归档消息到冷库失败: %v", err)
			break
		}
		totalArchived += len(coldMessages)
		// 如果本批不足 batchSize，说明已处理完
		if len(messages) < int(batchSize) {
			break
		}
	}
	// 归档完成后清理热库
	if totalArchived > 0 {
		deleted, err := svc.dao.DeleteMessagesBeforeTime(hotBoundary)
		if err != nil {
			return fmt.Errorf("清理热库归档消息失败: %w", err)
		}
		klog.Infof("归档完成: 共归档%d条消息, 清理热库%d条记录", totalArchived, deleted)
	}
	return nil
}

// SearchColdMessages 在冷库中搜索消息
// 当用户需要搜索历史消息时，路由到 ClickHouse 进行高效搜索
func (svc *ChatService) SearchColdMessages(ctx context.Context, conversationID int64, keyword string, userID int64, limit int16) ([]MessageDTO, error) {
	if !svc.coldEnabled || svc.coldStorageDao == nil {
		return nil, fmt.Errorf("冷库未启用")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	coldMessages, err := svc.coldStorageDao.SearchMessages(ctx, conversationID, keyword, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("搜索冷库消息失败: %w", err)
	}
	var msgs []MessageDTO
	for _, msg := range coldMessages {
		content := msg.Content
		if msg.Status == model.MsgStatusRecalled {
			content = "该消息已撤回"
		}
		msgs = append(msgs, MessageDTO{
			MsgID:          msg.MsgID,
			ClientSeq:      msg.ClientSeq,
			SenderID:       msg.SenderID,
			ConversationID: msg.ConversationID,
			Seq:            msg.Seq,
			Content:        content,
			Timestamp:      msg.Timestamp,
			Status:         msg.Status,
			IsEdited:       msg.IsEdited,
			QuoteMsgID:     msg.QuoteMsgID,
		})
	}
	return msgs, nil
}
