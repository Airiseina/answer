package dal

import (
	"chat_service/internal/model"
	"fmt"
)

// CreateMessage 将消息写入 PostgreSQL
// 参数 msg 需预先填充完整字段（MsgID 由 Snowflake 生成，Timestamp 为毫秒时间戳）
func (dao *chatDao) CreateMessage(msg *model.Message) error {
	err := dao.db.Create(msg).Error
	if err != nil {
		return fmt.Errorf("存储消息失败: %w", err)
	}
	return nil
}

// GetHistory 按会话ID拉取历史消息，支持基于 msg_id 的游标翻页
// 参数:
//   - conversationID: 会话ID，必填
//   - beforeMsgID: 游标，返回 msg_id < beforeMsgID 的消息；传 0 表示从最新消息开始
//   - limit: 返回条数上限
//
// 返回值: 消息列表（按 msg_id 降序排列，即最新消息在前）
func (dao *chatDao) GetHistory(conversationID int64, beforeMsgID int64, limit int16) ([]model.Message, error) {
	var messages []model.Message
	query := dao.db.Where("conversation_id = ?", conversationID)
	if beforeMsgID > 0 {
		query = query.Where("msg_id < ?", beforeMsgID)
	}
	err := query.Order("msg_id DESC").Limit(int(limit)).Find(&messages).Error
	if err != nil {
		return nil, fmt.Errorf("查询历史消息失败: %w", err)
	}
	return messages, nil
}
