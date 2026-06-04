package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// ParseChecksums 解析 sha256sum 格式的 checksum 文件。
// 格式：hash  filename（每行一个）
func ParseChecksums(data []byte) (map[string]string, error) {
	result := make(map[string]string)
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			// 也尝试单空格分隔
			parts = strings.SplitN(line, " ", 2)
		}
		if len(parts) != 2 {
			continue
		}

		hash := strings.TrimSpace(parts[0])
		filename := strings.TrimSpace(parts[1])

		if hash != "" && filename != "" {
			result[filename] = hash
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("checksum 文件为空或格式无效")
	}

	return result, nil
}

// VerifyChecksum 校验文件的 SHA-256 checksum。
func VerifyChecksum(filePath string, expected string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("计算 checksum 失败: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum 不匹配: 期望 %s, 实际 %s", expected, actual)
	}

	return nil
}

// ComputeChecksum 计算文件的 SHA-256 checksum。
func ComputeChecksum(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("计算 checksum 失败: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
