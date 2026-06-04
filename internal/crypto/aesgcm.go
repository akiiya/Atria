package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// EncryptAESGCM 使用 AES-256-GCM 加密数据。
//
// key: 32 字节密钥
// plaintext: 明文数据
// aad: 附加认证数据（可为 nil）
//
// 返回格式：nonce + ciphertext + tag
func EncryptAESGCM(key []byte, plaintext []byte, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 加密并拼接 nonce + ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, aad)
	return ciphertext, nil
}

// DecryptAESGCM 使用 AES-256-GCM 解密数据。
//
// key: 32 字节密钥
// ciphertext: 密文数据（nonce + ciphertext + tag）
// aad: 附加认证数据（必须与加密时一致）
func DecryptAESGCM(key []byte, ciphertext []byte, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("密文长度不足")
	}

	// 分离 nonce 和密文
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("解密失败")
	}

	return plaintext, nil
}

// EncryptString 加密字符串并返回 base64 编码的结果。
// 空字符串将直接返回空字符串。
func EncryptString(key []byte, plaintext string, aad []byte) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	ciphertext, err := EncryptAESGCM(key, []byte(plaintext), aad)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString 解密 base64 编码的密文字符串。
// 空字符串将直接返回空字符串。
func DecryptString(key []byte, ciphertextBase64 string, aad []byte) (string, error) {
	if ciphertextBase64 == "" {
		return "", nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", fmt.Errorf("base64 解码失败: %w", err)
	}

	plaintext, err := DecryptAESGCM(key, ciphertext, aad)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
