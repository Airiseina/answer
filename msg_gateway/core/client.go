package core

import (
	"encoding/json"
	"sync"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/gorilla/websocket"
)

type Client struct {
	Manager  *Manager        // 所属的连接管理器，用于注册/注销和消息分发
	UserId   int64           // 用户ID，作为客户端的唯一标识
	UserName string          // 用户名称，WebSocket 连接时从 user_service 获取
	Socket   *websocket.Conn // WebSocket 连接实例
	writeMu  sync.Mutex      // 保护 WebSocket 并发写入
}

type WsMessage struct {
	Type             string             `json:"type"`
	ConversationID   int64              `json:"conversation_id,string,omitempty"`
	ConversationType int16              `json:"conversation_type,omitempty"`
	PeerID           int64              `json:"peer_id,string,omitempty"`
	From             int64              `json:"from,string,omitempty"`
	FromName         string             `json:"from_name,omitempty"`
	Content          string             `json:"content,omitempty"`
	MsgID            int64              `json:"msg_id,string,omitempty"`
	Seq              int64              `json:"seq,omitempty"`
	Timestamp        int64              `json:"timestamp,omitempty"`
	ClientSeq        int64              `json:"client_seq,omitempty"`
	Success          bool               `json:"success,omitempty"`
	Reason           string             `json:"reason,omitempty"`
	TargetUserIds    []int64            `json:"target_user_ids,omitempty"`
	MaxReadSeq       int64              `json:"max_read_seq,omitempty"`
	NewContent       string             `json:"new_content,omitempty"`
	IsEdited         bool               `json:"is_edited,omitempty"`
	ConvSeqs         []ConvSeqItem      `json:"conv_seqs,omitempty"`
	Limit            int16              `json:"limit,omitempty"`
	ConvMessages     []ConvMessagesItem `json:"conv_messages,omitempty"`
}

type ConvSeqItem struct {
	ConversationID int64 `json:"conversation_id,string"`
	LastSeq        int64 `json:"last_seq"`
}

type ConvMessagesItem struct {
	ConversationID int64         `json:"conversation_id,string"`
	Messages       []SyncMsgItem `json:"messages"`
}

type SyncMsgItem struct {
	MsgID          int64  `json:"msg_id,string"`
	ClientSeq      int64  `json:"client_seq"`
	SenderID       int64  `json:"sender_id,string"`
	ConversationID int64  `json:"conversation_id,string"`
	Content        string `json:"content"`
	Timestamp      int64  `json:"timestamp"`
	Seq            int64  `json:"seq"`
	Status         int16  `json:"status"`
	IsEdited       bool   `json:"is_edited"`
}

type ClientMessage struct {
	Client  *Client    // 消息发送者
	Message *WsMessage // 消息内容
}

func (client *Client) ReadMessage() {
	defer func() {
		client.Manager.Unregister <- client
		client.Socket.Close()
	}()
	for {
		_, message, err := client.Socket.ReadMessage()
		if err != nil {
			klog.Errorf("读取%d用户消息失败: %v", client.UserId, err)
			break
		}
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
		client.Manager.Message <- &ClientMessage{
			Client:  client,
			Message: &wsMsg,
		}
	}
}

func (client *Client) Send(msg *WsMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	return client.Socket.WriteMessage(websocket.TextMessage, data)
}
