package credential

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/security"

	"gorm.io/gorm"
)

// setupTestDB 创建测试用的内存数据库。
func setupTestDB(t *testing.T) (*gorm.DB, []byte) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("创建测试数据库失败: %s", err)
	}

	if err := db.AutoMigrate(&model.APICredential{}, &model.TelegramAccount{}); err != nil {
		t.Fatalf("数据库迁移失败: %s", err)
	}

	// 测试密钥
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	return db, key
}

func TestService_Create_Success(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, err := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err != nil {
		t.Fatalf("创建凭据失败: %s", err)
	}

	if cred.ID == 0 {
		t.Error("凭据 ID 不应为 0")
	}
	if cred.DisplayName != "测试凭据" {
		t.Errorf("DisplayName 不匹配，实际=%s", cred.DisplayName)
	}
}

func TestService_Create_EncryptedHash(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	apiHash := "abcdef0123456789abcdef0123456789"
	cred, err := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     apiHash,
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err != nil {
		t.Fatalf("创建凭据失败: %s", err)
	}

	if cred.EncryptedAPIHash == apiHash {
		t.Error("encrypted_api_hash 不应等于明文")
	}
	if cred.EncryptedAPIHash == "" {
		t.Error("encrypted_api_hash 不应为空")
	}
}

func TestService_Create_HintFormat(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	apiHash := "abcdef0123456789abcdef0123456789"
	cred, err := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     apiHash,
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err != nil {
		t.Fatalf("创建凭据失败: %s", err)
	}

	expected := "abcd...6789"
	if cred.APIHashHint != expected {
		t.Errorf("APIHashHint 格式不正确，期望=%s，实际=%s", expected, cred.APIHashHint)
	}
}

func TestService_Create_FingerprintNotEmpty(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, err := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err != nil {
		t.Fatalf("创建凭据失败: %s", err)
	}

	if cred.APIHashFingerprint == "" {
		t.Error("APIHashFingerprint 不应为空")
	}
}

func TestService_Create_CanDecrypt(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	apiHash := "abcdef0123456789abcdef0123456789"
	cred, err := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     apiHash,
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err != nil {
		t.Fatalf("创建凭据失败: %s", err)
	}

	decrypted, err := security.DecryptAPIHash(key, cred.EncryptedAPIHash)
	if err != nil {
		t.Fatalf("解密失败: %s", err)
	}

	if decrypted != apiHash {
		t.Errorf("解密结果不匹配，期望=%s，实际=%s", apiHash, decrypted)
	}
}

