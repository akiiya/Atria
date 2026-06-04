// Package auth 提供 Web Session 和认证相关的基础能力。
package auth

import (
	"encoding/json"
	"fmt"
	"time"
)

// SessionClaims 是 Web Session 的数据结构。
type SessionClaims struct {
	AdminID             uint      `json:"admin_id"`
	Username            string    `json:"username"`
	CurrentCredentialID uint      `json:"current_credential_id,omitempty"` // 当前选中的 API 凭据 ID
	IssuedAt            time.Time `json:"issued_at"`
	ExpiresAt           time.Time `json:"expires_at"`
}

// IsExpired 检查 Session 是否已过期。
func (s *SessionClaims) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// EncodeSession 将 SessionClaims 序列化为 JSON 字节。
func EncodeSession(claims SessionClaims) ([]byte, error) {
	data, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("序列化 session 失败: %w", err)
	}
	return data, nil
}

// DecodeSession 将 JSON 字节反序列化为 SessionClaims。
func DecodeSession(data []byte) (SessionClaims, error) {
	var claims SessionClaims
	if err := json.Unmarshal(data, &claims); err != nil {
		return claims, fmt.Errorf("反序列化 session 失败: %w", err)
	}
	return claims, nil
}

// NewSessionClaims 创建新的 SessionClaims。
func NewSessionClaims(adminID uint, username string, ttl time.Duration) SessionClaims {
	now := time.Now()
	return SessionClaims{
		AdminID:   adminID,
		Username:  username,
		IssuedAt:  now,
		ExpiresAt: now.Add(ttl),
	}
}

// GetCurrentCredentialID 从 gin.Context 获取当前凭据 ID。
func GetCurrentCredentialID(c interface{ Get(string) (any, bool) }) uint {
	if id, exists := c.Get(ContextKeyCredentialID); exists {
		if credID, ok := id.(uint); ok {
			return credID
		}
	}
	return 0
}

// 上下文键名。
const (
	ContextKeyAdminID      = "admin_id"
	ContextKeyUsername     = "admin_username"
	ContextKeyCredentialID = "current_credential_id"
)
