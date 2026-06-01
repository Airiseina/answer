package handle

import (
	"context"
	"strconv"

	"github.com/Airiseina/answer/api_gateway/middleware"
	"github.com/Airiseina/answer/api_gateway/response"
	"github.com/Airiseina/answer/api_gateway/rpc"
	"github.com/Airiseina/answer/pkg/storage"

	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat"
	group "github.com/Airiseina/answer/kitex_service/group_service/kitex_gen/group"
	user "github.com/Airiseina/answer/kitex_service/user_service/kitex_gen/user"
	work "github.com/Airiseina/answer/kitex_service/work_service/kitex_gen/work"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

func buildAccountMap(ctx context.Context, userIDs []int64) map[int64]string {
	if len(userIDs) == 0 {
		return make(map[int64]string)
	}
	uniqueIDs := make(map[int64]struct{})
	for _, id := range userIDs {
		uniqueIDs[id] = struct{}{}
	}
	idList := make([]int64, 0, len(uniqueIDs))
	for id := range uniqueIDs {
		idList = append(idList, id)
	}
	nameResp, err := rpc.GetUserNames(ctx, &user.GetUserNamesReq{UserIds: idList})
	if err != nil || nameResp == nil {
		hlog.CtxWarnf(ctx, "批量获取用户account失败: %v", err)
		return make(map[int64]string)
	}
	m := make(map[int64]string, len(nameResp.Users))
	for _, u := range nameResp.Users {
		m[u.Id] = u.Account
	}
	return m
}

func buildUserIdMap(ctx context.Context, accounts []string) map[string]int64 {
	if len(accounts) == 0 {
		return make(map[string]int64)
	}
	uniqueAccounts := make(map[string]struct{})
	for _, a := range accounts {
		uniqueAccounts[a] = struct{}{}
	}
	accountList := make([]string, 0, len(uniqueAccounts))
	for a := range uniqueAccounts {
		accountList = append(accountList, a)
	}
	resp, err := rpc.GetUserIdsByAccounts(ctx, &user.GetUserIdsByAccountsReq{Accounts: accountList})
	if err != nil || resp == nil {
		hlog.CtxWarnf(ctx, "批量获取用户ID失败: %v", err)
		return make(map[string]int64)
	}
	m := make(map[string]int64, len(resp.Users))
	for _, u := range resp.Users {
		m[u.Account] = u.Id
	}
	return m
}

func buildGroupNumberMap(ctx context.Context, userID int64) map[int64]int64 {
	groupsResp, err := rpc.GetUserGroups(ctx, &group.GetUserGroupsReq{UserId: userID})
	if err != nil || groupsResp == nil {
		hlog.CtxWarnf(ctx, "获取用户群组列表失败: %v", err)
		return make(map[int64]int64)
	}
	m := make(map[int64]int64, len(groupsResp.Groups))
	for _, g := range groupsResp.Groups {
		m[g.GroupId] = g.GroupNumber
	}
	return m
}

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

	senderIDs := make([]int64, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		senderIDs = append(senderIDs, m.SenderId)
	}
	accountMap := buildAccountMap(ctx, senderIDs)
	nameMap := make(map[int64]string)
	if len(senderIDs) > 0 {
		nameResp, nameErr := rpc.GetUserNames(ctx, &user.GetUserNamesReq{UserIds: senderIDs})
		if nameErr == nil && nameResp != nil {
			for _, u := range nameResp.Users {
				nameMap[u.Id] = u.Name
			}
		}
	}

	type messageItem struct {
		MsgID          int64  `json:"msg_id,string"`
		ClientSeq      int64  `json:"client_seq"`
		SenderAccount  string `json:"sender_account"`
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
			SenderAccount:  accountMap[m.SenderId],
			SenderName:     nameMap[m.SenderId],
			ConversationID: m.ConversationId,
			Content:        storage.NormalizeContentURLs(m.Content),
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

	var allMemberIDs []int64
	for _, conv := range resp.Conversations {
		for _, mid := range conv.MemberIds {
			allMemberIDs = append(allMemberIDs, mid)
		}
	}
	accountMap := buildAccountMap(ctx, allMemberIDs)
	nameMap := make(map[int64]string)
	if len(allMemberIDs) > 0 {
		nameResp, nameErr := rpc.GetUserNames(ctx, &user.GetUserNamesReq{UserIds: allMemberIDs})
		if nameErr == nil && nameResp != nil {
			for _, u := range nameResp.Users {
				nameMap[u.Id] = u.Name
			}
		}
	}

	groupNumberMap := buildGroupNumberMap(ctx, userID)

	type conversationItem struct {
		ConversationID int64    `json:"conversation_id,string"`
		Type           int16    `json:"type"`
		Name           string   `json:"name"`
		GroupNumber    int64    `json:"group_number,string"`
		MemberAccounts []string `json:"member_accounts"`
		MaxSeq         int64    `json:"max_seq"`
		MaxReadSeq     int64    `json:"max_read_seq"`
		UnreadCount    int64    `json:"unread_count"`
	}

	var conversations []conversationItem
	for _, conv := range resp.Conversations {
		memberAccounts := make([]string, len(conv.MemberIds))
		for i, mid := range conv.MemberIds {
			memberAccounts[i] = accountMap[mid]
		}
		name := conv.Name
		if conv.Type == 1 {
			for _, mid := range conv.MemberIds {
				if mid != userID {
					if peerName, ok := nameMap[mid]; ok && peerName != "" {
						name = peerName
					}
					break
				}
			}
		}
		groupNumber := int64(0)
		if conv.GroupId != nil && *conv.GroupId != 0 {
			if gn, ok := groupNumberMap[*conv.GroupId]; ok {
				groupNumber = gn
			}
		}
		conversations = append(conversations, conversationItem{
			ConversationID: conv.ConversationId,
			Type:           conv.Type,
			Name:           name,
			GroupNumber:    groupNumber,
			MemberAccounts: memberAccounts,
			MaxSeq:         conv.GetMaxSeq(),
			MaxReadSeq:     conv.GetMaxReadSeq(),
			UnreadCount:    conv.GetUnreadCount(),
		})
	}
	if conversations == nil {
		conversations = []conversationItem{}
	}
	response.Success(c, map[string]interface{}{
		"conversations": conversations,
	})
}

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

func GetOnlineStatus(ctx context.Context, c *app.RequestContext) {
	var reqBody struct {
		Accounts []string `json:"accounts"`
	}
	if err := c.BindJSON(&reqBody); err != nil {
		response.Error(c, "参数错误", "请求格式不正确")
		return
	}
	if len(reqBody.Accounts) == 0 {
		response.Success(c, map[string]interface{}{
			"statuses": []interface{}{},
		})
		return
	}
	if len(reqBody.Accounts) > 100 {
		response.Error(c, "参数错误", "最多查询100个用户")
		return
	}

	userIdMap := buildUserIdMap(ctx, reqBody.Accounts)
	userIDs := make([]int64, 0, len(reqBody.Accounts))
	accountOrder := make([]string, 0, len(reqBody.Accounts))
	for _, acc := range reqBody.Accounts {
		if id, ok := userIdMap[acc]; ok && id != 0 {
			userIDs = append(userIDs, id)
			accountOrder = append(accountOrder, acc)
		}
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

	idToAccount := make(map[int64]string)
	for acc, id := range userIdMap {
		idToAccount[id] = acc
	}

	type onlineStatusItem struct {
		Account string `json:"account"`
		Online  bool   `json:"online"`
	}
	var statuses []onlineStatusItem
	for _, s := range resp.Statuses {
		acc := idToAccount[s.UserId]
		if acc == "" {
			continue
		}
		statuses = append(statuses, onlineStatusItem{
			Account: acc,
			Online:  s.Online,
		})
	}
	if statuses == nil {
		statuses = []onlineStatusItem{}
	}
	response.Success(c, map[string]interface{}{
		"statuses": statuses,
	})
}

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

	editorIDs := make([]int64, 0, len(resp.Histories))
	for _, h := range resp.Histories {
		editorIDs = append(editorIDs, h.EditorId)
	}
	editorAccountMap := buildAccountMap(ctx, editorIDs)

	type editHistoryItem struct {
		ID            int64  `json:"id,string"`
		MsgID         int64  `json:"msg_id,string"`
		Version       int32  `json:"version"`
		OldContent    string `json:"old_content"`
		EditorAccount string `json:"editor_account"`
		EditedAt      int64  `json:"edited_at"`
	}
	var items []editHistoryItem
	for _, h := range resp.Histories {
		items = append(items, editHistoryItem{
			ID:            h.Id,
			MsgID:         h.MsgId,
			Version:       h.Version,
			OldContent:    h.OldContent,
			EditorAccount: editorAccountMap[h.EditorId],
			EditedAt:      h.EditedAt,
		})
	}
	response.Success(c, map[string]interface{}{
		"histories": items,
	})
}

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

	var allSenderIDs []int64
	for _, cm := range resp.ConvMessages {
		for _, m := range cm.Messages {
			allSenderIDs = append(allSenderIDs, m.SenderId)
		}
	}
	accountMap := buildAccountMap(ctx, allSenderIDs)
	nameMap := make(map[int64]string)
	if len(allSenderIDs) > 0 {
		nameResp, nameErr := rpc.GetUserNames(ctx, &user.GetUserNamesReq{UserIds: allSenderIDs})
		if nameErr == nil && nameResp != nil {
			for _, u := range nameResp.Users {
				nameMap[u.Id] = u.Name
			}
		}
	}

	type messageItem struct {
		MsgID          int64  `json:"msg_id,string"`
		ClientSeq      int64  `json:"client_seq"`
		SenderAccount  string `json:"sender_account"`
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
				SenderAccount:  accountMap[m.SenderId],
				SenderName:     nameMap[m.SenderId],
				ConversationID: m.ConversationId,
				Content:        storage.NormalizeContentURLs(m.Content),
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

func GetConversationMembers(ctx context.Context, c *app.RequestContext) {
	var reqBody struct {
		Accounts []string `json:"accounts"`
	}
	if err := c.BindJSON(&reqBody); err != nil {
		response.Error(c, "参数错误", "请求格式不正确")
		return
	}
	if len(reqBody.Accounts) == 0 {
		response.Success(c, map[string]interface{}{
			"members": []interface{}{},
		})
		return
	}
	if len(reqBody.Accounts) > 500 {
		response.Error(c, "参数错误", "最多查询500个用户")
		return
	}
	resp, err := rpc.GetUsersInfoByAccounts(ctx, &user.GetUsersInfoByAccountsReq{Accounts: reqBody.Accounts})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC GetUsersInfoByAccounts失败: %v", err)
		response.Error(c, "系统繁忙", "获取成员信息失败")
		return
	}
	type memberItem struct {
		Account   string `json:"account"`
		Name      string `json:"name"`
		AvatarUrl string `json:"avatar_url"`
	}
	var members []memberItem
	for _, u := range resp.Users {
		members = append(members, memberItem{
			Account:   u.Account,
			Name:      u.Name,
			AvatarUrl: storage.NormalizeURL(u.AvatarUrl),
		})
	}
	if members == nil {
		members = []memberItem{}
	}
	response.Success(c, map[string]interface{}{
		"members": members,
	})
}

func SummarizeConversation(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id

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

	resp, err := rpc.SummarizeConversation(ctx, &work.SummarizeConversationReq{
		ConversationId: conversationID,
		UserId:         userID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC SummarizeConversation失败: %v", err)
		response.Error(c, "系统繁忙", "生成总结失败，请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "总结失败", "无法生成该会话的总结")
		return
	}
	response.Success(c, map[string]interface{}{
		"summary": resp.Summary,
	})
}

func SuggestReplies(ctx context.Context, c *app.RequestContext) {
	Identity, _ := c.Get(middleware.IdentityKey)
	userInfo := Identity.(*middleware.Resp)
	userID := userInfo.Id

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

	resp, err := rpc.SuggestReplies(ctx, &work.SuggestRepliesReq{
		ConversationId: conversationID,
		UserId:         userID,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC SuggestReplies失败: %v", err)
		response.Error(c, "系统繁忙", "生成回复候选失败，请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "生成失败", "无法生成回复候选")
		return
	}
	response.Success(c, map[string]interface{}{
		"replies": resp.Replies,
	})
}

func TranslateMessage(ctx context.Context, c *app.RequestContext) {
	var reqBody struct {
		Content    string `json:"content"`
		TargetLang string `json:"target_lang"`
	}
	if err := c.BindJSON(&reqBody); err != nil {
		response.Error(c, "参数错误", "请求格式不正确")
		return
	}
	if reqBody.Content == "" {
		response.Error(c, "参数错误", "消息内容不能为空")
		return
	}
	if reqBody.TargetLang == "" {
		response.Error(c, "参数错误", "目标语言不能为空")
		return
	}

	resp, err := rpc.TranslateMessage(ctx, &work.TranslateMessageReq{
		Content:    reqBody.Content,
		TargetLang: reqBody.TargetLang,
	})
	if err != nil {
		hlog.CtxErrorf(ctx, "RPC TranslateMessage失败: %v", err)
		response.Error(c, "系统繁忙", "翻译失败，请稍后重试")
		return
	}
	if !resp.Success {
		response.Error(c, "翻译失败", "无法翻译该消息")
		return
	}
	response.Success(c, map[string]interface{}{
		"translated_content": resp.TranslatedContent,
	})
}
