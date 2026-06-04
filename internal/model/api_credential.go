package model

import (
	"time"

	"gorm.io/gorm"
)

// APICredentialStatus 表示 API 凭据状态。
type APICredentialStatus string

const (
	APICredentialStatusEnabled  APICredentialStatus = "enabled"
	APICredentialStatusDisabled APICredentialStatus = "disabled"
)

// RiskPolicy 表示高风险操作策略。
type RiskPolicy string

const (
	RiskPolicyDisabled RiskPolicy = "disabled" // 禁止高风险操作
	RiskPolicyEnabled  RiskPolicy = "enabled"  // 允许高风险操作
	RiskPolicyConfirm  RiskPolicy = "confirm"  // 需要确认
)

// APICredential 表示 MTProto API 凭据配置。
type APICredential struct {
	ID                 uint                `gorm:"primaryKey" json:"id"`
	DisplayName        string              `gorm:"index;size:128;not null" json:"display_name"`
	APIID              int32               `gorm:"index;not null" json:"api_id"`
	EncryptedAPIHash   string              `gorm:"size:512;not null" json:"-"`          // 加密后的 api_hash，禁止暴露
	APIHashHint        string              `gorm:"size:16" json:"api_hash_hint"`        // 脱敏展示：前4位...后4位
	APIHashFingerprint string              `gorm:"size:64" json:"api_hash_fingerprint"` // 不可逆指纹
	Status             APICredentialStatus `gorm:"index;size:16;not null;default:enabled" json:"status"`
	RiskPolicy         RiskPolicy          `gorm:"size:16;not null;default:disabled" json:"risk_policy"`
	LastUsedAt         *time.Time          `json:"last_used_at"`
	CreatedAt          time.Time           `gorm:"not null" json:"created_at"`
	UpdatedAt          time.Time           `gorm:"not null" json:"updated_at"`
	DeletedAt          gorm.DeletedAt      `gorm:"index" json:"-"` // 软删除
}
