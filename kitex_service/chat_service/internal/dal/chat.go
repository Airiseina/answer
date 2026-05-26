package dal

import (
	"fmt"
	"github.com/Airiseina/answer/kitex_service/chat_service/internal/model"
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

// GetMessage 根据消息ID查询单条消息
// 用于撤回/编辑时校验消息是否存在、是否为发送者本人、是否已撤回等
func (dao *chatDao) GetMessage(msgID int64) (*model.Message, error) {
	var msg model.Message
	err := dao.db.Where("msg_id = ?", msgID).First(&msg).Error
	if err != nil {
		return nil, nil
	}
	return &msg, nil
}

// RecallMessage 将消息状态更新为已撤回
// 仅更新 status 字段，不修改 content（保留原始内容用于审计）
func (dao *chatDao) RecallMessage(msgID int64) error {
	err := dao.db.Model(&model.Message{}).Where("msg_id = ? AND status = ?", msgID, model.MsgStatusNormal).
		Update("status", model.MsgStatusRecalled).Error
	if err != nil {
		return fmt.Errorf("撤回消息失败: %w", err)
	}
	return nil
}

// EditMessage 更新消息内容并标记为已编辑
func (dao *chatDao) EditMessage(msgID int64, newContent string) error {
	err := dao.db.Model(&model.Message{}).Where("msg_id = ? AND status = ?", msgID, model.MsgStatusNormal).
		Updates(map[string]interface{}{
			"content":   newContent,
			"is_edited": true,
		}).Error
	if err != nil {
		return fmt.Errorf("编辑消息失败: %w", err)
	}
	return nil
}

func (dao *chatDao) SaveEditHistory(history *model.MessageEditHistory) error {
	err := dao.db.Create(history).Error
	if err != nil {
		return fmt.Errorf("保存编辑历史失败: %w", err)
	}
	return nil
}

func (dao *chatDao) GetEditHistory(msgID int64) ([]model.MessageEditHistory, error) {
	var histories []model.MessageEditHistory
	err := dao.db.Where("msg_id = ?", msgID).Order("version ASC").Find(&histories).Error
	if err != nil {
		return nil, fmt.Errorf("查询编辑历史失败: %w", err)
	}
	return histories, nil
}

func (dao *chatDao) GetLatestEditVersion(msgID int64) (int32, error) {
	var history model.MessageEditHistory
	err := dao.db.Where("msg_id = ?", msgID).Order("version DESC").First(&history).Error
	if err != nil {
		return 0, nil
	}
	return history.Version, nil
}

// GetMessagesAfterSeq 按会话ID拉取 seq > afterSeq 的消息，按 seq 升序排列
// 用于 Phase 6 上线同步：客户端上报本地最大 seq，服务端返回增量消息
func (dao *chatDao) GetMessagesAfterSeq(conversationID int64, afterSeq int64, limit int16) ([]model.Message, error) {
	var messages []model.Message
	err := dao.db.Where("conversation_id = ? AND seq > ?", conversationID, afterSeq).
		Order("seq ASC").Limit(int(limit)).Find(&messages).Error
	if err != nil {
		return nil, fmt.Errorf("查询seq之后的消息失败: %w", err)
	}
	return messages, nil
}
