package handle

import (
	"api_gateway/middleware"
	"api_gateway/response"
	"api_gateway/rpc"
	"context"
	"strconv"

	chat "chat_service/kitex_gen/chat"
	user "user_service/kitex_gen/user"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

func GetHistory(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id

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

	beforeMsgIDStr := c.DefaultQuery("before_msg_id", "0")
	beforeMsgID, _ := strconv.ParseInt(beforeMsgIDStr, 10, 64)

	limitStr := c.DefaultQuery("limit", "20")
	limitInt, _ := strconv.ParseInt(limitStr, 10, 16)
	if limitInt <= 0 {
		limitInt = 20
	}
	if limitInt > 100 {
		limitInt = 100
	}

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

	senderIDSet := make(map[int64]struct{})
	for _, m := range resp.Messages {
		senderIDSet[m.SenderId] = struct{}{}
	}
	senderIDs := make([]int64, 0, len(senderIDSet))
	for id := range senderIDSet {
		senderIDs = append(senderIDs, id)
	}
	senderNameMap := make(map[int64]string)
	if len(senderIDs) > 0 {
		nameResp, nameErr := rpc.GetUserNames(ctx, &user.GetUserNamesReq{UserIds: senderIDs})
		if nameErr == nil && nameResp != nil {
			for _, u := range nameResp.Users {
				senderNameMap[u.Id] = u.Name
			}
		}
	}

	type messageItem struct {
		MsgID          int64  `json:"msg_id,string"`
		ClientSeq      int64  `json:"client_seq"`
		SenderID       int64  `json:"sender_id,string"`
		SenderName     string `json:"sender_name"`
		ConversationID int64  `json:"conversation_id,string"`
		Content        string `json:"content"`
		Timestamp      int64  `json:"timestamp"`
		Status         int16  `json:"status"`
		IsEdited       bool   `json:"is_edited"`
	}

	var messages []messageItem
	for _, m := range resp.Messages {
		messages = append(messages, messageItem{
			MsgID:          m.MsgId,
			ClientSeq:      m.ClientSeq,
			SenderID:       m.SenderId,
			SenderName:     senderNameMap[m.SenderId],
			ConversationID: m.ConversationId,
			Content:        m.Content,
			Timestamp:      m.Timestamp,
			Status:         m.GetStatus(),
			IsEdited:       m.GetIsEdited(),
		})
	}
	if messages == nil {
		messages = []messageItem{}
	}
	response.Success(c, map[string]interface{}{
		"messages": messages,
	})
}

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
	type conversationItem struct {
		ConversationID int64    `json:"conversation_id,string"`
		Type           int16    `json:"type"`
		Name           string   `json:"name"`
		GroupID        int64    `json:"group_id,string"`
		MemberIds      []string `json:"member_ids"`
		MaxSeq         int64    `json:"max_seq"`
		MaxReadSeq     int64    `json:"max_read_seq"`
		UnreadCount    int64    `json:"unread_count"`
	}

	var privatePeerIDs []int64
	for _, c := range resp.Conversations {
		if c.Type == 1 && len(c.MemberIds) == 2 {
			for _, mid := range c.MemberIds {
				if mid != userID {
					privatePeerIDs = append(privatePeerIDs, mid)
				}
			}
		}
	}

	peerNameMap := make(map[int64]string)
	if len(privatePeerIDs) > 0 {
		uniqueIDs := make(map[int64]struct{})
		for _, id := range privatePeerIDs {
			uniqueIDs[id] = struct{}{}
		}
		idList := make([]int64, 0, len(uniqueIDs))
		for id := range uniqueIDs {
			idList = append(idList, id)
		}
		nameResp, nameErr := rpc.GetUserNames(ctx, &user.GetUserNamesReq{UserIds: idList})
		if nameErr == nil && nameResp != nil {
			for _, u := range nameResp.Users {
				peerNameMap[u.Id] = u.Name
			}
		}
	}

	var conversations []conversationItem
	for _, c := range resp.Conversations {
		memberStrs := make([]string, len(c.MemberIds))
		for i, id := range c.MemberIds {
			memberStrs[i] = strconv.FormatInt(id, 10)
		}
		name := c.Name
		if c.Type == 1 {
			for _, mid := range c.MemberIds {
				if mid != userID {
					if peerName, ok := peerNameMap[mid]; ok && peerName != "" {
						name = peerName
					}
					break
				}
			}
		}
		conversations = append(conversations, conversationItem{
			ConversationID: c.ConversationId,
			Type:           c.Type,
			Name:           name,
			GroupID: func() int64 {
				if c.GroupId != nil {
					return *c.GroupId
				}
				return 0
			}(),
			MemberIds:   memberStrs,
			MaxSeq:      c.GetMaxSeq(),
			MaxReadSeq:  c.GetMaxReadSeq(),
			UnreadCount: c.GetUnreadCount(),
		})
	}
	if conversations == nil {
		conversations = []conversationItem{}
	}
	response.Success(c, map[string]interface{}{
		"conversations": conversations,
	})
}

