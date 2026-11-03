package snowflake

// 各业务实体使用的 snowflake 分片（workerID）编号。
//
// 分片编号决定 ID 的高位段，必须全局唯一，否则不同服务的 ID 会互相碰撞。
// 新增服务时在此登记后再到服务初始化处引用，避免散落魔法数字。
const (
	ShardUser      int64 = 1 // 用户
	ShardGroup     int64 = 2 // 群组
	ShardChatMsg   int64 = 3 // 聊天消息
	ShardChatConv  int64 = 4 // 会话
	ShardBot       int64 = 5 // Bot 定义
	ShardKnowledge int64 = 6 // 知识库 / MCP Server
	ShardOpen      int64 = 7 // 开放平台（open_service）
	ShardBilling   int64 = 8 // 用量统计（billing_service）
	ShardAdmin     int64 = 9 // 管理后台（admin_service）
	ShardWork      int64 = 10 // 工作区 / 系统 Agent（日周总结报告）
)
