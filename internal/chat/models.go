// Package chat 提供聊天服务抽象。
package chat

import "time"

// PeerType 表示会话对象类型。
type PeerType string

const (
	PeerTypeUser    PeerType = "user"
	PeerTypeChat    PeerType = "chat"
	PeerTypeChannel PeerType = "channel"
)

// Dialog 表示一个会话。
type Dialog struct {
	PeerRef            string    `json:"peer_ref"`
	PeerType           PeerType  `json:"peer_type"`
	Title              string    `json:"title"`
	Username           string    `json:"username,omitempty"`
	AvatarPlaceholder  string    `json:"avatar_placeholder,omitempty"`
	LastMessagePreview string    `json:"last_message_preview,omitempty"`
	LastMessageAt      time.Time `json:"last_message_at,omitempty"`
	UnreadCount        int       `json:"unread_count"`
	IsPinned           bool      `json:"is_pinned,omitempty"`
	IsMuted            bool      `json:"is_muted,omitempty"`
}

// MessageDirection 表示消息方向。
type MessageDirection string

const (
	MessageDirectionIn  MessageDirection = "in"
	MessageDirectionOut MessageDirection = "out"
)

// MessageStatus 表示消息状态。
type MessageStatus string

const (
	MessageStatusSent    MessageStatus = "sent"
	MessageStatusFailed  MessageStatus = "failed"
	MessageStatusUnknown MessageStatus = "unknown"
)

// Message 表示一条消息。
type Message struct {
	MessageID   int              `json:"id"`
	PeerRef     string           `json:"peer_ref"`
	Direction   MessageDirection `json:"direction"`
	SenderName  string           `json:"sender_name,omitempty"`
	Text        string           `json:"text"`
	SentAt      time.Time        `json:"sent_at"`
	IsOutgoing  bool             `json:"is_outgoing"`
	Status      MessageStatus    `json:"status"`
	MessageType string           `json:"message_type"` // text, photo, sticker, etc.
}

// SendResult 表示发送消息的结果。
type SendResult struct {
	MessageID int       `json:"id"`
	SentAt    time.Time `json:"sent_at"`
	Status    string    `json:"status"`
	Direction string    `json:"direction"`
	Text      string    `json:"text"`
}

// Service 定义聊天服务接口。
type Service interface {
	// ListDialogs 获取最近会话列表。
	ListDialogs(accountID uint, limit int) ([]Dialog, error)

	// GetMessages 获取指定会话的最近消息。
	GetMessages(accountID uint, peerRef string, limit int) ([]Message, error)

	// SendText 向指定会话发送文本消息。
	SendText(accountID uint, peerRef string, text string) (*SendResult, error)
}
