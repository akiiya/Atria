package model

import "time"

// SystemSetting represents a system configuration setting.
type SystemSetting struct {
	Key         string    `gorm:"uniqueIndex;size:128;not null" json:"key"`
	Value       string    `gorm:"size:4096" json:"value"`                            // Encrypted if IsSensitive
	ValueType   string    `gorm:"size:32;not null;default:string" json:"value_type"` // string, int, bool, json
	IsSensitive bool      `gorm:"not null;default:false" json:"is_sensitive"`
	CreatedAt   time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null" json:"updated_at"`
}
