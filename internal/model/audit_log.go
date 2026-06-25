package model

import "time"

// AuditLog represents an operation audit log entry.
type AuditLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ActorType    string    `gorm:"size:32;not null" json:"actor_type"` // admin, system
	ActorID      uint      `gorm:"not null" json:"actor_id"`
	AccountID    uint      `gorm:"index" json:"account_id"`
	Action       string    `gorm:"index;size:64;not null" json:"action"`
	ResourceType string    `gorm:"size:64;not null" json:"resource_type"`
	ResourceID   uint      `json:"resource_id"`
	RiskLevel    string    `gorm:"size:16;not null;default:low" json:"risk_level"` // low, medium, high, critical
	IP           string    `gorm:"size:45" json:"ip"`                              // Supports IPv6
	UserAgent    string    `gorm:"size:512" json:"user_agent"`
	Message      string    `gorm:"size:1024" json:"message"`
	MetadataJSON string    `gorm:"size:4096" json:"metadata_json"` // Must not contain sensitive raw data
	CreatedAt    time.Time `gorm:"index;not null" json:"created_at"`
}
