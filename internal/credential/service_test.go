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

	// 创建两个凭据，这样可以禁用非默认的那个
	cred1, _ := svc.Create(CreateInput{
		DisplayName: "默认凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	cred2, _ := svc.Create(CreateInput{
		DisplayName: "备用凭据",
		APIID:       "87654321",
		APIHash:     "11112222333344445555666677778888",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// 禁用非默认凭据
	err := svc.UpdateStatus(cred2.ID, "disabled")
	if err != nil {
		t.Fatalf("禁用失败: %s", err)
	}

	updated, _ := svc.GetByID(cred2.ID)
	if updated.Status != model.APICredentialStatusDisabled {
		t.Error("状态应为 disabled")
	}

	// 默认凭据仍应为默认
	defaultCred, _ := svc.GetByID(cred1.ID)
	if !defaultCred.IsDefault {
		t.Error("默认凭据不应改变")
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

	// 创建两个凭据
	svc.Create(CreateInput{
		DisplayName: "默认凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	cred2, _ := svc.Create(CreateInput{
		DisplayName: "备用凭据",
		APIID:       "87654321",
		APIHash:     "11112222333344445555666677778888",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// 删除非默认凭据
	err := svc.Delete(cred2.ID)
	if err != nil {
		t.Fatalf("删除失败: %s", err)
	}

	// 删除后列表不应显示
	list, _ := svc.List()
	for _, c := range list {
		if c.ID == cred2.ID {
			t.Error("删除后列表不应显示该凭据")
		}
	}
}

func TestService_Delete_WithBinding(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	// 创建两个凭据
	svc.Create(CreateInput{
		DisplayName: "默认凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	cred2, _ := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "87654321",
		APIHash:     "11112222333344445555666677778888",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// 创建绑定
	binding := &model.TelegramAccount{
		APICredentialID: cred2.ID,
		UserID:          12345,
		PhoneEncrypted:  "encrypted",
		Status:          "active",
	}
	db.Create(binding)

	err := svc.Delete(cred2.ID)
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

	// 创建两个凭据
	cred1, _ := svc.Create(CreateInput{
		DisplayName: "默认凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	cred2, _ := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "87654321",
		APIHash:     "11112222333344445555666677778888",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	if !svc.IsValidCredential(cred1.ID) {
		t.Error("启用凭据应返回 true")
	}

	// 禁用非默认凭据
	svc.UpdateStatus(cred2.ID, "disabled")
	if svc.IsValidCredential(cred2.ID) {
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

// ===== 默认凭据测试 =====

func TestService_Create_FirstCredential_IsDefault(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, err := svc.Create(CreateInput{
		DisplayName: "第一个凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err != nil {
		t.Fatalf("创建凭据失败: %s", err)
	}

	if !cred.IsDefault {
		t.Error("第一个凭据应自动成为默认")
	}
}

func TestService_Create_SecondCredential_NotDefault(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	svc.Create(CreateInput{
		DisplayName: "第一个凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	cred2, _ := svc.Create(CreateInput{
		DisplayName: "第二个凭据",
		APIID:       "87654321",
		APIHash:     "11112222333344445555666677778888",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	if cred2.IsDefault {
		t.Error("第二个凭据不应自动成为默认")
	}
}

func TestService_EnsureDefault_FirstEnabled(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	// 创建凭据但手动清除默认标记
	cred, _ := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	db.Model(&cred).Update("is_default", false)

	// 执行 EnsureDefault
	err := svc.EnsureDefault()
	if err != nil {
		t.Fatalf("EnsureDefault 失败: %s", err)
	}

	// 验证已设为默认
	updated, _ := svc.GetByID(cred.ID)
	if !updated.IsDefault {
		t.Error("EnsureDefault 应将第一个启用凭据设为默认")
	}
}

func TestService_EnsureDefault_AlreadyHasDefault(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	svc.Create(CreateInput{
		DisplayName: "第一个凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	cred2, _ := svc.Create(CreateInput{
		DisplayName: "第二个凭据",
		APIID:       "87654321",
		APIHash:     "11112222333344445555666677778888",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// 执行 EnsureDefault（不应改变现有默认）
	err := svc.EnsureDefault()
	if err != nil {
		t.Fatalf("EnsureDefault 失败: %s", err)
	}

	// 第二个凭据不应成为默认
	updated, _ := svc.GetByID(cred2.ID)
	if updated.IsDefault {
		t.Error("已有默认时 EnsureDefault 不应改变")
	}
}

func TestService_SetDefault_OnlyOneDefault(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred1, _ := svc.Create(CreateInput{
		DisplayName: "第一个凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	cred2, _ := svc.Create(CreateInput{
		DisplayName: "第二个凭据",
		APIID:       "87654321",
		APIHash:     "11112222333344445555666677778888",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// 设置第二个为默认
	err := svc.SetDefault(cred2.ID)
	if err != nil {
		t.Fatalf("SetDefault 失败: %s", err)
	}

	// 验证只有一个默认
	updated1, _ := svc.GetByID(cred1.ID)
	updated2, _ := svc.GetByID(cred2.ID)

	if updated1.IsDefault {
		t.Error("第一个凭据不应再是默认")
	}
	if !updated2.IsDefault {
		t.Error("第二个凭据应成为默认")
	}
}

func TestService_GetDefault_Success(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, _ := svc.Create(CreateInput{
		DisplayName: "测试凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	defaultCred, err := svc.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault 失败: %s", err)
	}

	if defaultCred.ID != cred.ID {
		t.Error("GetDefault 返回的凭据不正确")
	}
}

func TestService_GetDefault_None(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	_, err := svc.GetDefault()
	if err == nil {
		t.Error("没有凭据时 GetDefault 应返回错误")
	}
}

func TestService_DisableDefault_WithOtherEnabled(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred1, _ := svc.Create(CreateInput{
		DisplayName: "默认凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	svc.Create(CreateInput{
		DisplayName: "备用凭据",
		APIID:       "87654321",
		APIHash:     "11112222333344445555666677778888",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// 禁用默认凭据（应自动切换默认）
	err := svc.UpdateStatus(cred1.ID, "disabled")
	if err != nil {
		t.Fatalf("禁用默认凭据失败: %s", err)
	}

	// 验证默认已切换
	defaultCred, _ := svc.GetDefault()
	if defaultCred.ID == cred1.ID {
		t.Error("默认凭据应已切换到其它凭据")
	}
}

func TestService_DisableDefault_WithoutOtherEnabled(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, _ := svc.Create(CreateInput{
		DisplayName: "唯一凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// 禁用唯一的默认凭据（应失败）
	err := svc.UpdateStatus(cred.ID, "disabled")
	if err == nil {
		t.Error("禁用唯一的默认凭据应该失败")
	}
}

func TestService_Delete_DefaultCredential_Protected(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	cred, _ := svc.Create(CreateInput{
		DisplayName: "默认凭据",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// 删除默认凭据（应失败）
	err := svc.Delete(cred.ID)
	if err == nil {
		t.Error("删除默认凭据应该失败")
	}
	if err != nil && !strings.Contains(err.Error(), "默认凭据") {
		t.Errorf("错误提示应包含'默认凭据'，实际=%s", err.Error())
	}
}

// ===== GetSystemAPIKey 测试 =====

func TestService_GetSystemAPIKey_ReadAfterWrite(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	// 创建一个 API Key
	created, err := svc.Create(CreateInput{
		DisplayName: "Test API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err != nil {
		t.Fatalf("创建凭据失败: %s", err)
	}

	// 立即读取系统 API Key
	systemKey, err := svc.GetSystemAPIKey()
	if err != nil {
		t.Fatalf("GetSystemAPIKey 失败: %s", err)
	}
	if systemKey == nil {
		t.Fatal("GetSystemAPIKey 不应返回 nil")
	}

	// 验证字段
	if systemKey.ID != created.ID {
		t.Errorf("ID 不匹配，期望=%d，实际=%d", created.ID, systemKey.ID)
	}
	if systemKey.DisplayName != "Test API" {
		t.Errorf("DisplayName 不匹配，期望=Test API，实际=%s", systemKey.DisplayName)
	}
	if !systemKey.IsDefault {
		t.Error("应为默认凭据")
	}
	if systemKey.Status != model.APICredentialStatusEnabled {
		t.Errorf("Status 应为 enabled，实际=%s", systemKey.Status)
	}
	if systemKey.APIID != 12345678 {
		t.Errorf("APIID 应为 12345678，实际=%d", systemKey.APIID)
	}
	// API Hash 不应是明文
	if systemKey.EncryptedAPIHash == "abcdef0123456789abcdef0123456789" {
		t.Error("EncryptedAPIHash 不应是明文")
	}
}

func TestService_GetSystemAPIKey_NoEnabledKey_ReturnsNil(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	// 没有任何凭据时
	systemKey, err := svc.GetSystemAPIKey()
	if err != nil {
		t.Fatalf("GetSystemAPIKey 失败: %s", err)
	}
	if systemKey != nil {
		t.Error("没有启用凭据时应返回 nil")
	}
}

func TestService_GetSystemAPIKey_LegacyEnabledKey_AutoSetDefault(t *testing.T) {
	db, key := setupTestDB(t)
	svc := NewService(db, key)

	// 创建一个凭据
	created, err := svc.Create(CreateInput{
		DisplayName: "Legacy API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	if err != nil {
		t.Fatalf("创建凭据失败: %s", err)
	}

	// 手动将 is_default 设为 false（模拟旧数据）
	db.Model(&model.APICredential{}).Where("id = ?", created.ID).Update("is_default", false)

	// 验证 GetDefault 找不到
	defaultCred, _ := svc.GetDefault()
	if defaultCred != nil {
		t.Error("GetDefault 不应找到非默认凭据")
	}

	// GetSystemAPIKey 应能找到并自动设为默认
	systemKey, err := svc.GetSystemAPIKey()
	if err != nil {
		t.Fatalf("GetSystemAPIKey 失败: %s", err)
	}
	if systemKey == nil {
		t.Fatal("GetSystemAPIKey 不应返回 nil")
	}
	if systemKey.ID != created.ID {
		t.Errorf("ID 不匹配，期望=%d，实际=%d", created.ID, systemKey.ID)
	}

	// 验证已被设为默认
	var updated model.APICredential
	db.First(&updated, created.ID)
	if !updated.IsDefault {
		t.Error("GetSystemAPIKey 应自动将启用凭据设为默认")
	}
}
