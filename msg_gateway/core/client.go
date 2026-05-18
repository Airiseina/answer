package core

import (
	"encoding/json"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/gorilla/websocket"
)

// Client WebSocket 客户端连接，代表一个已建立连接的用户
// 每个用户同一时刻只会有一个活跃的 Client（重复连接时旧连接会被踢出）
type Client struct {
	Manager *Manager        // 所属的连接管理器，用于注册/注销和消息分发
	UserId  uint            // 用户ID，作为客户端的唯一标识
	Socket  *websocket.Conn // WebSocket 连接实例
}

// WsMessage WebSocket 消息协议，客户端与服务端之间的消息格式
// 通过 Type 字段区分消息类型：
//   - "chat": 聊天消息（客户端发送或服务端推送）
//   - "system": 系统消息（错误提示、操作结果等）
type WsMessage struct {
	Type           string `json:"type"`                      // 消息类型："chat"=聊天消息，"system"=系统消息
	ConversationID int64  `json:"conversation_id,omitempty"` // 所属会话ID，chat 消息必填
	PeerID         uint   `json:"peer_id,omitempty"`         // 对端用户ID，仅单聊首次发消息时使用
	From           uint   `json:"from,omitempty"`            // 消息发送者用户ID，服务端推送时填充
	Content        string `json:"content,omitempty"`         // 消息文本内容
	MsgID          int64  `json:"msg_id,omitempty"`          // 消息唯一ID，服务端分配
	Timestamp      int64  `json:"timestamp,omitempty"`       // 消息时间戳（毫秒），服务端分配
	Success        bool   `json:"success,omitempty"`         // 操作是否成功，system 消息使用
	Reason         string `json:"reason,omitempty"`          // 失败原因，system 消息使用
}

// ClientMessage 包装客户端消息，附带发送者信息
// 用于 Manager 将消息从读取协程传递到处理协程
type ClientMessage struct {
	Client  *Client    // 消息发送者
	Message *WsMessage // 消息内容
}

// ReadMessage 持续读取 WebSocket 消息
// 在独立的 goroutine 中运行，负责：
//  1. 读取 WebSocket 帧并反序列化为 WsMessage
//  2. 将消息转发到 Manager 的 Message 通道进行异步处理
//  3. 连接断开时自动触发注销流程
//
// 退出条件：WebSocket 读取错误（连接关闭、网络异常等）
// 退出后：将 Client 发送到 Manager.Unregister 通道，触发下线流程
func (client *Client) ReadMessage() {
	defer func() {
		// 触发下线注销：从管理器移除、关闭连接、通知 Redis
		client.Manager.Unregister <- client
		client.Socket.Close()
	}()
	for {
		_, message, err := client.Socket.ReadMessage()
		if err != nil {
			klog.Errorf("读取%d用户消息失败: %v", client.UserId, err)
			break
		}
		// 反序列化 JSON 消息
		var wsMsg WsMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			klog.Errorf("解析%d用户消息失败: %v", client.UserId, err)
			client.Send(&WsMessage{
				Type:    "system",
				Reason:  "消息格式错误",
				Success: false,
			})
			continue
		}
		// 将消息转发到 Manager 的处理通道
		client.Manager.Message <- &ClientMessage{
			Client:  client,
			Message: &wsMsg,
		}
	}
}

// Send 向客户端发送 WebSocket 消息
// 参数 msg: 待发送的消息对象，会被序列化为 JSON 后通过 WebSocket 发送
// 返回值: 序列化或写入失败时返回 error
func (client *Client) Send(msg *WsMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return client.Socket.WriteMessage(websocket.TextMessage, data)
}
