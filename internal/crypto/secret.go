// Package crypto 提供 Atria 的加密工具函数。
package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	// SecretKeyLength 是 AES-256 密钥的字节长度。
	SecretKeyLength = 32
)

// LoadOrCreateKey 加载或生成 32 字节加密密钥。
// 优先级：环境变量 > 密钥文件 > 自动生成。
//
// envKey: 来自 ATRIA_SECRET_KEY 的值（base64 编码的 32 字节密钥）
// keyFilePath: 密钥文件路径（如 ./data/secret.key）
//
// 返回 32 字节原始密钥（非 base64/hex 编码）。
func LoadOrCreateKey(envKey string, keyFilePath string) ([]byte, error) {
	// 1. 环境变量优先
	if envKey != "" {
		key, err := decodeKey(envKey)
		if err != nil {
			return nil, fmt.Errorf("环境变量中的密钥格式无效: %w", err)
		}
		slog.Info("从环境变量加载密钥")
		return key, nil
	}

	// 2. 尝试读取密钥文件
	if keyFilePath != "" {
		key, err := loadKeyFromFile(keyFilePath)
		if err == nil {
			slog.Info("从文件加载密钥", "path", keyFilePath)
			return key, nil
		}

		// 文件不存在则生成新密钥
		if os.IsNotExist(err) {
			key, err := generateAndSaveKey(keyFilePath)
			if err != nil {
				return nil, fmt.Errorf("生成密钥失败: %w", err)
			}
			slog.Info("生成新密钥", "path", keyFilePath)
			return key, nil
		}

		return nil, fmt.Errorf("读取密钥文件失败: %w", err)
	}

	// 3. 无文件路径时直接生成（不持久化）
	key := make([]byte, SecretKeyLength)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成随机密钥失败: %w", err)
	}
	slog.Warn("使用临时密钥（未持久化），重启后将丢失")
	return key, nil
}

// decodeKey 解码密钥字符串，支持 base64 和 hex 两种格式。
func decodeKey(encoded string) ([]byte, error) {
	// 尝试 base64 解码
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err == nil && len(key) == SecretKeyLength {
		return key, nil
	}

	// 尝试 hex 解码
	key, err = hex.DecodeString(encoded)
	if err == nil && len(key) == SecretKeyLength {
		return key, nil
	}

	return nil, fmt.Errorf("密钥必须是 %d 字节（base64 或 hex 编码），实际解码长度不匹配", SecretKeyLength)
}

// loadKeyFromFile 从文件读取密钥。
func loadKeyFromFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 去除空白字符
	data = []byte(string(data))

	key, err := decodeKey(string(data))
	if err != nil {
		return nil, fmt.Errorf("密钥文件格式无效: %w", err)
	}

	return key, nil
}

// generateAndSaveKey 生成新密钥并保存到文件。
func generateAndSaveKey(path string) ([]byte, error) {
	// 确保目录存在
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("创建密钥目录失败: %w", err)
		}
	}

	// 生成随机密钥
	key := make([]byte, SecretKeyLength)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成随机密钥失败: %w", err)
	}

	// 以 base64 格式保存
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := os.WriteFile(path, []byte(encoded), 0600); err != nil {
		return nil, fmt.Errorf("写入密钥文件失败: %w", err)
	}

	return key, nil
}
