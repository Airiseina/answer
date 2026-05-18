package dal

import (
	"chat_service/internal/model"
	"context"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ==================== DAO 实现结构体 ====================

// chatDao 消息数据访问对象，操作 PostgreSQL
type chatDao struct {
	db *gorm.DB
}

// NewChatDao 创建消息 DAO 实例
func NewChatDao(db *gorm.DB) ChatDao {
	return &chatDao{db}
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
}
