package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Airiseina/answer/pkg/observability/meter"
	"github.com/Airiseina/answer/pkg/storage"

	chat "github.com/Airiseina/answer/kitex_service/chat_service/kitex_gen/chat"
	"github.com/Airiseina/answer/msg_gateway/rpc"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/segmentio/kafka-go"
)

var pushSecret []byte
var kafkaWriter *kafka.Writer

func InitPushSecret(secret string) {
	pushSecret = []byte(secret)
}

func InitKafkaProducer(writer *kafka.Writer) {
	kafkaWriter = writer
}

type Manager struct {
	Clients     map[int64]*Client   // 在线用户映射表：UserID → Client
	Register    chan *Client        // 上线注册通道，新连接建立时写入
	Unregister  chan *Client        // 下线注销通道，连接断开时写入
	Message     chan *ClientMessage // 消息处理通道，客户端消息经此通道进入处理流程
	GatewayAddr string              // 本网关的对外地址（host:port），用于跨网关推送时标识自己
	Lock        sync.RWMutex        // 保护 Clients 映射的读写锁
}

var GlobalManager Manager

func InitManager(gatewayAddr string) {
	GlobalManager = Manager{
		Clients:     make(map[int64]*Client),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		Message:     make(chan *ClientMessage, 256),
		GatewayAddr: gatewayAddr,
	}
}

func (manager *Manager) Start() {
	klog.Infof("WebSocket 管理器启动...")
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		select {
		case client := <-manager.Register:
			manager.Lock.Lock()
			if old, ok := manager.Clients[client.UserId]; ok {
				old.Socket.Close()
				klog.Infof("用户%d重复连接，关闭旧连接", client.UserId)
			}
			manager.Clients[client.UserId] = client
			onlineCount := len(manager.Clients)
			manager.Lock.Unlock()
			meter.M.WsConnectTotal.Add(context.Background(), 1)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_, err := rpc.SetOnline(ctx, &chat.SetOnlineReq{
					UserId:      client.UserId,
					GatewayAddr: manager.GatewayAddr,
				})
				if err != nil {
					klog.Errorf("用户%d上线注册到Redis失败: %v", client.UserId, err)
				}
			}()
			klog.Infof("用户%d上线, 当前在线: %d", client.UserId, onlineCount)

		case client := <-manager.Unregister:
			manager.Lock.Lock()
			// 只有当 map 中的 client 与发起 Unregister 的 client 是同一个时，才删除
			// 防止旧连接关闭时误删新连接：
			//   用户重连 → Register 关闭旧 Socket → 旧 ReadMessage 退出 → Unregister 旧 client
			//   此时 map 中已经是新 client，不应删除
			if current, ok := manager.Clients[client.UserId]; ok && current == client {
				delete(manager.Clients, client.UserId)
				onlineCount := len(manager.Clients)
				meter.M.WsDisconnectTotal.Add(context.Background(), 1)
				manager.Lock.Unlock()

				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					_, err := rpc.SetOffline(ctx, &chat.SetOfflineReq{
						UserId: client.UserId,
					})
					if err != nil {
						klog.Errorf("用户%d下线注销从Redis失败: %v", client.UserId, err)
					}
				}()
				klog.Infof("用户%d下线, 当前在线: %d", client.UserId, onlineCount)
			} else {
				manager.Lock.Unlock()
			}

		case clientMsg := <-manager.Message:
			manager.handleMessage(clientMsg)
		case <-heartbeatTicker.C:
			manager.renewAllOnline()
		}
	}
}