func TestService_Create_EmptyDisplayName(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	_, err := svc.Create(CreateInput{
		DisplayName: "",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err == nil {
		t.Error("空 DisplayName 应该失败")
	}
}

func TestService_Create_InvalidAPIID(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	_, err := svc.Create(CreateInput{
		DisplayName: "测试",
		APIID:       "abc",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err == nil {
		t.Error("非法 API ID 应该失败")
	}
}

func TestService_Create_EmptyAPIHash(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	_, err := svc.Create(CreateInput{
		DisplayName: "测试",
		APIID:       "12345678",
		APIHash:     "",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err == nil {
		t.Error("空 API Hash 应该失败")
	}
}

func TestService_Create_InvalidHashLength(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	_, err := svc.Create(CreateInput{
		DisplayName: "测试",
		APIID:       "12345678",
		APIHash:     "short",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err == nil {
		t.Error("短 API Hash 应该失败")
	}
}

func TestService_Create_InvalidHashChars(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	_, err := svc.Create(CreateInput{
		DisplayName: "测试",
		APIID:       "12345678",
		APIHash:     "ghij000000000000000000000000000000",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err == nil {
		t.Error("非 hex 字符的 API Hash 应该失败")
	}
}

func TestService_Create_InvalidRiskPolicy(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	_, err := svc.Create(CreateInput{
		DisplayName: "测试",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "invalid",
	})
	if err == nil {
		t.Error("非法 RiskPolicy 应该失败")
	}
}

func TestService_Create_InvalidStatus(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	_, err := svc.Create(CreateInput{
		DisplayName: "测试",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "invalid",
		RiskPolicy:  "disabled",
	})
	if err == nil {
		t.Error("非法 Status 应该失败")
	}
}

func TestService_Update_KeepHashWhenEmpty(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, _ := svc.Create(CreateInput{
		DisplayName: "原始名称",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	originalHash := cred.EncryptedAPIHash

	updated, err := svc.Update(cred.ID, UpdateInput{
		DisplayName: "新名称",
		APIID:       "12345678",
		APIHash:     "", // 空表示保持不变
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err != nil {
		t.Fatalf("更新失败: %s", err)
	}

	if updated.EncryptedAPIHash != originalHash {
		t.Error("空 API Hash 时应保持原 hash 不变")
	}
	if updated.DisplayName != "新名称" {
		t.Errorf("DisplayName 应更新，实际=%s", updated.DisplayName)
	}
}

func TestService_Update_RotateHash(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, _ := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	originalHash := cred.EncryptedAPIHash
	originalHint := cred.APIHashHint

	newAPIHash := "11112222333344445555666677778888"
	updated, err := svc.Update(cred.ID, UpdateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     newAPIHash,
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err != nil {
		t.Fatalf("更新失败: %s", err)
	}

	if updated.EncryptedAPIHash == originalHash {
		t.Error("填写新 API Hash 时应重新加密")
	}
	if updated.APIHashHint == originalHint {
		t.Error("填写新 API Hash 时应更新 hint")
	}
}

func TestService_UpdateStatus_Success(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, _ := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	err := svc.UpdateStatus(cred.ID, "disabled")
	if err != nil {
		t.Fatalf("禁用失败: %s", err)
	}

	updated, _ := svc.GetByID(cred.ID)
	if updated.Status != model.APICredentialStatusDisabled {
		t.Error("状态应为 disabled")
	}
}

func TestService_UpdateStatus_ReEnable(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, _ := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	svc.UpdateStatus(cred.ID, "disabled")
	svc.UpdateStatus(cred.ID, "enabled")

	updated, _ := svc.GetByID(cred.ID)
	if updated.Status != model.APICredentialStatusEnabled {
		t.Error("状态应为 enabled")
	}
}

func TestService_Delete_Success(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, _ := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	err := svc.Delete(cred.ID)
	if err != nil {
		t.Fatalf("删除失败: %s", err)
	}

	// 删除后列表不应显示
	list, _ := svc.List()
	for _, c := range list {
		if c.ID == cred.ID {
			t.Error("删除后列表不应显示该凭据")
		}
	}
}

func TestService_Delete_WithBinding(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, _ := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// 创建绑定
	binding := &model.TelegramAccount{
		APICredentialID: cred.ID,
		UserID:          12345,
		PhoneEncrypted:  "encrypted",
		Status:          "active",
	}
	db.Create(binding)

	err := svc.Delete(cred.ID)
	if err == nil {
		t.Error("已绑定账号的凭据应该删除失败")
	}
	if err != nil && !strings.Contains(err.Error(), "只能禁用") {
		t.Errorf("错误提示应包含'只能禁用'，实际=%s", err.Error())
	}
}

func TestService_ListEnabled_FilterDisabled(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	svc.Create(CreateInput{
		DisplayName: "启用凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	cred2, _ := svc.Create(CreateInput{
		DisplayName: "禁用凭据",
		APIID:       "87654321",
		APIHash:     "11112222333344445555666677778888",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	svc.UpdateStatus(cred2.ID, "disabled")

	list, err := svc.ListEnabled()
	if err != nil {
		t.Fatalf("查询失败: %s", err)
	}

	if len(list) != 1 {
		t.Errorf("应只有 1 条启用凭据，实际=%d", len(list))
	}
}

func TestService_IsValidCredential(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, _ := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	if !svc.IsValidCredential(cred.ID) {
		t.Error("启用凭据应返回 true")
	}

	svc.UpdateStatus(cred.ID, "disabled")
	if svc.IsValidCredential(cred.ID) {
		t.Error("禁用凭据应返回 false")
	}

	if svc.IsValidCredential(999) {
		t.Error("不存在的凭据应返回 false")
	}
}

func TestGenerateAPIHashHint(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abcdef0123456789abcdef0123456789", "abcd...6789"},
		{"1234567890abcdef1234567890abcdef", "1234...cdef"},
		{"short", "***"},
		{"", "***"},
	}

	for _, tt := range tests {
		result := GenerateAPIHashHint(tt.input)
		if result != tt.expected {
			t.Errorf("GenerateAPIHashHint(%q) = %q, 期望 %q", tt.input, result, tt.expected)
		}
	}
}
