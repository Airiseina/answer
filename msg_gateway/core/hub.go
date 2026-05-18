package core

import (
	"answer_pkg/meter"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	chat "chat_service/kitex_gen/chat"
	"msg_gateway/rpc"

	"github.com/cloudwego/kitex/pkg/klog"
)

// Manager WebSocket 连接管理器，负责所有客户端连接的生命周期管理和消息路由
// 核心职责：
//  1. 维护在线用户 → Client 的映射（支持同一用户重复连接时踢出旧连接）
//  2. 处理用户上线/下线事件，同步在线状态到 Redis
//  3. 接收客户端消息，调用 chat_service RPC 并推送至会话成员
//  4. 支持跨网关推送（通过 HTTP 转发到其他 msg_gateway 实例）
type Manager struct {
	Clients     map[uint]*Client    // 在线用户映射表：UserID → Client
	Register    chan *Client        // 上线注册通道，新连接建立时写入
	Unregister  chan *Client        // 下线注销通道，连接断开时写入
	Message     chan *ClientMessage // 消息处理通道，客户端消息经此通道进入处理流程
	GatewayAddr string              // 本网关的对外地址（host:port），用于跨网关推送时标识自己
	Lock        sync.RWMutex        // 保护 Clients 映射的读写锁
}

// GlobalManager 全局唯一的 Manager 实例，在 InitManager 中初始化
var GlobalManager Manager

// InitManager 初始化全局 Manager 实例
// 参数 gatewayAddr: 本网关的对外地址，其他网关通过此地址推送消息到本网关
func InitManager(gatewayAddr string) {
	GlobalManager = Manager{
		Clients:     make(map[uint]*Client),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		Message:     make(chan *ClientMessage, 256), // 缓冲256条消息，防止短时高峰导致阻塞
		GatewayAddr: gatewayAddr,
	}
}

// Start 启动 Manager 的事件循环
// 在主 goroutine 中运行，通过 select 监听三个通道：
//   - Register: 处理新连接上线（踢出旧连接、注册到 Redis）
//   - Unregister: 处理连接断开（从映射表移除、注销 Redis）
//   - Message: 处理客户端发送的聊天消息
func (manager *Manager) Start() {
	klog.Infof("WebSocket 管理器启动...")
	for {
		select {
		case client := <-manager.Register:
			manager.Lock.Lock()
			// 踢出同一用户的旧连接（单设备登录策略）
			if old, ok := manager.Clients[client.UserId]; ok {
				old.Socket.Close()
				klog.Infof("用户%d重复连接，关闭旧连接", client.UserId)
			}
			manager.Clients[client.UserId] = client
			manager.Lock.Unlock()
			meter.M.WsConnectTotal.Add(context.Background(), 1)

			// 异步注册在线状态到 Redis，超时3秒
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_, err := rpc.SetOnline(ctx, &chat.SetOnlineReq{
					UserId:      int64(client.UserId),
					GatewayAddr: manager.GatewayAddr,
				})
				if err != nil {
					klog.Errorf("用户%d上线注册到Redis失败: %v", client.UserId, err)
				}
			}()
			klog.Infof("用户%d上线, 当前在线: %d", client.UserId, len(manager.Clients))

		case client := <-manager.Unregister:
			manager.Lock.Lock()
			if _, ok := manager.Clients[client.UserId]; ok {
				delete(manager.Clients, client.UserId)
				meter.M.WsDisconnectTotal.Add(context.Background(), 1)
				manager.Lock.Unlock()

				// 异步注销 Redis 在线状态，超时3秒
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					_, err := rpc.SetOffline(ctx, &chat.SetOfflineReq{
						UserId: int64(client.UserId),
					})
					if err != nil {
						klog.Errorf("用户%d下线注销从Redis失败: %v", client.UserId, err)
					}
				}()
				klog.Infof("用户%d下线, 当前在线: %d", client.UserId, len(manager.Clients))
			} else {
				manager.Lock.Unlock()
			}

		case clientMsg := <-manager.Message:
			manager.handleMessage(clientMsg)
		}
	}
}