func (manager *Manager) renewAllOnline() {
	manager.Lock.RLock()
	userIDs := make([]int64, 0, len(manager.Clients))
	for uid := range manager.Clients {
		userIDs = append(userIDs, uid)
	}
	manager.Lock.RUnlock()
	if len(userIDs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, uid := range userIDs {
		_, err := rpc.RenewOnline(ctx, &chat.RenewOnlineReq{UserId: uid})
		if err != nil {
			klog.Errorf("用户%d在线状态续期失败: %v", uid, err)
		}
	}
}

func (manager *Manager) handleMessage(clientMsg *ClientMessage) {
	wsMsg := clientMsg.Message
	sender := clientMsg.Client
	// 处理标记已读消息
	if wsMsg.Type == "mark_read" {
		manager.handleMarkRead(sender, wsMsg)
		return
	}
	// 处理输入状态消息：完全不落库，直接透传给会话中的其他在线成员
	if wsMsg.Type == "typing" {
		manager.handleTyping(sender, wsMsg)
		return
	}
	// 处理撤回消息请求：调用 RPC 撤回后，向会话成员推送 recall 事件
	if wsMsg.Type == "recall" {
		manager.handleRecall(sender, wsMsg)
		return
	}
	// 处理编辑消息请求：调用 RPC 编辑后，向会话成员推送 edit 事件
	if wsMsg.Type == "edit" {
		manager.handleEdit(sender, wsMsg)
		return
	}
	// 处理同步消息请求：客户端断线重连后，携带各会话本地最大 seq，拉取增量消息
	if wsMsg.Type == "sync" {
		manager.handleSync(sender, wsMsg)
		return
	}
	if wsMsg.Type != "chat" {
		sender.Send(&WsMessage{
			Type:      "system",
			Reason:    "未知消息类型",
			Success:   false,
			ClientSeq: wsMsg.ClientSeq,
		})
		return
	}
	if wsMsg.Content == "" {
		sender.Send(&WsMessage{
			Type:           "system",
			Reason:         "消息内容不能为空",
			Success:        false,
			ConversationID: wsMsg.ConversationID,
			ClientSeq:      wsMsg.ClientSeq,
		})
		return
	}
	if wsMsg.ConversationID == 0 && wsMsg.PeerAccount == "" {
		sender.Send(&WsMessage{
			Type:           "system",
			Reason:         "conversation_id和peer_account不能同时为空",
			Success:        false,
			ConversationID: wsMsg.ConversationID,
			ClientSeq:      wsMsg.ClientSeq,
		})
		return
	}
	var peerID int64
	if wsMsg.PeerAccount != "" {
		peerIdMap := rpc.GetUserIdMap(context.Background(), []string{wsMsg.PeerAccount})
		if id, ok := peerIdMap[wsMsg.PeerAccount]; ok && id != 0 {
			peerID = id
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := rpc.SendMessage(ctx, &chat.SendMessageReq{
		SenderId:       sender.UserId,
		ConversationId: wsMsg.ConversationID,
		PeerId:         peerID,
		Content:        wsMsg.Content,
		ClientSeq:      wsMsg.ClientSeq,
		QuoteMsgId:     wsMsg.QuoteMsgId,
	})
	if err != nil {
		klog.Errorf("RPC SendMessage失败: %v", err)
		sender.Send(&WsMessage{
			Type:           "system",
			Reason:         "发送失败",
			Success:        false,
			ConversationID: wsMsg.ConversationID,
			ClientSeq:      wsMsg.ClientSeq,
		})
		return
	}
	if !resp.Success {
		sender.Send(&WsMessage{
			Type:           "system",
			Reason:         "发送失败",
			Success:        false,
			ConversationID: wsMsg.ConversationID,
			ClientSeq:      wsMsg.ClientSeq,
		})
		return
	}
	pushContent := wsMsg.Content
	if resp.Content != nil && *resp.Content != "" {
		pushContent = *resp.Content
	}
	var convType int16
	if resp.ConversationType != nil {
		convType = *resp.ConversationType
	}
	chatMsg := &WsMessage{
		Type:             "chat",
		ConversationID:   resp.ConversationId,
		ConversationType: convType,
		FromAccount:      sender.UserAccount,
		FromName:         sender.UserName,
		Content:          storage.NormalizeContentURLs(pushContent),
		MsgID:            resp.MsgId,
		Seq:              resp.GetSeq(),
		Timestamp:        resp.Timestamp,
		QuoteMsgId:       wsMsg.QuoteMsgId,
	}
	sender.Send(chatMsg)
	meter.M.MessageSentTotal.Add(context.Background(), 1)
	go manager.pushToMembers(resp.MemberIds, sender.UserId, chatMsg)
	var mentionedUserIDs []int64
	if len(wsMsg.MentionedIds) > 0 {
		uidMap := rpc.GetUserIdMap(context.Background(), wsMsg.MentionedIds)
		for _, acc := range wsMsg.MentionedIds {
			if id, ok := uidMap[acc]; ok && id != 0 {
				mentionedUserIDs = append(mentionedUserIDs, id)
			}
		}
	}
	go manager.triggerBots(sender.UserId, resp.ConversationId, convType, resp.MemberIds, mentionedUserIDs, pushContent, func() int64 {
		if wsMsg.QuoteMsgId != nil {
			return *wsMsg.QuoteMsgId
		}
		return 0
	}())
}

func (manager *Manager) pushToMembers(memberIDs []int64, senderID int64, chatMsg *WsMessage) {
	if len(memberIDs) == 0 {
		return
	}
	var otherMemberIDs []int64
	for _, uid := range memberIDs {
		if uid != senderID {
			otherMemberIDs = append(otherMemberIDs, uid)
		}
	}
	if len(otherMemberIDs) == 0 {
		return
	}
	pushCtx, pushCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pushCancel()
	statusResp, err := rpc.GetOnlineStatus(pushCtx, &chat.GetOnlineStatusReq{
		UserIds: otherMemberIDs,
	})
	if err != nil {
		klog.Errorf("查询会话成员在线状态失败: %v", err)
		return
	}
	for _, status := range statusResp.Statuses {
		if !status.Online {
			continue
		}
		manager.Lock.RLock()
		client, localOnline := manager.Clients[status.UserId]
		manager.Lock.RUnlock()
		if localOnline {
			if err := client.Send(chatMsg); err != nil {
				klog.Errorf("推送消息给本地用户%d失败: %v", status.UserId, err)
			}
		} else if status.GatewayAddr != manager.GatewayAddr {
			pushToGateway(status.GatewayAddr, chatMsg, []int64{status.UserId})
		}
	}
}

func pushToGateway(gatewayAddr string, msg *WsMessage, targetUserIDs []int64) {
	pushMsg := *msg
	pushMsg.TargetUserIds = StringInt64Slice(targetUserIDs)
	data, err := json.Marshal(pushMsg)
	if err != nil {
		klog.Errorf("序列化推送消息失败: %v", err)
		return
	}
	url := fmt.Sprintf("http://%s/push", gatewayAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		klog.Errorf("创建推送请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	signPushRequest(req, data)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		klog.Errorf("推送消息到网关%s失败: %v", gatewayAddr, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		klog.Errorf("推送消息到网关%s返回非200: %d, body: %s", gatewayAddr, resp.StatusCode, string(body))
	}
}

func signPushRequest(req *http.Request, body []byte) {
	mac := hmac.New(sha256.New, pushSecret)
	mac.Write(body)
	req.Header.Set("X-Push-Signature", hex.EncodeToString(mac.Sum(nil)))
}

func verifyPushSignature(r *http.Request, body []byte) bool {
	sig := r.Header.Get("X-Push-Signature")
	if sig == "" {
		return false
	}
	mac := hmac.New(sha256.New, pushSecret)
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

func HandlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var msg WsMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	data, _ := json.Marshal(msg)
	if !verifyPushSignature(r, data) {
		klog.Warnf("推送请求签名验证失败, remote=%s", r.RemoteAddr)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if len(msg.TargetUserIds) == 0 {
		klog.Warnf("推送请求缺少 target_user_ids, remote=%s", r.RemoteAddr)
		http.Error(w, "Bad Request: missing target_user_ids", http.StatusBadRequest)
		return
	}
	GlobalManager.Lock.RLock()
	for _, uid := range msg.TargetUserIds {
		if client, ok := GlobalManager.Clients[uid]; ok {
			pushMsg := msg
			pushMsg.TargetUserIds = nil
			if err := client.Send(&pushMsg); err != nil {
				klog.Errorf("推送消息给用户%d失败: %v", uid, err)
			}
		}
	}
	GlobalManager.Lock.RUnlock()
	w.WriteHeader(http.StatusOK)
}

// handleMarkRead 处理客户端通过 WS 发送的标记已读请求
// 流程：
//  1. 调用 chat_service MarkRead RPC 更新 PG 和 Redis 中的已读序号
//  2. 向发送者返回 read_receipt 确认消息
//  3. 向该用户的所有其他在线设备推送 read_receipt，实现多端同步
func (manager *Manager) handleMarkRead(sender *Client, wsMsg *WsMessage) {
	if wsMsg.ConversationID == 0 {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "conversation_id不能为空",
			Success: false,
		})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := rpc.MarkRead(ctx, &chat.MarkReadReq{
		UserId:         sender.UserId,
		ConversationId: wsMsg.ConversationID,
	})
	if err != nil {
		klog.Errorf("RPC MarkRead失败: %v", err)
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "标记已读失败",
			Success: false,
		})
		return
	}
	if !resp.Success {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "标记已读失败",
			Success: false,
		})
		return
	}

	maxReadSeq := resp.GetMaxReadSeq()
	// 向发送者返回已读确认
	sender.Send(&WsMessage{
		Type:           "read_receipt",
		ConversationID: wsMsg.ConversationID,
		MaxReadSeq:     maxReadSeq,
		Success:        true,
	})

	// 向该用户的其他在线设备推送已读回执（多端同步）
	manager.pushReadReceiptToOtherDevices(sender.UserId, wsMsg.ConversationID, maxReadSeq)
}

