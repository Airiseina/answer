package handle

import (
	"api_gateway/middleware"
	"api_gateway/response"
	"api_gateway/rpc"
	"context"
	"strconv"

	chat "chat_service/kitex_gen/chat"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// GetHistory 拉取会话历史消息
// GET /api/v1/chat/history?conversation_id=xxx&before_msg_id=0&limit=20
// 参数:
//   - conversation_id: 会话ID（必填）
//   - before_msg_id: 游标，返回 msg_id < before_msg_id 的消息，默认 0（从最新开始）
//   - limit: 返回条数，范围 [1, 100]，默认 20
func GetHistory(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id

	// 解析必填参数 conversation_id
	conversationIDStr := c.Query("conversation_id")
	if conversationIDStr == "" {
		response.Error(c, "参数缺失", "conversation_id不能为空")
		return
	}
	conversationID, err := strconv.ParseInt(conversationIDStr, 10, 64)
	if err != nil {
		response.Error(c, "参数错误", "conversation_id格式不正确")
		return
	}

	// 解析可选参数 before_msg_id，默认 0
	beforeMsgIDStr := c.DefaultQuery("before_msg_id", "0")
	beforeMsgID, _ := strconv.ParseInt(beforeMsgIDStr, 10, 64)

	// 解析可选参数 limit，默认 20，上限 100
	limitStr := c.DefaultQuery("limit", "20")
	limitInt, _ := strconv.ParseInt(limitStr, 10, 16)
	if limitInt <= 0 {
		limitInt = 20
	}
	if limitInt > 100 {
		limitInt = 100
	}

	// 调用 chat_service RPC，传入 userID 进行成员身份校验
	resp, err := rpc.GetHistory(ctx, &chat.GetHistoryReq{
		UserId:         userID,
		ConversationId: conversationID,
		BeforeMsgId:    beforeMsgID,
		Limit:          int16(limitInt),
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC GetHistory失败: %v", err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "查询失败", "无权查看该会话消息或获取历史消息失败")
		return
	}

	// 转换为前端友好的 JSON 格式
	type messageItem struct {
		MsgID          int64  `json:"msg_id"`
		ClientSeq      int64  `json:"client_seq"`
		SenderID       int64  `json:"sender_id"`
		ConversationID int64  `json:"conversation_id"`
		Content        string `json:"content"`
		Timestamp      int64  `json:"timestamp"`
	}

	var messages []messageItem
	for _, m := range resp.Messages {
		messages = append(messages, messageItem{
			MsgID:          m.MsgId,
			ClientSeq:      m.ClientSeq,
			SenderID:       m.SenderId,
			ConversationID: m.ConversationId,
			Content:        m.Content,
			Timestamp:      m.Timestamp,
		})
	}
	// 保证空列表返回 [] 而非 null
	if messages == nil {
		messages = []messageItem{}
	}
	response.Success(c, map[string]interface{}{
		"messages": messages,
	})
}

// GetConversations 查询当前用户的会话列表
// GET /api/v1/chat/conversations
// 返回用户参与的所有会话，按最近活跃时间降序排列
func GetConversations(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id

	resp, err := rpc.GetConversations(ctx, &chat.GetConversationsReq{
		UserId: userID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC GetConversations失败: %v", err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "查询失败", "获取会话列表失败")
		return
	}

	// 转换为前端友好的 JSON 格式
	type conversationItem struct {
		ConversationID int64   `json:"conversation_id"`
		Type           int16   `json:"type"`
		Name           string  `json:"name"`
		GroupID        int64   `json:"group_id"` // 群聊关联的群组ID，前端通过此字段将会话与群组关联
		MemberIds      []int64 `json:"member_ids"`
	}

	var conversations []conversationItem
	for _, c := range resp.Conversations {
		conversations = append(conversations, conversationItem{
			ConversationID: c.ConversationId,
			Type:           c.Type,
			Name:           c.Name,
			GroupID: func() int64 {
				if c.GroupId != nil {
					return *c.GroupId
				}
				return 0
			}(),
			MemberIds: c.MemberIds,
		})
	}
	// 保证空列表返回 [] 而非 null
	if conversations == nil {
		conversations = []conversationItem{}
	}
	response.Success(c, map[string]interface{}{
		"conversations": conversations,
	})
}
