package model

import "time"

// TelegramChannelUpdateState 存储 Telegram 频道 update state（pts）。
// 用于 gotd updates.Manager 的 channel state 持久化。
// 按 account_id + channel_id 唯一隔离，不存储敏感字段。
type TelegramChannelUpdateState struct {
	ID         uint       `gorm:"primaryKey" json:"-"`
	AccountID  uint       `gorm:"uniqueIndex:idx_channel_state;not null" json:"account_id"`
	ChannelID  int64      `gorm:"uniqueIndex:idx_channel_state;not null" json:"channel_id"`
	Pts        int        `gorm:"not null;default:0" json:"pts"`
	LastSyncAt *time.Time `json:"last_sync_at,omitempty"`
	CreatedAt  time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"not null" json:"updated_at"`
}

// TableName 返回表名。
func (TelegramChannelUpdateState) TableName() string {
	return "telegram_channel_update_state"
}
