package auth

import (
	"strings"
	"testing"
	"time"
)

func TestEncodeSessionToken_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	claims := NewSessionClaims(1, "admin", 24*time.Hour)

	token, err := EncodeSessionToken(key, claims)
	if err != nil {
		t.Fatalf("编码 token 失败: %s", err)
	}

	if token == "" {
		t.Error("token 不应为空")
	}

	// token 不应包含用户名明文
	if strings.Contains(token, "admin") {
		t.Error("token 不应包含用户名明文")
	}

	decoded, err := DecodeSessionToken(key, token)
	if err != nil {
		t.Fatalf("解码 token 失败: %s", err)
	}

	if decoded.AdminID != 1 {
		t.Errorf("AdminID 不匹配，期望=1，实际=%d", decoded.AdminID)
	}
	if decoded.Username != "admin" {
		t.Errorf("Username 不匹配，期望=admin，实际=%s", decoded.Username)
	}
}

func TestDecodeSessionToken_Expired(t *testing.T) {
	key := make([]byte, 32)

	// 创建已过期的 claims
	claims := SessionClaims{
		AdminID:   1,
		Username:  "admin",
		IssuedAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	token, err := EncodeSessionToken(key, claims)
	if err != nil {
		t.Fatalf("编码 token 失败: %s", err)
	}

	_, err = DecodeSessionToken(key, token)
	if err == nil {
		t.Error("过期 token 解码应该失败")
	}
}

func TestDecodeSessionToken_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(i + 1)
	}

	claims := NewSessionClaims(1, "admin", 24*time.Hour)
	token, err := EncodeSessionToken(key1, claims)
	if err != nil {
		t.Fatalf("编码 token 失败: %s", err)
	}

	_, err = DecodeSessionToken(key2, token)
	if err == nil {
		t.Error("错误密钥解码应该失败")
	}
}

func TestDecodeSessionToken_InvalidToken(t *testing.T) {
	key := make([]byte, 32)

	_, err := DecodeSessionToken(key, "invalid_base64_token!!!")
	if err == nil {
		t.Error("无效 token 解码应该失败")
	}
}

func TestSessionClaims_IsExpired(t *testing.T) {
	claims := SessionClaims{
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	if !claims.IsExpired() {
		t.Error("已过期的 claims 应该返回 true")
	}

	claims.ExpiresAt = time.Now().Add(1 * time.Hour)
	if claims.IsExpired() {
		t.Error("未过期的 claims 应该返回 false")
	}
}

func TestNewSessionClaims(t *testing.T) {
	before := time.Now()
	claims := NewSessionClaims(42, "testuser", 1*time.Hour)
	after := time.Now()

	if claims.AdminID != 42 {
		t.Errorf("AdminID 期望=42，实际=%d", claims.AdminID)
	}
	if claims.Username != "testuser" {
		t.Errorf("Username 期望=testuser，实际=%s", claims.Username)
	}
	if claims.IssuedAt.Before(before) || claims.IssuedAt.After(after) {
		t.Error("IssuedAt 应在当前时间附近")
	}
	if claims.ExpiresAt.Before(claims.IssuedAt) {
		t.Error("ExpiresAt 应晚于 IssuedAt")
	}
}
