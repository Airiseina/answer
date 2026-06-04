package dal

import (
	"context"

	"github.com/Airiseina/answer/kitex_service/chat_service/internal/model"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ConvSeqPair 会话序号对，用于 SyncMessages 接口
// 客户端上报每个会话本地已同步的最大 seq，服务端据此拉取增量消息
type ConvSeqPair struct {
	ConversationID int64
	LastSeq        int64
}

// ==================== DAO 实现结构体 ====================

// chatDao 消息数据访问对象，操作 PostgreSQL 和 Redis
type chatDao struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewChatDao 创建消息 DAO 实例
func NewChatDao(db *gorm.DB, rdb *redis.Client) ChatDao {
	return &chatDao{db: db, rdb: rdb}
}

// onlineDao 在线状态数据访问对象，操作 Redis
type onlineDao struct {
	rdb *redis.Client
}

// NewOnlineDao 创建在线状态 DAO 实例
func NewOnlineDao(rdb *redis.Client) OnlineDao {
	return &onlineDao{rdb}
}

// conversationDao 会话数据访问对象，同时操作 PostgreSQL 和 Redis
// PostgreSQL 存储会话和成员的持久化数据，Redis 缓存热点数据以加速查询
type conversationDao struct {
	db  *gorm.DB      // PostgreSQL 连接，用于持久化读写
	rdb *redis.Client // Redis 连接，用于缓存会话对映射和成员列表
}

// NewConversationDao 创建会话 DAO 实例
func NewConversationDao(db *gorm.DB, rdb *redis.Client) ConversationDao {
	return &conversationDao{db: db, rdb: rdb}
}

// inboxDao 写扩散收件箱数据访问对象，操作 PostgreSQL
type inboxDao struct {
	db *gorm.DB
}

// NewInboxDao 创建收件箱 DAO 实例
func NewInboxDao(db *gorm.DB) InboxDao {
	return &inboxDao{db}
}

// coldStorageDao 冷库数据访问对象，操作 ClickHouse
type coldStorageDao struct {
	chConn clickhouse.Conn
}

// NewColdStorageDao 创建冷库 DAO 实例
func NewColdStorageDao(chConn clickhouse.Conn) ColdStorageDao {
	return &coldStorageDao{chConn: chConn}
}

// ==================== DAO 接口定义 ====================

// ChatDao 消息数据访问接口
type ChatDao interface {
	// CreateMessage 将消息写入 PostgreSQL
	// 参数 msg: 已填充完整字段的消息对象（含 Snowflake 生成的 MsgID）
	// 返回值: 写入失败时返回 error
	CreateMessage(msg *model.Message) error

	// GetHistory 按会话ID拉取历史消息，支持基于 msg_id 的游标翻页
	// 参数 conversationID: 会话ID
	// 参数 beforeMsgID: 游标，返回 msg_id < beforeMsgID 的消息；传 0 表示从最新消息开始
	// 参数 limit: 返回条数上限
	// 返回值: 消息列表（按 msg_id 降序），查询失败时返回 error
	GetHistory(conversationID int64, beforeMsgID int64, limit int16) ([]model.Message, error)

	// GetMessage 根据消息ID查询单条消息
	// 用于撤回/编辑时校验消息是否存在、是否为发送者本人、是否已撤回等
	// 参数 msgID: 消息ID
	// 返回值: 消息对象指针，不存在时返回 nil, nil
	GetMessage(msgID int64) (*model.Message, error)

	// RecallMessage 将消息状态更新为已撤回
	// 仅更新 status 字段，不修改 content（保留原始内容用于审计）
	// 参数 msgID: 消息ID
	// 返回值: 错误信息
	RecallMessage(msgID int64) error

	// EditMessage 更新消息内容并标记为已编辑
	// 参数 msgID: 消息ID
	// 参数 newContent: 新的消息内容（已 Normalize）
	// 返回值: 错误信息
	EditMessage(msgID int64, newContent string) error

	// SaveEditHistory 保存一条编辑历史记录
	// 每次编辑消息前，将旧内容存入 message_edit_history 表
	// 参数 history: 编辑历史记录对象
	// 返回值: 错误信息
	SaveEditHistory(history *model.MessageEditHistory) error

	// GetEditHistory 查询消息的编辑历史，按版本号升序
	// 参数 msgID: 消息ID
	// 返回值: 编辑历史列表、错误信息
	GetEditHistory(msgID int64) ([]model.MessageEditHistory, error)

	// GetLatestEditVersion 获取消息的最新编辑版本号
	// 用于生成下一个版本号：新版本 = 最新版本 + 1
	// 参数 msgID: 消息ID
	// 返回值: 最新版本号（无记录时返回 0）、错误信息
	GetLatestEditVersion(msgID int64) (int32, error)

	// GetMessagesAfterSeq 按会话ID拉取 seq > afterSeq 的消息，按 seq 升序排列
	// 用于 Phase 6 上线同步：客户端上报本地最大 seq，服务端返回增量消息
	// 参数 conversationID: 会话ID
	// 参数 afterSeq: 客户端本地最大已同步seq
	// 参数 limit: 返回条数上限
	// 返回值: 消息列表（按 seq 升序），查询失败时返回 error
	GetMessagesAfterSeq(conversationID int64, afterSeq int64, limit int16) ([]model.Message, error)

	// CacheMessage 将消息写入 Redis 缓存，加速首屏加载
	// 使用 Redis List 结构，每个会话保留最近 N 条消息
	// 参数 ctx: 上下文
	// 参数 msg: 消息对象
	CacheMessage(ctx context.Context, msg *model.Message)

	// GetCachedMessages 从 Redis 缓存获取会话最近 N 条消息
	// 缓存未命中时返回空切片和 nil 错误，由调用方决定是否回源数据库
	// 参数 ctx: 上下文
	// 参数 conversationID: 会话ID
	// 参数 limit: 返回条数上限
	// 返回值: 消息列表、是否命中缓存、错误信息
	GetCachedMessages(ctx context.Context, conversationID int64, limit int16) ([]model.Message, bool, error)

	// GetHotMessagesBeforeTime 查询热库中指定时间之前的消息（用于冷热分界判断）
	// 参数 conversationID: 会话ID
	// 参数 beforeTimestamp: 时间戳（毫秒），返回 timestamp < beforeTimestamp 的消息
	// 参数 limit: 返回条数上限
	// 返回值: 消息列表、错误信息
	GetHotMessagesBeforeTime(conversationID int64, beforeTimestamp int64, limit int16) ([]model.Message, error)

	// DeleteMessagesBeforeTime 删除热库中指定时间之前的消息（归档后清理）
	// 参数 beforeTimestamp: 时间戳（毫秒），删除 timestamp < beforeTimestamp 的消息
	// 返回值: 删除的行数、错误信息
	DeleteMessagesBeforeTime(beforeTimestamp int64) (int64, error)
}

// OnlineDao 在线状态数据访问接口
type OnlineDao interface {
	// SetOnline 将用户标记为在线，并记录其连接的网关地址
	// 参数 userID: 用户ID
	// 参数 gatewayAddr: 用户所连接的 msg_gateway 地址（格式 host:port）
	SetOnline(ctx context.Context, userID int64, gatewayAddr string) error

	// SetOffline 将用户标记为离线，清除其在线状态和网关地址
	SetOffline(ctx context.Context, userID int64) error

	// GetOnlineStatus 批量查询用户在线状态
	// 参数 userIDs: 待查询的用户ID列表
	// 返回值: 每个用户的在线信息（是否在线、网关地址）
	GetOnlineStatus(ctx context.Context, userIDs []int64) ([]OnlineInfo, error)

	// IsOnline 查询单个用户的在线状态
	// 返回值: 是否在线、网关地址、错误信息
	IsOnline(ctx context.Context, userID int64) (bool, string, error)

	// RenewOnline 续期用户在线状态，防止 Redis key 过期
	RenewOnline(ctx context.Context, userID int64) error
}

// ConversationDao 会话数据访问接口
type ConversationDao interface {
	// CreateConversation 创建会话并插入所有成员记录（在同一个数据库事务中）
	// 参数 conv: 会话对象（需预填充 ID、Type、Name、时间戳）
	// 参数 memberIDs: 成员用户ID列表，第一个元素将被设为创建者（Role=1）
	// 返回值: 会话ID、错误信息
	CreateConversation(ctx context.Context, conv *model.Conversation, memberIDs []int64) (int64, error)

	// GetOrCreatePrivateConversation 获取或创建单聊会话（隐式创建的核心方法）
	// 查找顺序：Redis 缓存 → 数据库 → 加锁创建
	// 参数 userA, userB: 两个用户的ID，顺序无关（内部会按大小排序保证一致性）
	// 参数 idGen: 会话ID生成函数，由 Service 层提供，保持 DAL 层不依赖 ID 生成策略
	// 返回值: 会话ID、错误信息
	GetOrCreatePrivateConversation(ctx context.Context, userA, userB int64, idGen func() int64) (int64, error)

	// GetConversationMembers 查询会话的所有成员ID
	// 优先从 Redis 缓存读取，缓存未命中时回源数据库并回填缓存
	// 参数 conversationID: 会话ID
	// 返回值: 成员用户ID列表、错误信息
	GetConversationMembers(ctx context.Context, conversationID int64) ([]int64, error)

	// GetUserConversations 查询用户参与的所有会话
	// 参数 userID: 用户ID
	// 返回值: 会话列表（按 updated_at 降序排列）、错误信息
	GetUserConversations(ctx context.Context, userID int64) ([]model.Conversation, error)

	// GetConversationInfo 查询单个会话的详细信息
	// 参数 conversationID: 会话ID
	// 返回值: 会话对象指针、错误信息
	GetConversationInfo(ctx context.Context, conversationID int64) (*model.Conversation, error)

	// AddMembers 向已有会话中添加成员
	// 用于群聊场景：group_service 邀请成员入群后，通过 RPC 同步会话成员
	// 操作流程：批量插入 conversation_member 记录 → 删除成员缓存（触发下次查询时重建）
	// 参数 conversationID: 目标会话ID
	// 参数 memberIDs: 待添加的用户ID列表
	// 返回值: 错误信息
	AddMembers(ctx context.Context, conversationID int64, memberIDs []int64) error

	// RemoveMembers 从已有会话中移除成员
	// 用于群聊场景：group_service 踢出成员后，通过 RPC 同步会话成员
	// 操作流程：删除 conversation_member 记录 → 删除成员缓存（触发下次查询时重建）
	// 参数 conversationID: 目标会话ID
	// 参数 memberIDs: 待移除的用户ID列表
	// 返回值: 错误信息
	RemoveMembers(ctx context.Context, conversationID int64, memberIDs []int64) error

	// BatchGetConversationMembers 批量查询多个会话的成员列表
	// 优先从 Redis 缓存读取，缓存未命中时回源数据库并回填缓存
	// 用于 GetConversations 等需要一次性获取多个会话成员的场景，避免 N+1 查询
	// 参数 conversationIDs: 会话ID列表
	// 返回值: map[conversationID][]memberID、错误信息
	BatchGetConversationMembers(ctx context.Context, conversationIDs []int64) (map[int64][]int64, error)

	// DeleteConversation 删除会话及其所有成员记录（事务操作）
	// 用于 CreateGroup 补偿回滚：群组创建失败时删除已创建的会话
	DeleteConversation(ctx context.Context, conversationID int64) error

	// IncrConvMaxSeq 原子递增会话的最大消息序号，并返回递增后的值
	// 使用 Redis INCR 实现原子递增，保证并发安全
	// Key 格式: conv:max_seq:{conversationID}
	// 每次发送消息时调用，生成该会话内单调递增的 seq
	// 参数 conversationID: 会话ID
	// 返回值: 递增后的 seq 值、错误信息
	IncrConvMaxSeq(ctx context.Context, conversationID int64) (int64, error)

	// GetConvMaxSeq 获取会话的当前最大消息序号
	// 优先从 Redis 读取，Key 不存在时返回 0（表示该会话尚无消息）
	// 参数 conversationID: 会话ID
	// 返回值: 当前最大 seq（0 表示无消息）、错误信息
	GetConvMaxSeq(ctx context.Context, conversationID int64) (int64, error)

	// BatchGetConvMaxSeq 批量获取多个会话的最大消息序号
	// 使用 Redis Pipeline 批量读取，减少网络往返
	// 用于会话列表接口，一次性获取所有会话的 max_seq 以计算未读数
	// 参数 conversationIDs: 会话ID列表
	// 返回值: map[conversationID]maxSeq、错误信息
	BatchGetConvMaxSeq(ctx context.Context, conversationIDs []int64) (map[int64]int64, error)

	// UpdateMemberReadSeq 更新会话成员的已读序号
	// 写扩散模型的核心写入操作：同时更新 PostgreSQL（持久化）和 Redis（缓存）
	// PG 更新: UPDATE conversation_member SET max_read_seq = ? WHERE conversation_id = ? AND user_id = ?
	// Redis 更新: SET conv:member:read_seq:{conversationID}:{userID} maxReadSeq
	// 参数 conversationID: 会话ID
	// 参数 userID: 用户ID
	// 参数 maxReadSeq: 用户已读的最大消息序号
	// 返回值: 错误信息
	UpdateMemberReadSeq(ctx context.Context, conversationID int64, userID int64, maxReadSeq int64) error

	// GetMemberReadSeq 获取会话成员的已读序号
	// 优先从 Redis 缓存读取，缓存未命中时回源 PostgreSQL 并回填缓存
	// 用于未读数计算：unread = conv:max_seq - member:max_read_seq
	// 参数 conversationID: 会话ID
	// 参数 userID: 用户ID
	// 返回值: 用户已读的最大消息序号（0 表示未读任何消息）、错误信息
	GetMemberReadSeq(ctx context.Context, conversationID int64, userID int64) (int64, error)

	// BatchGetMemberReadSeq 批量获取用户在多个会话中的已读序号
	// 使用 Redis Pipeline 批量读取，缓存未命中时回源 PostgreSQL 并回填
	// 用于会话列表接口，一次性获取所有会话的已读序号以计算未读数
	// 参数 userID: 用户ID
	// 参数 conversationIDs: 会话ID列表
	// 返回值: map[conversationID]maxReadSeq、错误信息
	BatchGetMemberReadSeq(ctx context.Context, userID int64, conversationIDs []int64) (map[int64]int64, error)
}

// InboxDao 写扩散收件箱数据访问接口
// 小群（≤100人）采用写扩散：发消息时为每个成员写入一条收件箱记录
// 拉取消息时直接查个人收件箱，无需实时计算，读取快
type InboxDao interface {
	// BatchCreateInboxMessages 批量写入收件箱消息
	// 写扩散的核心写入操作：为会话中的每个成员创建一条收件箱记录
	// 参数 messages: 收件箱消息列表
	// 返回值: 错误信息
	BatchCreateInboxMessages(ctx context.Context, messages []model.InboxMessage) error

	// GetInboxMessages 查询用户的收件箱消息（按会话维度）
	// 写扩散的核心读取操作：直接查个人收件箱，无需关联会话表
	// 参数 userID: 用户ID
	// 参数 conversationID: 会话ID
	// 参数 beforeSeq: 游标，返回 seq < beforeSeq 的消息；传 0 表示从最新开始
	// 参数 limit: 返回条数上限
	// 返回值: 消息列表（按 seq 降序）、错误信息
	GetInboxMessages(ctx context.Context, userID int64, conversationID int64, beforeSeq int64, limit int16) ([]model.InboxMessage, error)

	// GetInboxMessagesAfterSeq 查询用户收件箱中 seq > afterSeq 的消息
	// 用于上线同步：客户端上报本地最大 seq，服务端返回增量消息
	// 参数 userID: 用户ID
	// 参数 conversationID: 会话ID
	// 参数 afterSeq: 客户端本地最大已同步seq
	// 参数 limit: 返回条数上限
	// 返回值: 消息列表（按 seq 升序）、错误信息
	GetInboxMessagesAfterSeq(ctx context.Context, userID int64, conversationID int64, afterSeq int64, limit int16) ([]model.InboxMessage, error)

	// RecallInboxMessage 更新收件箱中消息的撤回状态
	// 撤回消息时需要更新所有成员收件箱中对应消息的状态
	// 参数 msgID: 消息ID
	// 返回值: 错误信息
	RecallInboxMessage(ctx context.Context, msgID int64) error

	// EditInboxMessage 更新收件箱中消息的内容和编辑状态
	// 编辑消息时需要更新所有成员收件箱中对应消息的内容
	// 参数 msgID: 消息ID
	// 参数 newContent: 新的消息内容
	// 返回值: 错误信息
	EditInboxMessage(ctx context.Context, msgID int64, newContent string) error

	// GetInboxSeqByMsgID 根据消息ID查询收件箱中的 seq
	// 用于 GetHistory 翻页游标转换：客户端传 beforeMsgID，需转为 beforeSeq 查收件箱
	// 参数 userID: 用户ID
	// 参数 conversationID: 会话ID
	// 参数 msgID: 消息ID
	// 返回值: 对应的 seq（0 表示未找到）、错误信息
	GetInboxSeqByMsgID(ctx context.Context, userID int64, conversationID int64, msgID int64) (int64, error)
}

// ColdStorageDao 冷库数据访问接口
// 历史消息异步归档至 ClickHouse，当用户搜索或拉取更早历史时路由到冷库查询
type ColdStorageDao interface {
	// ArchiveMessages 批量归档消息到 ClickHouse 冷库
	// 将 PostgreSQL 中的历史消息批量写入 ClickHouse
	// 参数 messages: 待归档的冷库消息列表
	// 返回值: 错误信息
	ArchiveMessages(ctx context.Context, messages []model.ColdMessage) error

	// GetColdHistory 从冷库拉取历史消息
	// 当热库查询不到足够消息时，路由到冷库查询更早的历史
	// 参数 conversationID: 会话ID
	// 参数 beforeTimestamp: 游标，返回 timestamp < beforeTimestamp 的消息
	// 参数 limit: 返回条数上限
	// 返回值: 冷库消息列表、错误信息
	GetColdHistory(ctx context.Context, conversationID int64, beforeTimestamp int64, limit int16) ([]model.ColdMessage, error)

	// SearchMessages 在冷库中搜索消息
	// ClickHouse 支持海量数据的高效文本搜索和分析查询
	// 参数 conversationID: 会话ID（0 表示搜索所有会话）
	// 参数 keyword: 搜索关键词
	// 参数 userID: 用户ID（用于权限校验，只搜索用户参与的会话）
	// 参数 limit: 返回条数上限
	// 返回值: 冷库消息列表、错误信息
	SearchMessages(ctx context.Context, conversationID int64, keyword string, userID int64, limit int16) ([]model.ColdMessage, error)
}