// handleMessage 处理客户端发送的聊天消息
// 流程：
//  1. 参数校验（消息类型、内容非空、conversation_id/peer_id 至少一个非零）
//  2. 调用 chat_service RPC 发送消息
//  3. 向发送者回执消息（确认发送成功）
//  4. 异步推送消息给会话中的其他在线成员
func (manager *Manager) handleMessage(clientMsg *ClientMessage) {
	wsMsg := clientMsg.Message
	sender := clientMsg.Client

	// 校验消息类型
	if wsMsg.Type != "chat" {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "未知消息类型",
			Success: false,
		})
		return
	}
	// 校验消息内容
	if wsMsg.Content == "" {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "消息内容不能为空",
			Success: false,
		})
		return
	}
	// 校验会话标识：conversation_id 和 peer_id 至少需要一个
	if wsMsg.ConversationID == 0 && wsMsg.PeerID == 0 {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "conversation_id和peer_id不能同时为空",
			Success: false,
		})
		return
	}

	// 调用 chat_service RPC 发送消息
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := rpc.SendMessage(ctx, &chat.SendMessageReq{
		SenderId:       int64(sender.UserId),
		ConversationId: wsMsg.ConversationID,
		PeerId:         int64(wsMsg.PeerID),
		Content:        wsMsg.Content,
		ClientSeq:      0,
	})
	if err != nil {
		klog.Errorf("RPC SendMessage失败: %v", err)
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "发送失败",
			Success: false,
		})
		return
	}
	if !resp.Success {
		sender.Send(&WsMessage{
			Type:    "system",
			Reason:  "发送失败",
			Success: false,
		})
		return
	}

	// 构造聊天消息，回执给发送者
	chatMsg := &WsMessage{
		Type:           "chat",
		ConversationID: resp.ConversationId,
		From:           sender.UserId,
		Content:        wsMsg.Content,
		MsgID:          resp.MsgId,
		Timestamp:      resp.Timestamp,
	}
	sender.Send(chatMsg)
	meter.M.MessageSentTotal.Add(context.Background(), 1)

	// 异步推送给会话中的其他在线成员
	go manager.pushToMembers(resp.MemberIds, int64(sender.UserId), chatMsg)
}

// pushToMembers 将消息推送给会话中除发送者外的所有在线成员
// 推送策略：
//  1. 过滤掉发送者自身
//  2. 批量查询成员在线状态
//  3. 本地在线（连接在本网关）→ 直接通过 WebSocket 推送
//  4. 远程在线（连接在其他网关）→ 通过 HTTP 转发到对应网关
//  5. 离线成员 → 不推送（后续可接入离线推送如 APNs/FCM）
func (manager *Manager) pushToMembers(memberIDs []int64, senderID int64, chatMsg *WsMessage) {
	if len(memberIDs) == 0 {
		return
	}

	// 过滤掉发送者，只推送给其他成员
	var otherMemberIDs []int64
	for _, uid := range memberIDs {
		if uid != senderID {
			otherMemberIDs = append(otherMemberIDs, uid)
		}
	}
	if len(otherMemberIDs) == 0 {
		return
	}

	// 批量查询在线状态
	pushCtx, pushCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pushCancel()

	statusResp, err := rpc.GetOnlineStatus(pushCtx, &chat.GetOnlineStatusReq{
		UserIds: otherMemberIDs,
	})
	if err != nil {
		klog.Errorf("查询会话成员在线状态失败: %v", err)
		return
	}

	// 逐个推送
	for _, status := range statusResp.Statuses {
		if !status.Online {
			continue // 离线成员跳过
		}
		manager.Lock.RLock()
		client, localOnline := manager.Clients[uint(status.UserId)]
		manager.Lock.RUnlock()

		if localOnline {
			// 本地在线：直接 WebSocket 推送
			if err := client.Send(chatMsg); err != nil {
				klog.Errorf("推送消息给本地用户%d失败: %v", status.UserId, err)
			}
		} else if status.GatewayAddr != manager.GatewayAddr {
			// 远程在线：转发到对应网关
			pushToGateway(status.GatewayAddr, chatMsg)
		}
	}
}

// pushToGateway 将消息转发到其他 msg_gateway 实例
// 通过 HTTP POST 调用目标网关的 /push 接口
// 参数 gatewayAddr: 目标网关地址（host:port）
// 参数 msg: 待转发的消息
func pushToGateway(gatewayAddr string, msg *WsMessage) {
	data, err := json.Marshal(msg)
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

// HandlePush 处理来自其他网关的消息推送请求
// 当用户连接在其他网关时，该网关通过 HTTP POST /push 转发消息到本网关
// 本方法将消息广播给本网关上所有在线的客户端
// 注意：当前实现是广播给本网关所有客户端，后续可优化为按用户ID定向推送
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
	GlobalManager.Lock.RLock()
	for _, client := range GlobalManager.Clients {
		if err := client.Send(&msg); err != nil {
			klog.Errorf("推送消息给用户%d失败: %v", client.UserId, err)
		}
	}
	GlobalManager.Lock.RUnlock()
	w.WriteHeader(http.StatusOK)
}