// pushReadReceiptToOtherDevices 向用户的其他在线设备推送已读回执
// 用于多端同步：一端标记已读后，其他端也需要更新未读数
// 当前实现仅推送本地网关上的连接，跨网关推送将在 Phase 7 完善
func (manager *Manager) pushReadReceiptToOtherDevices(userID int64, conversationID int64, maxReadSeq int64) {
	manager.Lock.RLock()
	defer manager.Lock.RUnlock()
	// 当前 Clients 映射为 UserID → 单个 Client，多端场景需在 Phase 7 改造为多连接
	// 此处预留推送逻辑，Phase 7 将 Clients 改为 map[int64][]*Client 后即可生效
	_ = userID
	_ = conversationID
	_ = maxReadSeq
}

// handleTyping 处理输入状态消息
// 输入状态是高频轻状态，完全不落库，仅通过 WS 实时透传给会话中的其他在线成员
// 流程：
//  1. 校验 conversation_id 有效性
//  2. 查询该会话的所有成员
//  3. 查询成员在线状态
//  4. 向在线的其他成员推送 typing 事件（本地直推 + 跨网关 HTTP 推送）
func (manager *Manager) handleTyping(sender *Client, wsMsg *WsMessage) {
	if wsMsg.ConversationID == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 查询会话成员列表，确定需要推送的目标用户
	membersResp, err := rpc.GetConversationMembers(ctx, &chat.GetConversationMembersReq{
		ConversationId: wsMsg.ConversationID,
	})
	if err != nil {
		klog.Errorf("查询会话[%d]成员列表失败: %v", wsMsg.ConversationID, err)
		return
	}
	if len(membersResp.MemberIds) == 0 {
		return
	}

	// 构造 typing 推送消息，携带发送者信息和会话ID
	typingMsg := &WsMessage{
		Type:           "typing",
		ConversationID: wsMsg.ConversationID,
		FromAccount:    sender.UserAccount,
		FromName:       sender.UserName,
	}

	// 向会话中除发送者外的其他在线成员推送
	var otherMemberIDs []int64
	for _, uid := range membersResp.MemberIds {
		if uid != sender.UserId {
			otherMemberIDs = append(otherMemberIDs, uid)
		}
	}
	if len(otherMemberIDs) == 0 {
		return
	}

	// 查询在线状态
	statusResp, err := rpc.GetOnlineStatus(ctx, &chat.GetOnlineStatusReq{
		UserIds: otherMemberIDs,
	})
	if err != nil {
		klog.Errorf("查询会话成员在线状态失败: %v", err)
		return
	}

	for _, status := range statusResp.Statuses {
		if !status.Online {
			continue
		}
		manager.Lock.RLock()
		client, localOnline := manager.Clients[status.UserId]
		manager.Lock.RUnlock()
		if localOnline {
			// 本地用户：直接通过 WS 推送
			client.Send(typingMsg)
		} else if status.GatewayAddr != manager.GatewayAddr {
			// 跨网关用户：HTTP 推送到目标网关
			pushToGateway(status.GatewayAddr, typingMsg, []int64{status.UserId})
		}
	}
}