// MarkRead 标记会话已读
// 客户端打开会话时调用，将用户在该会话中的已读位置更新到当前最大消息序号
// 返回更新后的已读序号，客户端可用于更新本地未读数状态
func MarkRead(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id
	conversationIDStr := c.Param("conversation_id")
	if conversationIDStr == "" {
		response.Error(c, "参数缺失", "conversation_id不能为空")
		return
	}
	conversationID, err := strconv.ParseInt(conversationIDStr, 10, 64)
	if err != nil {
		response.Error(c, "参数错误", "conversation_id格式不正确")
		return
	}
	resp, err := rpc.MarkRead(ctx, &chat.MarkReadReq{
		UserId:         userID,
		ConversationId: conversationID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC MarkRead失败: %v", err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "标记失败", "标记已读失败")
		return
	}
	response.Success(c, map[string]interface{}{
		"max_read_seq": resp.GetMaxReadSeq(),
	})
}

// GetOnlineStatus 查询用户在线状态
// 客户端传入一组用户ID，返回每个用户的在线/离线状态
// 用于会话列表和聊天界面中展示在线状态指示器
func GetOnlineStatus(ctx context.Context, c *app.RequestContext) {
	var reqBody struct {
		UserIds []string `json:"user_ids"`
	}
	if err := c.BindJSON(&reqBody); err != nil {
		response.Error(c, "参数错误", "请求格式不正确")
		return
	}
	if len(reqBody.UserIds) == 0 {
		response.Success(c, map[string]interface{}{
			"statuses": []interface{}{},
		})
		return
	}
	if len(reqBody.UserIds) > 100 {
		response.Error(c, "参数错误", "最多查询100个用户")
		return
	}
	userIDs := make([]int64, 0, len(reqBody.UserIds))
	for _, idStr := range reqBody.UserIds {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}
		userIDs = append(userIDs, id)
	}
	if len(userIDs) == 0 {
		response.Success(c, map[string]interface{}{
			"statuses": []interface{}{},
		})
		return
	}
	resp, err := rpc.GetOnlineStatus(ctx, &chat.GetOnlineStatusReq{
		UserIds: userIDs,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC GetOnlineStatus失败: %v", err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	type onlineStatusItem struct {
		UserID string `json:"user_id"`
		Online bool   `json:"online"`
	}
	var statuses []onlineStatusItem
	for _, s := range resp.Statuses {
		statuses = append(statuses, onlineStatusItem{
			UserID: strconv.FormatInt(s.UserId, 10),
			Online: s.Online,
		})
	}
	if statuses == nil {
		statuses = []onlineStatusItem{}
	}
	response.Success(c, map[string]interface{}{
		"statuses": statuses,
	})
}

// RecallMessage 撤回消息
// 发送者在 2 分钟内可撤回自己发送的消息
// 路径参数：msg_id（消息ID）
// 成功后通过 WS 推送 recall 事件给会话中的所有成员
func RecallMessage(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id

	msgIDStr := c.Param("msg_id")
	if msgIDStr == "" {
		response.Error(c, "参数缺失", "msg_id不能为空")
		return
	}
	msgID, err := strconv.ParseInt(msgIDStr, 10, 64)
	if err != nil {
		response.Error(c, "参数错误", "msg_id格式不正确")
		return
	}

	var reqBody struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := c.BindJSON(&reqBody); err != nil {
		response.Error(c, "参数错误", "请求格式不正确")
		return
	}
	conversationID, err := strconv.ParseInt(reqBody.ConversationID, 10, 64)
	if err != nil || conversationID == 0 {
		response.Error(c, "参数错误", "conversation_id格式不正确")
		return
	}

	resp, err := rpc.RecallMessage(ctx, &chat.RecallMessageReq{
		UserId:         userID,
		MsgId:          msgID,
		ConversationId: conversationID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC RecallMessage失败: %v", err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "撤回失败", "无法撤回该消息，可能已超时或无权限")
		return
	}
	response.Success(c, map[string]interface{}{
		"recalled": true,
	})
}

// EditMessage 编辑消息
// 发送者可编辑自己发送的消息内容
// 路径参数：msg_id（消息ID）
// 成功后通过 WS 推送 edit 事件给会话中的所有成员
func EditMessage(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id

	msgIDStr := c.Param("msg_id")
	if msgIDStr == "" {
		response.Error(c, "参数缺失", "msg_id不能为空")
		return
	}
	msgID, err := strconv.ParseInt(msgIDStr, 10, 64)
	if err != nil {
		response.Error(c, "参数错误", "msg_id格式不正确")
		return
	}

	var reqBody struct {
		ConversationID string `json:"conversation_id"`
		NewContent     string `json:"new_content"`
	}
	if err := c.BindJSON(&reqBody); err != nil {
		response.Error(c, "参数错误", "请求格式不正确")
		return
	}
	conversationID, err := strconv.ParseInt(reqBody.ConversationID, 10, 64)
	if err != nil || conversationID == 0 {
		response.Error(c, "参数错误", "conversation_id格式不正确")
		return
	}
	if reqBody.NewContent == "" {
		response.Error(c, "参数错误", "消息内容不能为空")
		return
	}

	resp, err := rpc.EditMessage(ctx, &chat.EditMessageReq{
		UserId:         userID,
		MsgId:          msgID,
		ConversationId: conversationID,
		NewContent_:    reqBody.NewContent,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC EditMessage失败: %v", err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "编辑失败", "无法编辑该消息，可能已撤回或无权限")
		return
	}
	response.Success(c, map[string]interface{}{
		"edited": true,
	})
}

// GetEditHistory 查询消息编辑历史
func GetEditHistory(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id

	msgIDStr := c.Param("msg_id")
	if msgIDStr == "" {
		response.Error(c, "参数缺失", "msg_id不能为空")
		return
	}
	msgID, err := strconv.ParseInt(msgIDStr, 10, 64)
	if err != nil {
		response.Error(c, "参数错误", "msg_id格式不正确")
		return
	}

	var reqBody struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := c.BindJSON(&reqBody); err != nil {
		response.Error(c, "参数错误", "请求格式不正确")
		return
	}
	conversationID, err := strconv.ParseInt(reqBody.ConversationID, 10, 64)
	if err != nil || conversationID == 0 {
		response.Error(c, "参数错误", "conversation_id格式不正确")
		return
	}

	resp, err := rpc.GetEditHistory(ctx, &chat.GetEditHistoryReq{
		UserId:         userID,
		MsgId:          msgID,
		ConversationId: conversationID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC GetEditHistory失败: %v", err)
		response.Error(c, "系统繁忙", "请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "查询失败", "无法查看编辑历史")
		return
	}

	type editHistoryItem struct {
		ID         int64  `json:"id,string"`
		MsgID      int64  `json:"msg_id,string"`
		Version    int32  `json:"version"`
		OldContent string `json:"old_content"`
		EditorID   int64  `json:"editor_id,string"`
		EditedAt   int64  `json:"edited_at"`
	}
	var items []editHistoryItem
	for _, h := range resp.Histories {
		items = append(items, editHistoryItem{
			ID:         h.Id,
			MsgID:      h.MsgId,
			Version:    h.Version,
			OldContent: h.OldContent,
			EditorID:   h.EditorId,
			EditedAt:   h.EditedAt,
		})
	}
	response.Success(c, map[string]interface{}{
		"histories": items,
	})
}

// SyncMessages 同步消息
// 客户端断线重连后，遍历本地所有会话，携带每个会话的本地最大 seq
// 服务端对每个会话拉取 seq > last_seq 的消息返回
func SyncMessages(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id

	var reqBody struct {
		ConvSeqs []struct {
			ConversationID string `json:"conversation_id"`
			LastSeq        int64  `json:"last_seq"`
		} `json:"conv_seqs"`
		LimitPerConv int16 `json:"limit_per_conv"`
	}
	if err := c.BindJSON(&reqBody); err != nil {
		response.Error(c, "参数错误", "请求格式不正确")
		return
	}
	if len(reqBody.ConvSeqs) == 0 {
		response.Success(c, map[string]interface{}{
			"conv_messages": []interface{}{},
		})
		return
	}
	if len(reqBody.ConvSeqs) > 100 {
		response.Error(c, "参数错误", "最多同步100个会话")
		return
	}
	limit := reqBody.LimitPerConv
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var convSeqs []*chat.ConvSeqPair
	senderIDSet := make(map[int64]struct{})
	for _, pair := range reqBody.ConvSeqs {
		convID, err := strconv.ParseInt(pair.ConversationID, 10, 64)
		if err != nil || convID == 0 {
			continue
		}
		convSeqs = append(convSeqs, &chat.ConvSeqPair{
			ConversationId: convID,
			LastSeq:        pair.LastSeq,
		})
	}
	if len(convSeqs) == 0 {
		response.Success(c, map[string]interface{}{
			"conv_messages": []interface{}{},
		})
		return
	}

	resp, err := rpc.SyncMessages(ctx, &chat.SyncMessagesReq{
		UserId:   userID,
		ConvSeqs: convSeqs,
		Limit:    limit,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC SyncMessages失败: %v", err)
		response.Error(c, "系统繁忙", "同步消息失败")
		return
	}
	if !resp.Success {
		response.Error(c, "同步失败", "同步消息失败")
		return
	}

	for _, cm := range resp.ConvMessages {
		for _, m := range cm.Messages {
			senderIDSet[m.SenderId] = struct{}{}
		}
	}
	senderIDs := make([]int64, 0, len(senderIDSet))
	for id := range senderIDSet {
		senderIDs = append(senderIDs, id)
	}
	senderNameMap := make(map[int64]string)
	if len(senderIDs) > 0 {
		nameResp, nameErr := rpc.GetUserNames(ctx, &user.GetUserNamesReq{UserIds: senderIDs})
		if nameErr == nil && nameResp != nil {
			for _, u := range nameResp.Users {
				senderNameMap[u.Id] = u.Name
			}
		}
	}

	type messageItem struct {
		MsgID          int64  `json:"msg_id,string"`
		ClientSeq      int64  `json:"client_seq"`
		SenderID       int64  `json:"sender_id,string"`
		SenderName     string `json:"sender_name"`
		ConversationID int64  `json:"conversation_id,string"`
		Content        string `json:"content"`
		Timestamp      int64  `json:"timestamp"`
		Seq            int64  `json:"seq"`
		Status         int16  `json:"status"`
		IsEdited       bool   `json:"is_edited"`
	}
	type convMessagesItem struct {
		ConversationID int64         `json:"conversation_id,string"`
		Messages       []messageItem `json:"messages"`
	}

	var convMessages []convMessagesItem
	for _, cm := range resp.ConvMessages {
		var msgs []messageItem
		for _, m := range cm.Messages {
			msgs = append(msgs, messageItem{
				MsgID:          m.MsgId,
				ClientSeq:      m.ClientSeq,
				SenderID:       m.SenderId,
				SenderName:     senderNameMap[m.SenderId],
				ConversationID: m.ConversationId,
				Content:        m.Content,
				Timestamp:      m.Timestamp,
				Seq:            m.GetSeq(),
				Status:         m.GetStatus(),
				IsEdited:       m.GetIsEdited(),
			})
		}
		if msgs == nil {
			msgs = []messageItem{}
		}
		convMessages = append(convMessages, convMessagesItem{
			ConversationID: cm.ConversationId,
			Messages:       msgs,
		})
	}
	if convMessages == nil {
		convMessages = []convMessagesItem{}
	}
	response.Success(c, map[string]interface{}{
		"conv_messages": convMessages,
	})
}
