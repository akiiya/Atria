package model

import "time"

// TelegramUpdateState 存储 Telegram updates 状态（pts/qts/date/seq）。
// 用于 gotd updates.Manager 的 StateStorage 实现，
// 支持离线恢复时的 getDifference。
// 按 account_id 隔离，不存储敏感字段。
type TelegramUpdateState struct {
	ID         uint       `gorm:"primaryKey" json:"-"`
	AccountID  uint       `gorm:"uniqueIndex;not null" json:"account_id"`
	Pts        int        `gorm:"not null;default:0" json:"pts"`
	Qts        int        `gorm:"not null;default:0" json:"qts"`
	Date       int        `gorm:"not null;default:0" json:"date"`
	Seq        int        `gorm:"not null;default:0" json:"seq"`
	LastSyncAt *time.Time `json:"last_sync_at,omitempty"`
	CreatedAt  time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"not null" json:"updated_at"`
}

// TableName 返回表名。
func (TelegramUpdateState) TableName() string {
	return "telegram_update_state"
}