// handleRecall 处理撤回消息请求
// 流程：
//  1. 校验参数有效性（conversation_id、msg_id）
//  2. 调用 chat_service RecallMessage RPC 执行撤回（含权限和时限校验）
//  3. 构造 recall 推送消息，向会话中的所有在线成员推送
func (manager *Manager) handleRecall(sender *Client, wsMsg *WsMessage) {
	if wsMsg.ConversationID == 0 || wsMsg.MsgID == 0 {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "conversation_id和msg_id不能为空",
			Success: false,
		})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := rpc.RecallMessage(ctx, &chat.RecallMessageReq{
		UserId:         sender.UserId,
		MsgId:          wsMsg.MsgID,
		ConversationId: wsMsg.ConversationID,
	})
	if err != nil {
		klog.Errorf("RPC RecallMessage失败: %v", err)
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "撤回失败: " + err.Error(),
			Success: false,
		})
		return
	}
	if !resp.Success {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "撤回失败，可能已超时或无权限",
			Success: false,
		})
		return
	}

	// 向发送者返回撤回成功确认
	sender.Send(&WsMessage{
		Type:           "recall",
		ConversationID: wsMsg.ConversationID,
		MsgID:          wsMsg.MsgID,
		FromAccount:    sender.UserAccount,
		Success:        true,
	})

	recallMsg := &WsMessage{
		Type:           "recall",
		ConversationID: wsMsg.ConversationID,
		MsgID:          wsMsg.MsgID,
		FromAccount:    sender.UserAccount,
		FromName:       sender.UserName,
	}
	if resp.MemberIds != nil {
		manager.pushToMembers(resp.MemberIds, sender.UserId, recallMsg)
	}
}

