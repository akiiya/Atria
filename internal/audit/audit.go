// Package audit 提供审计日志写入辅助函数。
package audit

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/user/atria/internal/model"
	"gorm.io/gorm"
)

// Event 是审计日志事件。
type Event struct {
	ActorType    string         // 操作者类型：admin、system
	ActorID      string         // 操作者 ID
	Action       string         // 操作标识，如 api_credential.create
	ResourceType string         // 资源类型，如 api_credential
	ResourceID   string         // 资源 ID
	RiskLevel    string         // 风险等级：low、medium、high、critical
	IP           string         // 客户端 IP
	UserAgent    string         // 客户端 User-Agent
	Message      string         // 人类可读描述
	Metadata     map[string]any // 附加元数据（将被过滤敏感字段）
}

// sensitiveKeys 是需要过滤的敏感字段名称（小写匹配）。
var sensitiveKeys = map[string]bool{
	"password":      true,
	"password_hash": true,
	"api_hash":      true,
	"session":       true,
	"token":         true,
	"code":          true,
	"two_factor":    true,
	"secret":        true,
	"secret_key":    true,
	"csrf_token":    true,
	"cookie":        true,
	"authorization": true,
}

// filterMetadata 过滤元数据中的敏感字段。
// 匹配规则：key 名称包含敏感关键词（大小写不敏感）。
func filterMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}

	filtered := make(map[string]any, len(metadata))
	for k, v := range metadata {
		lowerKey := strings.ToLower(k)
		sensitive := false
		for sensitiveKey := range sensitiveKeys {
			if strings.Contains(lowerKey, sensitiveKey) {
				sensitive = true
				break
			}
		}
		if sensitive {
			filtered[k] = "***REDACTED***"
		} else {
			filtered[k] = v
		}
	}
	return filtered
}

// Log 写入审计日志。
//
// 安全要求：
//   - Metadata 中的敏感字段会被自动替换为 ***REDACTED***
//   - 不记录密码、API Hash、Session、验证码、2FA 密码等
func Log(ctx context.Context, db *gorm.DB, event Event) error {
	// 过滤敏感字段
	filteredMetadata := filterMetadata(event.Metadata)

	var metadataJSON string
	if filteredMetadata != nil {
		data, err := json.Marshal(filteredMetadata)
		if err != nil {
			metadataJSON = "{}"
		} else {
			metadataJSON = string(data)
		}
	}

	auditLog := model.AuditLog{
		ActorType:    event.ActorType,
		ActorID:      0, // 由调用方在需要时设置
		Action:       event.Action,
		ResourceType: event.ResourceType,
		ResourceID:   0, // 由调用方在需要时设置
		RiskLevel:    event.RiskLevel,
		IP:           event.IP,
		UserAgent:    event.UserAgent,
		Message:      event.Message,
		MetadataJSON: metadataJSON,
		CreatedAt:    time.Now(),
	}

	return db.WithContext(ctx).Create(&auditLog).Error
}
