package model

import "encoding/json"

const (
	MsgStatusNormal   int16 = 0
	MsgStatusRecalled int16 = 1
)

type Message struct {
	MsgID          int64  `gorm:"primaryKey;autoIncrement:false" json:"msg_id"`
	ClientSeq      int64  `gorm:"not null;default:0" json:"client_seq"`
	SenderID       int64  `gorm:"not null;index:idx_sender" json:"sender_id"`
	ConversationID int64  `gorm:"not null;default:0;index:idx_conversation_timestamp" json:"conversation_id"`
	Seq            int64  `gorm:"not null;default:0;index:idx_conversation_seq" json:"seq"`
	Content        string `gorm:"type:jsonb;not null" json:"content"`
	Status         int16  `gorm:"not null;default:0" json:"status"`
	IsEdited       bool   `gorm:"not null;default:false" json:"is_edited"`
	Timestamp      int64  `gorm:"not null;index:idx_conversation_timestamp" json:"timestamp"`
}

func (Message) TableName() string {
	return "message_table"
}

const (
	MsgTypeText  = "text"
	MsgTypeImage = "image"
	MsgTypeFile  = "file"
	MsgTypeVoice = "voice"
)

type BaseContent struct {
	Type string `json:"type"`
}

type MentionItem struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

type TextContent struct {
	Type     string        `json:"type"`
	Text     string        `json:"text"`
	Mentions []MentionItem `json:"mentions,omitempty"`
}

type ImageContent struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type FileContent struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	FileName string `json:"filename"`
	Size     int64  `json:"size"`
}

type VoiceContent struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Duration int    `json:"duration"`
	Size     int64  `json:"size"`
}

func NormalizeContent(content string) string {
	if content == "" {
		b, _ := json.Marshal(TextContent{Type: MsgTypeText, Text: ""})
		return string(b)
	}
	var base BaseContent
	if json.Unmarshal([]byte(content), &base) == nil && base.Type != "" {
		return content
	}
	b, _ := json.Marshal(TextContent{Type: MsgTypeText, Text: content})
	return string(b)
}

func ParseContentType(content string) string {
	var base BaseContent
	if json.Unmarshal([]byte(content), &base) == nil {
		return base.Type
	}
	return MsgTypeText
}

const (
	ConvTypePrivate int16 = 1
	ConvTypeGroup   int16 = 2
)

type Conversation struct {
	ID        int64  `gorm:"primaryKey;autoIncrement:false" json:"id"`
	Type      int16  `gorm:"not null" json:"type"`
	Name      string `gorm:"type:varchar(128);not null;default:''" json:"name"`
	GroupID   int64  `gorm:"default:0;index" json:"group_id"`
	CreatedAt int64  `gorm:"not null" json:"created_at"`
	UpdatedAt int64  `gorm:"not null" json:"updated_at"`
}

func (Conversation) TableName() string {
	return "conversation_table"
}

const (
	MemberRoleCreator int16 = 1
	MemberRoleNormal  int16 = 2
)

type ConversationMember struct {
	ConversationID int64 `gorm:"primaryKey" json:"conversation_id"`
	UserID         int64 `gorm:"primaryKey" json:"user_id"`
	Role           int16 `gorm:"not null;default:2" json:"role"`
	MaxReadSeq     int64 `gorm:"not null;default:0" json:"max_read_seq"`
	JoinedAt       int64 `gorm:"not null" json:"joined_at"`
}

func (ConversationMember) TableName() string {
	return "conversation_member"
}

type MessageEditHistory struct {
	ID         int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	MsgID      int64  `gorm:"not null;index:idx_msg_id_version" json:"msg_id"`
	Version    int32  `gorm:"not null;index:idx_msg_id_version" json:"version"`
	OldContent string `gorm:"type:jsonb;not null" json:"old_content"`
	EditorID   int64  `gorm:"not null" json:"editor_id"`
	EditedAt   int64  `gorm:"not null" json:"edited_at"`
}

func (MessageEditHistory) TableName() string {
	return "message_edit_history"
}