// handleEdit 处理编辑消息请求
// 流程：
//  1. 校验参数有效性（conversation_id、msg_id、new_content）
//  2. 调用 chat_service EditMessage RPC 执行编辑（含权限校验）
//  3. 构造 edit 推送消息，向会话中的所有在线成员推送
func (manager *Manager) handleEdit(sender *Client, wsMsg *WsMessage) {
	if wsMsg.ConversationID == 0 || wsMsg.MsgID == 0 {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "conversation_id和msg_id不能为空",
			Success: false,
		})
		return
	}
	if wsMsg.NewContent == "" {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "新消息内容不能为空",
			Success: false,
		})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := rpc.EditMessage(ctx, &chat.EditMessageReq{
		UserId:         sender.UserId,
		MsgId:          wsMsg.MsgID,
		ConversationId: wsMsg.ConversationID,
		NewContent_:    wsMsg.NewContent,
	})
	if err != nil {
		klog.Errorf("RPC EditMessage失败: %v", err)
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "编辑失败",
			Success: false,
		})
		return
	}
	if !resp.Success {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "编辑失败，可能已撤回或无权限",
			Success: false,
		})
		return
	}

	// 向发送者返回编辑成功确认
	sender.Send(&WsMessage{
		Type:           "edit",
		ConversationID: wsMsg.ConversationID,
		MsgID:          wsMsg.MsgID,
		FromAccount:    sender.UserAccount,
		NewContent:     storage.NormalizeContentURLs(wsMsg.NewContent),
		IsEdited:       true,
		Success:        true,
	})

	editMsg := &WsMessage{
		Type:           "edit",
		ConversationID: wsMsg.ConversationID,
		MsgID:          wsMsg.MsgID,
		FromAccount:    sender.UserAccount,
		FromName:       sender.UserName,
		NewContent:     storage.NormalizeContentURLs(wsMsg.NewContent),
		IsEdited:       true,
	}
	if resp.MemberIds != nil {
		manager.pushToMembers(resp.MemberIds, sender.UserId, editMsg)
	}
}

