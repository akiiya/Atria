package auth

import (
	"encoding/base64"
	"fmt"

	"github.com/user/atria/internal/crypto"
)

// AAD 用于 Web Session 加密。
var aadWebSession = []byte("atria:web_session:v1")

// EncodeSessionToken 将 SessionClaims 加密为 Cookie token。
//
// 流程：Claims → JSON → AES-GCM 加密 → base64 编码
//
// 安全要求：token 中不包含明文用户名等信息。
func EncodeSessionToken(key []byte, claims SessionClaims) (string, error) {
	// 序列化为 JSON
	data, err := EncodeSession(claims)
	if err != nil {
		return "", err
	}

	// AES-GCM 加密
	ciphertext, err := crypto.EncryptAESGCM(key, data, aadWebSession)
	if err != nil {
		return "", fmt.Errorf("加密 session 失败: %w", err)
	}

	// base64 编码
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecodeSessionToken 解密 Cookie token 为 SessionClaims。
//
// 流程：base64 解码 → AES-GCM 解密 → JSON 反序列化 → 检查过期
func DecodeSessionToken(key []byte, token string) (SessionClaims, error) {
	// base64 解码
	ciphertext, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return SessionClaims{}, fmt.Errorf("token base64 解码失败: %w", err)
	}

	// AES-GCM 解密
	data, err := crypto.DecryptAESGCM(key, ciphertext, aadWebSession)
	if err != nil {
		return SessionClaims{}, fmt.Errorf("token 解密失败: %w", err)
	}

	// 反序列化
	claims, err := DecodeSession(data)
	if err != nil {
		return SessionClaims{}, err
	}

	// 检查过期
	if claims.IsExpired() {
		return SessionClaims{}, fmt.Errorf("session 已过期")
	}

	return claims, nil
}
