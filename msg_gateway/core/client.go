package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/Airiseina/answer/pkg/observability/meter"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type StringInt64Slice []int64

func (s StringInt64Slice) MarshalJSON() ([]byte, error) {
	result := make([]string, len(s))
	for i, v := range s {
		result[i] = fmt.Sprintf(`"%d"`, v)
	}
	return []byte("[" + joinStrings(result, ",") + "]"), nil
}

func (s *StringInt64Slice) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = nil
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		var nums []int64
		if err2 := json.Unmarshal(data, &nums); err2 != nil {
			return err
		}
		*s = nums
		return nil
	}
	result := make([]int64, len(raw))
	for i, r := range raw {
		str := string(r)
		if str[0] == '"' {
			v, err := strconv.ParseInt(str[1:len(str)-1], 10, 64)
			if err != nil {
				return err
			}
			result[i] = v
		} else {
			v, err := strconv.ParseInt(str, 10, 64)
			if err != nil {
				return err
			}
			result[i] = v
		}
	}
	*s = result
	return nil
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for i := 1; i < len(ss); i++ {
		result += sep + ss[i]
	}
	return result
}

type Client struct {
	Manager     *Manager        // 所属的连接管理器，用于注册/注销和消息分发
	UserId      int64           // 用户ID，作为客户端的唯一标识
	UserName    string          // 用户名称，WebSocket 连接时从 user_service 获取
	UserAccount string          // 用户账号，WebSocket 连接时从 user_service 获取
	Socket      *websocket.Conn // WebSocket 连接实例
	writeMu     sync.Mutex      // 保护 WebSocket 并发写入
}

type WsMessage struct {
	Type             string             `json:"type"`
	ConversationID   int64              `json:"conversation_id,string,omitempty"`
	ConversationType int16              `json:"conversation_type,omitempty"`
	PeerAccount      string             `json:"peer_account,omitempty"`
	FromAccount      string             `json:"from_account,omitempty"`
	FromName         string             `json:"from_name,omitempty"`
	Content          string             `json:"content,omitempty"`
	MsgID            int64              `json:"msg_id,string,omitempty"`
	Seq              int64              `json:"seq,omitempty"`
	Timestamp        int64              `json:"timestamp,omitempty"`
	ClientSeq        int64              `json:"client_seq,omitempty"`
	Success          bool               `json:"success,omitempty"`
	Reason           string             `json:"reason,omitempty"`
	TargetUserIds    StringInt64Slice   `json:"target_user_ids,omitempty"`
	MaxReadSeq       int64              `json:"max_read_seq,omitempty"`
	NewContent       string             `json:"new_content,omitempty"`
	IsEdited         bool               `json:"is_edited,omitempty"`
	ConvSeqs         []ConvSeqItem      `json:"conv_seqs,omitempty"`
	Limit            int16              `json:"limit,omitempty"`
	ConvMessages     []ConvMessagesItem `json:"conv_messages,omitempty"`
	MentionedIds     []string           `json:"mentioned_ids,omitempty"`
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
	SenderAccount  string `json:"sender_account"`
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
			meter.M.MessageReceivedTotal.Add(context.Background(), 1, metric.WithAttributes(attribute.String("status", "read_error")))
			break
		}
		var wsMsg WsMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			klog.Errorf("解析%d用户消息失败: %v", client.UserId, err)
			meter.M.MessageReceivedTotal.Add(context.Background(), 1, metric.WithAttributes(attribute.String("status", "parse_error")))
			client.Send(&WsMessage{
				Type:    "system",
				Reason:  "消息格式错误",
				Success: false,
			})
			continue
		}
		meter.M.MessageReceivedTotal.Add(context.Background(), 1, metric.WithAttributes(attribute.String("status", "success")))
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