// handleSync 处理客户端通过 WS 发送的消息同步请求
// 客户端断线重连后，遍历本地所有会话，携带每个会话的本地最大 seq
// 服务端对每个会话拉取 seq > last_seq 的消息返回
func (manager *Manager) handleSync(sender *Client, wsMsg *WsMessage) {
	if len(wsMsg.ConvSeqs) == 0 {
		sender.Send(&WsMessage{
			Type:         "sync",
			Success:      true,
			ConvMessages: []ConvMessagesItem{},
		})
		return
	}
	if len(wsMsg.ConvSeqs) > 100 {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "最多同步100个会话",
			Success: false,
		})
		return
	}
	limit := wsMsg.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var convSeqs []*chat.ConvSeqPair
	for _, pair := range wsMsg.ConvSeqs {
		if pair.ConversationID == 0 {
			continue
		}
		convSeqs = append(convSeqs, &chat.ConvSeqPair{
			ConversationId: pair.ConversationID,
			LastSeq:        pair.LastSeq,
		})
	}
	if len(convSeqs) == 0 {
		sender.Send(&WsMessage{
			Type:         "sync",
			Success:      true,
			ConvMessages: []ConvMessagesItem{},
		})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := rpc.SyncMessages(ctx, &chat.SyncMessagesReq{
		UserId:   sender.UserId,
		ConvSeqs: convSeqs,
		Limit:    limit,
	})
	if err != nil {
		klog.Errorf("RPC SyncMessages失败: %v", err)
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "同步消息失败",
			Success: false,
		})
		return
	}
	if !resp.Success {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "同步消息失败",
			Success: false,
		})
		return
	}

	var allSenderIDs []int64
	for _, cm := range resp.ConvMessages {
		for _, m := range cm.Messages {
			allSenderIDs = append(allSenderIDs, m.SenderId)
		}
	}
	accountMap := rpc.GetUserAccountMap(context.Background(), allSenderIDs)
	var convMessages []ConvMessagesItem
	for _, cm := range resp.ConvMessages {
		var msgs []SyncMsgItem
		for _, m := range cm.Messages {
			msgs = append(msgs, SyncMsgItem{
				MsgID:          m.MsgId,
				ClientSeq:      m.ClientSeq,
				SenderAccount:  accountMap[m.SenderId],
				ConversationID: m.ConversationId,
				Content:        storage.NormalizeContentURLs(m.Content),
				Timestamp:      m.Timestamp,
				Seq:            m.GetSeq(),
				Status:         m.GetStatus(),
				IsEdited:       m.GetIsEdited(),
			})
		}
		if msgs == nil {
			msgs = []SyncMsgItem{}
		}
		convMessages = append(convMessages, ConvMessagesItem{
			ConversationID: cm.ConversationId,
			Messages:       msgs,
		})
	}
	if convMessages == nil {
		convMessages = []ConvMessagesItem{}
	}
	sender.Send(&WsMessage{
		Type:         "sync",
		Success:      true,
		ConvMessages: convMessages,
	})
}

