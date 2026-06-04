package dal

import (
	"context"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/chat_service/internal/model"
)

// BatchCreateInboxMessages 批量写入收件箱消息
// 写扩散的核心写入操作：为会话中的每个成员创建一条收件箱记录
// 使用 GORM 的 CreateInBatches 分批写入，避免单次 INSERT 过大
func (dao *inboxDao) BatchCreateInboxMessages(ctx context.Context, messages []model.InboxMessage) error {
	if len(messages) == 0 {
		return nil
	}
	err := dao.db.WithContext(ctx).CreateInBatches(messages, 100).Error
	if err != nil {
		return fmt.Errorf("批量写入收件箱消息失败: %w", err)
	}
	return nil
}

// GetInboxMessages 查询用户的收件箱消息（按会话维度）
// 写扩散的核心读取操作：直接查个人收件箱，无需关联会话表
// 利用 idx_user_conv_seq 索引加速查询
func (dao *inboxDao) GetInboxMessages(ctx context.Context, userID int64, conversationID int64, beforeSeq int64, limit int16) ([]model.InboxMessage, error) {
	var messages []model.InboxMessage
	query := dao.db.WithContext(ctx).Where("user_id = ? AND conversation_id = ?", userID, conversationID)
	if beforeSeq > 0 {
		query = query.Where("seq < ?", beforeSeq)
	}
	err := query.Order("seq DESC").Limit(int(limit)).Find(&messages).Error
	if err != nil {
		return nil, fmt.Errorf("查询收件箱消息失败: %w", err)
	}
	return messages, nil
}

// GetInboxMessagesAfterSeq 查询用户收件箱中 seq > afterSeq 的消息
// 用于上线同步：客户端上报本地最大 seq，服务端返回增量消息
func (dao *inboxDao) GetInboxMessagesAfterSeq(ctx context.Context, userID int64, conversationID int64, afterSeq int64, limit int16) ([]model.InboxMessage, error) {
	var messages []model.InboxMessage
	err := dao.db.WithContext(ctx).
		Where("user_id = ? AND conversation_id = ? AND seq > ?", userID, conversationID, afterSeq).
		Order("seq ASC").
		Limit(int(limit)).
		Find(&messages).Error
	if err != nil {
		return nil, fmt.Errorf("查询收件箱增量消息失败: %w", err)
	}
	return messages, nil
}

// RecallInboxMessage 更新收件箱中消息的撤回状态
// 撤回消息时需要更新所有成员收件箱中对应消息的状态
func (dao *inboxDao) RecallInboxMessage(ctx context.Context, msgID int64) error {
	err := dao.db.WithContext(ctx).
		Model(&model.InboxMessage{}).
		Where("msg_id = ?", msgID).
		Update("status", model.MsgStatusRecalled).Error
	if err != nil {
		return fmt.Errorf("更新收件箱消息撤回状态失败: %w", err)
	}
	return nil
}

// EditInboxMessage 更新收件箱中消息的内容和编辑状态
// 编辑消息时需要更新所有成员收件箱中对应消息的内容
func (dao *inboxDao) EditInboxMessage(ctx context.Context, msgID int64, newContent string) error {
	err := dao.db.WithContext(ctx).
		Model(&model.InboxMessage{}).
		Where("msg_id = ?", msgID).
		Updates(map[string]interface{}{
			"content":   newContent,
			"is_edited": true,
		}).Error
	if err != nil {
		return fmt.Errorf("更新收件箱消息内容失败: %w", err)
	}
	return nil
}

// GetInboxSeqByMsgID 根据消息ID查询收件箱中的 seq
// 用于 GetHistory 翻页游标转换：客户端传 beforeMsgID，需转为 beforeSeq 查收件箱
func (dao *inboxDao) GetInboxSeqByMsgID(ctx context.Context, userID int64, conversationID int64, msgID int64) (int64, error) {
	var seq int64
	err := dao.db.WithContext(ctx).
		Model(&model.InboxMessage{}).
		Where("user_id = ? AND conversation_id = ? AND msg_id = ?", userID, conversationID, msgID).
		Select("seq").
		Scan(&seq).Error
	if err != nil {
		return 0, fmt.Errorf("查询收件箱seq失败: %w", err)
	}
	return seq, nil
}
