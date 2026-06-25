// Package telegramclient 定义 Telegram 客户端适配器边界。
// 上层业务（chat、server）只依赖此包的中立类型和接口，
// 不直接依赖 gotd/td 或未来 TDLib 的具体类型。
package telegramclient

import "time"

// PeerType 表示会话对象类型。
type PeerType string

const (
	PeerTypeUser       PeerType = "user"
	PeerTypeBot        PeerType = "bot"
	PeerTypeChat       PeerType = "chat"       // 基础群组
	PeerTypeSupergroup PeerType = "supergroup" // 超级群组（从 chat 迁移或独立创建）
	PeerTypeChannel    PeerType = "channel"
)

// MessageKind 表示消息类型。
type MessageKind string

const (
	MessageKindText        MessageKind = "text"
	MessageKindPhoto       MessageKind = "photo"
	MessageKindDocument    MessageKind = "document"
	MessageKindSticker     MessageKind = "sticker"
	MessageKindVideo       MessageKind = "video"
	MessageKindVoice       MessageKind = "voice"
	MessageKindAudio       MessageKind = "audio"
	MessageKindService     MessageKind = "service"
	MessageKindUnsupported MessageKind = "unsupported"
)

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

// DataSource 表示数据来源。
type DataSource string

const (
	DataSourceCache    DataSource = "cache"
	DataSourceTelegram DataSource = "telegram"
	DataSourceMixed    DataSource = "mixed"
)

// Dialog 表示一个会话（中立 DTO）。
type Dialog struct {
	PeerRef            string      `json:"peer_ref"`
	PeerType           PeerType    `json:"peer_type"`
	Title              string      `json:"title"`
	Username           string      `json:"username,omitempty"`
	AvatarText         string      `json:"avatar_text,omitempty"`
	LastMessagePreview string      `json:"last_message_preview,omitempty"`
	LastMessageKind    MessageKind `json:"last_message_kind,omitempty"`
	LastMessageAt      time.Time   `json:"last_message_at,omitempty"`
	UnreadCount        int         `json:"unread_count"`
	IsPinned           bool        `json:"is_pinned,omitempty"`
	IsMuted            bool        `json:"is_muted,omitempty"`
	MemberCount        int         `json:"member_count,omitempty"`
	Flags              string      `json:"flags,omitempty"` // 逗号分隔：verified,scam,fake,restricted,broadcast,megagroup
	AccessHash         int64       `json:"-"`               // 不返回前端，仅内部使用
	PeerID             int64       `json:"-"`               // 不返回前端，仅内部使用
}

// Message 表示一条消息（中立 DTO）。
type Message struct {
	ID                string           `json:"id"`
	TelegramMessageID int              `json:"telegram_message_id"`
	PeerRef           string           `json:"peer_ref"`
	Direction         MessageDirection `json:"direction"`
	SenderName        string           `json:"sender_name,omitempty"`
	Text              string           `json:"text"`
	Kind              MessageKind      `json:"kind"`
	Caption           string           `json:"caption,omitempty"`
	SentAt            time.Time        `json:"sent_at"`
	EditedAt          *time.Time       `json:"edited_at,omitempty"`
	IsOutgoing        bool             `json:"is_outgoing"`
	Status            MessageStatus    `json:"status"`
	Media             *Media           `json:"media,omitempty"`
}

// Media 表示消息媒体信息。
type Media struct {
	FileName string `json:"file_name,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Emoji    string `json:"emoji,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Duration int    `json:"duration,omitempty"`
}

// ListDialogsRequest 是获取会话列表的请求。
type ListDialogsRequest struct {
	AccountID       uint
	Limit           int
	APIID           int
	APIHash         string
	SessionFilePath string
}

// GetRecentMessagesRequest 是获取最近消息的请求。
type GetRecentMessagesRequest struct {
	AccountID       uint
	PeerRef         string
	Limit           int
	APIID           int
	APIHash         string
	SessionFilePath string
	PeerID          int64
	PeerType        PeerType
	AccessHash      int64
}

// LoadOlderMessagesRequest 是加载更早消息的请求。
type LoadOlderMessagesRequest struct {
	AccountID       uint
	PeerRef         string
	BeforeMessageID int64
	Limit           int
	APIID           int
	APIHash         string
	SessionFilePath string
	PeerID          int64
	PeerType        PeerType
	AccessHash      int64
}

// SendTextRequest 是发送文本消息的请求。
type SendTextRequest struct {
	AccountID       uint
	PeerRef         string
	Text            string
	ClientRandomID  int64
	APIID           int
	APIHash         string
	SessionFilePath string
	PeerID          int64
	PeerType        PeerType
	AccessHash      int64
}

// DialogsPage 是会话列表结果。
type DialogsPage struct {
	Source  DataSource `json:"source"`
	Stale   bool       `json:"stale"`
	Dialogs []Dialog   `json:"dialogs"`
}

// MessagesPage 是消息历史结果。
type MessagesPage struct {
	Source          DataSource `json:"source"`
	Stale           bool       `json:"stale"`
	Messages        []Message  `json:"messages"`
	HasOlder        bool       `json:"has_older"`
	OldestMessageID int64      `json:"oldest_message_id,omitempty"`
	NewestMessageID int64      `json:"newest_message_id,omitempty"`
}

// SendResult 是发送消息的结果。
type SendResult struct {
	MessageID int       `json:"id"`
	SentAt    time.Time `json:"sent_at"`
	Status    string    `json:"status"`
	Direction string    `json:"direction"`
	Text      string    `json:"text"`
}

// PeerInfo 是从缓存或 Telegram 获取的 peer 信息。
// 用于 adapter 构造 InputPeer 时的中间数据。
type PeerInfo struct {
	PeerRef    string
	PeerType   PeerType
	PeerID     int64
	AccessHash int64
	Title      string
	Username   string
}

// Contact 表示一个联系人（中立 DTO）。
type Contact struct {
	PeerRef     string   `json:"peer_ref"`
	PeerType    PeerType `json:"peer_type"`
	DisplayName string   `json:"display_name"`
	Username    string   `json:"username,omitempty"`
	Phone       string   `json:"phone,omitempty"` // 脱敏后的手机号
	AvatarText  string   `json:"avatar_text,omitempty"`
	AccessHash  int64    `json:"-"` // 不返回前端，仅内部使用
	PeerID      int64    `json:"-"` // 不返回前端，仅内部使用
}

// GetContactsRequest 是获取联系人列表的请求。
type GetContactsRequest struct {
	AccountID       uint
	APIID           int
	APIHash         string
	SessionFilePath string
}

// ContactsResult 是联系人列表结果。
type ContactsResult struct {
	Source   DataSource `json:"source"`
	Stale    bool       `json:"stale"`
	Contacts []Contact  `json:"contacts"`
}

// DownloadMediaRequest 是下载媒体的请求。
type DownloadMediaRequest struct {
	AccountID       uint
	PeerRef         string
	MessageID       int
	APIID           int
	APIHash         string
	SessionFilePath string
	PeerID          int64
	PeerType        PeerType
	AccessHash      int64
	OutputDir       string // 媒体缓存目录，adapter 负责写入文件
}

// DownloadMediaResult 是下载结果。
type DownloadMediaResult struct {
	FilePath     string `json:"-"` // 本地缓存路径（不返回前端）
	FileName     string `json:"file_name"`
	MIMEType     string `json:"mime_type"`
	Size         int64  `json:"size"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}
