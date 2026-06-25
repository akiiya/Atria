package model

import "time"

// MediaCache 缓存已下载的媒体文件元数据。
type MediaCache struct {
	ID                uint      `gorm:"primaryKey" json:"-"`
	AccountID         uint      `gorm:"index:idx_media_account;not null" json:"-"`
	PeerRef           string    `gorm:"index:idx_media_account;size:64;not null" json:"-"`
	TelegramMessageID int       `gorm:"index:idx_media_account;not null" json:"-"`
	FileName          string    `gorm:"size:256" json:"file_name"`
	MIMEType          string    `gorm:"size:128" json:"mime_type"`
	FileSize          int64     `gorm:"not null;default:0" json:"file_size"`
	LocalPath         string    `gorm:"size:512;not null" json:"-"`                  // 不返回前端
	Status            string    `gorm:"size:16;not null;default:none" json:"status"` // none / cached / downloading / failed
	ErrorMessage      string    `gorm:"size:512" json:"-"`
	CreatedAt         time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt         time.Time `gorm:"not null" json:"updated_at"`
}

// TableName 返回表名。
func (MediaCache) TableName() string {
	return "media_cache"
}
