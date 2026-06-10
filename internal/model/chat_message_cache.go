package model

import "time"

// ChatMessageCache 缓存 Telegram 消息，用于 cache-first 聊天加载。
// 消息正文加密保存，按 account_id + peer_ref 隔离。
type ChatMessageCache struct {
	ID                uint      `gorm:"primaryKey" json:"-"`
	AccountID         uint      `gorm:"index:idx_msg_account_peer;not null" json:"-"`
	PeerRef           string    `gorm:"index:idx_msg_account_peer;size:64;not null" json:"peer_ref"`
	TelegramMessageID int       `gorm:"index:idx_msg_account_peer;not null" json:"telegram_message_id"`
	Direction         string    `gorm:"size:8;not null" json:"direction"` // in, out
	SenderName        string    `gorm:"size:256" json:"sender_name"`
	Kind              string    `gorm:"size:16;not null;default:text" json:"kind"` // text, photo, document, etc.
	TextEncrypted     string    `gorm:"size:8192" json:"-"`                        // AES-256-GCM 加密
	CaptionEncrypted  string    `gorm:"size:4096" json:"-"`                        // AES-256-GCM 加密
	MediaJSON         string    `gorm:"size:2048" json:"media_json"`
	SentAt            time.Time `gorm:"index:idx_msg_account_sent;not null" json:"sent_at"`
	CreatedAt         time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt         time.Time `gorm:"not null" json:"updated_at"`
}

// TableName 返回表名。
func (ChatMessageCache) TableName() string {
	return "chat_message_cache"
}
