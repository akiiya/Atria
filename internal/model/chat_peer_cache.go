package model

import "time"

// ChatPeerCache 缓存 Telegram peer 信息，用于聊天 API 调用。
// access_hash 加密保存，peer_ref 为不透明引用。
type ChatPeerCache struct {
	ID                  uint       `gorm:"primaryKey" json:"-"`
	AccountID           uint       `gorm:"index;not null" json:"-"`
	PeerRef             string     `gorm:"uniqueIndex;size:64;not null" json:"peer_ref"`
	PeerType            string     `gorm:"size:16;not null" json:"peer_type"` // user, chat, channel
	PeerID              int64      `gorm:"index;not null" json:"peer_id"`
	AccessHashEncrypted string     `gorm:"size:512" json:"-"` // AES-256-GCM 加密，chat 类型可为空
	Title               string     `gorm:"size:256" json:"title"`
	Username            string     `gorm:"size:128" json:"username"`
	LastMessagePreview  string     `gorm:"size:256" json:"last_message_preview"`
	LastMessageAt       *time.Time `json:"last_message_at"`
	UnreadCount         int        `gorm:"not null;default:0" json:"unread_count"`
	CreatedAt           time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"not null" json:"updated_at"`
}

// TableName 返回表名。
func (ChatPeerCache) TableName() string {
	return "chat_peer_cache"
}
