package auth

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost 是 bcrypt 哈希的 cost factor。
	BcryptCost = 12

	// MinPasswordLength 是密码最小长度。
	MinPasswordLength = 10

	// MaxPasswordLength 是密码最大长度。
	MaxPasswordLength = 128

	// MinUsernameLength 是用户名最小长度。
	MinUsernameLength = 3

	// MaxUsernameLength 是用户名最大长度。
	MaxUsernameLength = 32
)

// HashPassword 使用 bcrypt 哈希密码。
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt 哈希失败: %w", err)
	}
	return string(hash), nil
}

// CheckPassword 验证密码是否匹配哈希值。
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ValidateUsername 校验用户名合法性。
func ValidateUsername(username string) error {
	if len(username) < MinUsernameLength {
		return fmt.Errorf("用户名长度不能少于 %d 个字符", MinUsernameLength)
	}
	if len(username) > MaxUsernameLength {
		return fmt.Errorf("用户名长度不能超过 %d 个字符", MaxUsernameLength)
	}
	for _, c := range username {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return fmt.Errorf("用户名只能包含字母、数字、下划线和短横线")
		}
	}
	return nil
}

// ValidatePassword 校验密码策略。
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("密码长度不能少于 %d 位", MinPasswordLength)
	}
	if len(password) > MaxPasswordLength {
		return fmt.Errorf("密码长度不能超过 %d 位", MaxPasswordLength)
	}
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("密码不能全是空白字符")
	}
	return nil
}
