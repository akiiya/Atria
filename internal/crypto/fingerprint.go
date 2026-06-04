package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

// FingerprintLength 是指纹输出的十六进制字符长度。
const FingerprintLength = 16

// Fingerprint 计算字符串的短指纹（SHA-256 前 16 位 hex）。
//
// 用途：api_hash_fingerprint、phone_fingerprint、session_file_fingerprint
//
// 注意：指纹仅用于展示和去重辅助，不是加密，不可用于还原原文。
func Fingerprint(value string) string {
	if value == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])[:FingerprintLength]
}
