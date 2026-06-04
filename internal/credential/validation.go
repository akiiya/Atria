// Package credential 提供 API 凭据管理功能。
package credential

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/user/atria/internal/model"
)

// hexRegex 匹配十六进制字符串。
var hexRegex = regexp.MustCompile(`^[0-9a-fA-F]+$`)

// ValidateDisplayName 校验凭据名称。
func ValidateDisplayName(name string) error {
	name = strings.TrimSpace(name)
	if len(name) == 0 {
		return fmt.Errorf("自定义名称不能为空")
	}
	if len(name) > 64 {
		return fmt.Errorf("自定义名称长度不能超过 64 个字符")
	}
	return nil
}

// ValidateAPIID 校验 API ID。
func ValidateAPIID(apiIDStr string) (int32, error) {
	apiIDStr = strings.TrimSpace(apiIDStr)
	if apiIDStr == "" {
		return 0, fmt.Errorf("API ID 不能为空")
	}

	id, err := strconv.ParseInt(apiIDStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("API ID 必须为正整数")
	}

	if id <= 0 {
		return 0, fmt.Errorf("API ID 必须为正整数")
	}

	return int32(id), nil
}

// ValidateAPIHash 校验 API Hash。
func ValidateAPIHash(hash string) error {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return fmt.Errorf("API Hash 不能为空")
	}
	if len(hash) != 32 {
		return fmt.Errorf("API Hash 长度必须为 32 个字符")
	}
	if !hexRegex.MatchString(hash) {
		return fmt.Errorf("API Hash 只能包含十六进制字符（0-9, a-f, A-F）")
	}
	return nil
}

// ValidateStatus 校验凭据状态。
func ValidateStatus(status string) (model.APICredentialStatus, error) {
	switch model.APICredentialStatus(status) {
	case model.APICredentialStatusEnabled:
		return model.APICredentialStatusEnabled, nil
	case model.APICredentialStatusDisabled:
		return model.APICredentialStatusDisabled, nil
	default:
		return "", fmt.Errorf("状态值不合法，允许值: enabled, disabled")
	}
}

// ValidateRiskPolicy 校验风险策略。
func ValidateRiskPolicy(policy string) (model.RiskPolicy, error) {
	switch model.RiskPolicy(policy) {
	case model.RiskPolicyDisabled:
		return model.RiskPolicyDisabled, nil
	case model.RiskPolicyEnabled:
		return model.RiskPolicyEnabled, nil
	case model.RiskPolicyConfirm:
		return model.RiskPolicyConfirm, nil
	default:
		return "", fmt.Errorf("风险策略不合法，允许值: disabled, enabled, confirm")
	}
}

// GenerateAPIHashHint 生成 API Hash 脱敏提示。
// 格式：前4位...后4位，例如 abcd...wxyz
func GenerateAPIHashHint(hash string) string {
	hash = strings.TrimSpace(hash)
	if len(hash) < 8 {
		return "***"
	}
	return hash[:4] + "..." + hash[len(hash)-4:]
}