const (
	convTypePrivate int16 = 1
	convTypeGroup   int16 = 2
	botTaskTopic          = "bot-task-topic"
)

func (manager *Manager) triggerBots(senderID, conversationID int64, convType int16, memberIDs []int64, mentionedIDs []int64, content string, quoteMsgID int64) {
	if len(memberIDs) == 0 {
		return
	}
	isMember := false
	for _, m := range memberIDs {
		if m == senderID {
			isMember = true
			break
		}
	}
	if !isMember {
		klog.Errorf("发送者[%d]不在会话[%d]成员列表中，拒绝触发Bot", senderID, conversationID)
		return
	}
	mentionedSet := make(map[int64]struct{})
	for _, id := range mentionedIDs {
		mentionedSet[id] = struct{}{}
	}
	for _, memberID := range memberIDs {
		if memberID == senderID {
			continue
		}
		isBot, botId, err := rpc.IsBotCached(context.Background(), memberID)
		if err != nil {
			klog.Errorf("查询用户[%d]是否为Bot失败: %v", memberID, err)
			continue
		}
		if !isBot {
			continue
		}
		_, isMentioned := mentionedSet[memberID]
		if convType == convTypeGroup {
			if !isMentioned {
				continue
			}
		} else if convType == convTypePrivate {
			if len(memberIDs) > 2 && !isMentioned {
				continue
			}
		} else {
			continue
		}
		task := map[string]interface{}{
			"bot_id":          botId,
			"conversation_id": conversationID,
			"sender_id":       senderID,
			"content":         content,
			"quote_msg_id":    quoteMsgID,
		}
		taskJSON, _ := json.Marshal(task)
		err = kafkaWriter.WriteMessages(context.Background(), kafka.Message{
			Topic: botTaskTopic,
			Key:   []byte(fmt.Sprintf("%d", botId)),
			Value: taskJSON,
		})
		if err != nil {
			klog.Errorf("推送Bot任务到Kafka失败: %v", err)
		}
	}
}

type BotReplyMessage struct {
	MsgID            int64   `json:"msg_id"`
	Seq              int64   `json:"seq"`
	ConversationID   int64   `json:"conversation_id"`
	ConversationType int16   `json:"conversation_type"`
	SenderID         int64   `json:"sender_id"`
	Content          string  `json:"content"`
	Timestamp        int64   `json:"timestamp"`
	MemberIDs        []int64 `json:"member_ids"`
}

func StartBotReplyConsumer(reader *kafka.Reader) {
	go func() {
		for {
			msg, err := reader.FetchMessage(context.Background())
			if err != nil {
				klog.Errorf("Bot回复Kafka FetchMessage失败: %v", err)
				time.Sleep(time.Second)
				continue
			}
			var reply BotReplyMessage
			if err := json.Unmarshal(msg.Value, &reply); err != nil {
				klog.Errorf("解析Bot回复消息失败[partition=%d,offset=%d]: %v", msg.Partition, msg.Offset, err)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				reader.CommitMessages(ctx, msg)
				cancel()
				continue
			}
			senderAccount := ""
			senderName := ""
			if reply.SenderID > 0 {
				senderAccount = rpc.GetUserAccount(context.Background(), reply.SenderID)
				senderName = rpc.GetUserName(context.Background(), reply.SenderID)
			}
			chatMsg := &WsMessage{
				Type:             "chat",
				ConversationID:   reply.ConversationID,
				ConversationType: reply.ConversationType,
				FromAccount:      senderAccount,
				FromName:         senderName,
				Content:          storage.NormalizeContentURLs(reply.Content),
				MsgID:            reply.MsgID,
				Seq:              reply.Seq,
				Timestamp:        reply.Timestamp,
			}
			GlobalManager.pushToMembers(reply.MemberIDs, reply.SenderID, chatMsg)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			reader.CommitMessages(ctx, msg)
			cancel()
		}
	}()
	klog.Infof("Bot回复Kafka消费者已启动")
}
