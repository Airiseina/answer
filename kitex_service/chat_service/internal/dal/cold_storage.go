package dal

import (
	"context"
	"fmt"

	"github.com/Airiseina/answer/kitex_service/chat_service/internal/model"
)

// ArchiveMessages 批量归档消息到 ClickHouse 冷库
// 使用 clickhouse-go 的 Batch 接口实现高效批量写入
// ClickHouse 批量写入性能远优于逐条 INSERT，适合归档场景
func (dao *coldStorageDao) ArchiveMessages(ctx context.Context, messages []model.ColdMessage) error {
	if len(messages) == 0 {
		return nil
	}
	batch, err := dao.chConn.PrepareBatch(ctx, "INSERT INTO answer_cold.cold_message")
	if err != nil {
		return fmt.Errorf("准备ClickHouse批量写入失败: %w", err)
	}
	for _, msg := range messages {
		if err := batch.Append(
			msg.MsgID,
			msg.ClientSeq,
			msg.SenderID,
			msg.ConversationID,
			msg.Seq,
			msg.Content,
			msg.Status,
			msg.IsEdited,
			msg.Timestamp,
			msg.QuoteMsgID,
			msg.ArchivedAt,
		); err != nil {
			return fmt.Errorf("写入ClickHouse行数据失败: %w", err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("提交ClickHouse批量写入失败: %w", err)
	}
	return nil
}

// GetColdHistory 从冷库拉取历史消息
// 按 conversation_id + timestamp 查询，利用 ClickHouse 的主键排序加速
func (dao *coldStorageDao) GetColdHistory(ctx context.Context, conversationID int64, beforeTimestamp int64, limit int16) ([]model.ColdMessage, error) {
	query := "SELECT msg_id, client_seq, sender_id, conversation_id, seq, content, status, is_edited, timestamp, quote_msg_id, archived_at FROM answer_cold.cold_message WHERE conversation_id = ?"
	args := []interface{}{conversationID}
	if beforeTimestamp > 0 {
		query += " AND timestamp < ?"
		args = append(args, beforeTimestamp)
	}
	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := dao.chConn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询ClickHouse冷库失败: %w", err)
	}
	defer rows.Close()

	var messages []model.ColdMessage
	for rows.Next() {
		var msg model.ColdMessage
		if err := rows.Scan(
			&msg.MsgID, &msg.ClientSeq, &msg.SenderID, &msg.ConversationID,
			&msg.Seq, &msg.Content, &msg.Status, &msg.IsEdited,
			&msg.Timestamp, &msg.QuoteMsgID, &msg.ArchivedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描ClickHouse行数据失败: %w", err)
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// SearchMessages 在冷库中搜索消息
// 使用 ClickHouse 的 like 实现文本搜索，适合海量数据场景
// 对于更复杂的全文搜索需求，可后续引入 ClickHouse 的 textSearch 索引
func (dao *coldStorageDao) SearchMessages(ctx context.Context, conversationID int64, keyword string, userID int64, limit int16) ([]model.ColdMessage, error) {
	query := "SELECT msg_id, client_seq, sender_id, conversation_id, seq, content, status, is_edited, timestamp, quote_msg_id, archived_at FROM answer_cold.cold_message WHERE content LIKE ?"
	args := []interface{}{"%" + keyword + "%"}

	if conversationID > 0 {
		query += " AND conversation_id = ?"
		args = append(args, conversationID)
	}

	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := dao.chConn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("搜索ClickHouse冷库失败: %w", err)
	}
	defer rows.Close()

	var messages []model.ColdMessage
	for rows.Next() {
		var msg model.ColdMessage
		if err := rows.Scan(
			&msg.MsgID, &msg.ClientSeq, &msg.SenderID, &msg.ConversationID,
			&msg.Seq, &msg.Content, &msg.Status, &msg.IsEdited,
			&msg.Timestamp, &msg.QuoteMsgID, &msg.ArchivedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描ClickHouse搜索结果失败: %w", err)
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// SearchMessagesByTimeRange 在冷库中按关键词和时间范围搜索消息
// 扩展 SearchMessages，增加时间范围过滤
// keyword 为空时仅按时间范围过滤
func (dao *coldStorageDao) SearchMessagesByTimeRange(ctx context.Context, conversationID int64, keyword string, userID int64, startTime int64, endTime int64, limit int16) ([]model.ColdMessage, error) {
	query := "SELECT msg_id, client_seq, sender_id, conversation_id, seq, content, status, is_edited, timestamp, quote_msg_id, archived_at FROM answer_cold.cold_message WHERE 1=1"
	args := []interface{}{}

	if keyword != "" {
		query += " AND content LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	if conversationID > 0 {
		query += " AND conversation_id = ?"
		args = append(args, conversationID)
	}
	if startTime > 0 {
		query += " AND timestamp >= ?"
		args = append(args, startTime)
	}
	if endTime > 0 {
		query += " AND timestamp <= ?"
		args = append(args, endTime)
	}

	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := dao.chConn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("搜索ClickHouse冷库失败: %w", err)
	}
	defer rows.Close()

	var messages []model.ColdMessage
	for rows.Next() {
		var msg model.ColdMessage
		if err := rows.Scan(
			&msg.MsgID, &msg.ClientSeq, &msg.SenderID, &msg.ConversationID,
			&msg.Seq, &msg.Content, &msg.Status, &msg.IsEdited,
			&msg.Timestamp, &msg.QuoteMsgID, &msg.ArchivedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描ClickHouse搜索结果失败: %w", err)
		}
		messages = append(messages, msg)
	}
	return messages, nil
}
