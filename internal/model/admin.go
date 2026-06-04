package model

import "time"

// Admin represents the single administrator account.
type Admin struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Username      string     `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash  string     `gorm:"size:256;not null" json:"-"` // Never expose in JSON
	PasswordAlgo  string     `gorm:"size:32;not null" json:"password_algo"`
	IsInitialized bool       `gorm:"not null;default:false" json:"is_initialized"`
	LastLoginAt   *time.Time `json:"last_login_at"`
	CreatedAt     time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"not null" json:"updated_at"`
}
