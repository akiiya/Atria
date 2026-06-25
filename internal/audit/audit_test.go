package audit

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/user/atria/internal/model"
	"gorm.io/gorm"
)

func TestFilterMetadata_SensitiveKeys(t *testing.T) {
	metadata := map[string]any{
		"username":     "admin",
		"password":     "secret123",
		"api_hash":     "abc123",
		"session_data": "raw_session",
		"token":        "bearer_token",
		"code":         "123456",
		"two_factor":   "2fa_secret",
		"secret_key":   "key_value",
		"display_name": "Production",
	}

	filtered := filterMetadata(metadata)

	// 非敏感字段应保留
	if filtered["username"] != "admin" {
		t.Error("username 应保留")
	}
	if filtered["display_name"] != "Production" {
		t.Error("display_name 应保留")
	}

	// 敏感字段应被替换
	sensitiveFields := []string{"password", "api_hash", "session_data", "token", "code", "two_factor", "secret_key"}
	for _, field := range sensitiveFields {
		if filtered[field] != "***REDACTED***" {
			t.Errorf("%s 应被替换为 ***REDACTED***，实际=%v", field, filtered[field])
		}
	}
}

func TestFilterMetadata_CaseInsensitive(t *testing.T) {
	metadata := map[string]any{
		"Password":   "secret",
		"API_HASH":   "hash",
		"SessionKey": "key",
	}

	filtered := filterMetadata(metadata)

	for _, field := range []string{"Password", "API_HASH", "SessionKey"} {
		if filtered[field] != "***REDACTED***" {
			t.Errorf("%s 应被替换（大小写不敏感）", field)
		}
	}
}

func TestFilterMetadata_NilInput(t *testing.T) {
	filtered := filterMetadata(nil)
	if filtered != nil {
		t.Error("nil 输入应返回 nil")
	}
}

func TestFilterMetadata_EmptyInput(t *testing.T) {
	filtered := filterMetadata(map[string]any{})
	if len(filtered) != 0 {
		t.Error("空输入应返回空 map")
	}
}

func TestFilterMetadata_PartialMatch(t *testing.T) {
	metadata := map[string]any{
		"user_password_hash":   "hash",
		"api_hash_fingerprint": "fp",
		"session_file_path":    "path",
	}

	filtered := filterMetadata(metadata)

	// 包含敏感关键词的字段应被替换
	for _, field := range []string{"user_password_hash", "api_hash_fingerprint", "session_file_path"} {
		if filtered[field] != "***REDACTED***" {
			t.Errorf("%s 应被替换（包含敏感关键词）", field)
		}
	}
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("创建测试数据库失败: %s", err)
	}
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("数据库迁移失败: %s", err)
	}
	return db
}

func TestLog_SavesAccountID(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := Log(ctx, db, Event{
		ActorType:    "admin",
		Action:       "test.action",
		ResourceType: "test",
		AccountID:    42,
		RiskLevel:    "low",
		Message:      "test with account",
	})
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	var log model.AuditLog
	db.First(&log)
	if log.AccountID != 42 {
		t.Errorf("expected AccountID=42, got %d", log.AccountID)
	}
}
