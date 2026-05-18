package model

// Message 聊天消息模型，对应 PostgreSQL 中的 message_table
// 以 conversation_id 为分区键，支持按会话维度查询和推送
type Message struct {
	MsgID          int64  `gorm:"primaryKey;autoIncrement:false" json:"msg_id"`                     // 消息唯一ID，由 Snowflake 算法生成，不自增
	ClientSeq      int64  `gorm:"not null;default:0" json:"client_seq"`                             // 客户端序列号，用于客户端去重和排序
	SenderID       int64  `gorm:"not null;index:idx_sender" json:"sender_id"`                       // 发送者用户ID，建立索引用于查询某用户发送的所有消息
	ConversationID int64  `gorm:"not null;index:idx_conversation_timestamp" json:"conversation_id"` // 所属会话ID，与 Timestamp 组成复合索引
	Content        string `gorm:"type:text;not null" json:"content"`                                // 消息文本内容
	Timestamp      int64  `gorm:"not null;index:idx_conversation_timestamp" json:"timestamp"`       // 消息发送时间戳（毫秒），与 ConversationID 组成复合索引，支持按时间翻页
}

func (Message) TableName() string {
	return "message_table"
}

// 会话类型常量
const (
	ConvTypePrivate int16 = 1 // 单聊会话：仅包含两个成员
	ConvTypeGroup   int16 = 2 // 群聊会话：包含多个成员
)

// Conversation 会话模型，统一单聊和群聊
// 单聊和群聊共享同一张表，通过 Type 字段区分
type Conversation struct {
	ID        int64  `gorm:"primaryKey;autoIncrement:false" json:"id"`          // 会话唯一ID，由 Snowflake 算法（节点4）生成，不自增
	Type      int16  `gorm:"not null" json:"type"`                              // 会话类型：1=单聊，2=群聊
	Name      string `gorm:"type:varchar(128);not null;default:''" json:"name"` // 会话名称，单聊时可为空，群聊时必填
	GroupID   int64  `gorm:"default:0;index" json:"group_id"`                   // 群聊关联的群组ID，单聊时为0。用于前端将会话与群组关联
	CreatedAt int64  `gorm:"not null" json:"created_at"`                        // 创建时间（毫秒时间戳）
	UpdatedAt int64  `gorm:"not null" json:"updated_at"`                        // 最后更新时间（毫秒时间戳），用于会话列表排序
}

func (Conversation) TableName() string {
	return "conversation_table"
}

// 会话成员角色常量
const (
	MemberRoleCreator int16 = 1 // 创建者：群聊中拥有最高权限（转让群主、设置管理员等）
	MemberRoleNormal  int16 = 2 // 普通成员：默认角色
)

// ConversationMember 会话成员关联模型
// 使用 (ConversationID, UserID) 作为联合主键，一个用户在同一会话中只能有一条记录
type ConversationMember struct {
	ConversationID int64 `gorm:"primaryKey" json:"conversation_id"` // 所属会话ID
	UserID         int64 `gorm:"primaryKey" json:"user_id"`         // 用户ID
	Role           int16 `gorm:"not null;default:2" json:"role"`    // 角色：1=创建者，2=普通成员
	JoinedAt       int64 `gorm:"not null" json:"joined_at"`         // 加入时间（毫秒时间戳）
}

func (ConversationMember) TableName() string {
	return "conversation_member"
}
