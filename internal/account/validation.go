// Package account 提供 Telegram 账号管理业务逻辑。
package account

import (
	"fmt"
	"regexp"
	"strings"
)

// phoneRegex 匹配手机号格式（以 + 开头，允许数字和 +）。
var phoneRegex = regexp.MustCompile(`^\+[0-9]{7,19}$`)

// ValidatePhone 校验手机号格式。
func ValidatePhone(phone string) error {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return fmt.Errorf("手机号不能为空")
	}
	if !strings.HasPrefix(phone, "+") {
		return fmt.Errorf("手机号必须以 + 开头")
	}
	if len(phone) < 8 || len(phone) > 20 {
		return fmt.Errorf("手机号长度必须在 8-20 个字符之间")
	}
	if !phoneRegex.MatchString(phone) {
		return fmt.Errorf("手机号格式不正确，只允许数字和 +")
	}
	return nil
}
