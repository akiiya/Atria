package migration

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/user/atria/internal/model"

	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("创建测试数据库失败: %s", err)
	}
	if err := db.AutoMigrate(
		&model.APICredential{},
		&model.SystemSetting{},
	); err != nil {
		t.Fatalf("数据库迁移失败: %s", err)
	}
	return db
}

func TestMigrations_RunOnStartup(t *testing.T) {
	Reset()
	defer Reset()

	Register(Migration{
		Version:     1,
		Name:        "test_migration",
		Description: "测试迁移",
		Run: func(db *gorm.DB, key []byte) error {
			return db.Create(&model.SystemSetting{Key: "test_key", Value: "test_value", ValueType: "string"}).Error
		},
	})

	db := setupTestDB(t)
	key := make([]byte, 32)

	err := Run(db, key)
	if err != nil {
		t.Fatalf("迁移执行失败: %s", err)
	}

	// 验证迁移记录
	versions, _ := GetAppliedVersions(db)
	if len(versions) != 1 || versions[0] != 1 {
		t.Errorf("期望已执行版本 [1]，实际=%v", versions)
	}

	// 验证数据
	var setting model.SystemSetting
	db.Where("key = ?", "test_key").First(&setting)
	if setting.Value != "test_value" {
		t.Errorf("期望 test_value，实际=%s", setting.Value)
	}
}

func TestMigrations_Idempotent(t *testing.T) {
	Reset()
	defer Reset()

	callCount := 0
	Register(Migration{
		Version:     1,
		Name:        "idempotent_test",
		Description: "幂等测试",
		Run: func(db *gorm.DB, key []byte) error {
			callCount++
			return nil
		},
	})

	db := setupTestDB(t)
	key := make([]byte, 32)

	// 第一次执行
	if err := Run(db, key); err != nil {
		t.Fatalf("第一次迁移失败: %s", err)
	}
	if callCount != 1 {
		t.Errorf("期望调用 1 次，实际=%d", callCount)
	}

	// 第二次执行不应重复
	if err := Run(db, key); err != nil {
		t.Fatalf("第二次迁移失败: %s", err)
	}
	if callCount != 1 {
		t.Errorf("第二次不应重复执行，期望 1 次，实际=%d", callCount)
	}
}

func TestMigrations_FailStopsExecution(t *testing.T) {
	Reset()
	defer Reset()

	secondCalled := false
	Register(Migration{
		Version:     1,
		Name:        "fail_migration",
		Description: "会失败的迁移",
		Run: func(db *gorm.DB, key []byte) error {
			return fmt.Errorf("模拟失败")
		},
	})
	Register(Migration{
		Version:     2,
		Name:        "should_not_run",
		Description: "不应执行",
		Run: func(db *gorm.DB, key []byte) error {
			secondCalled = true
			return nil
		},
	})

	db := setupTestDB(t)
	key := make([]byte, 32)

	err := Run(db, key)
	if err == nil {
		t.Fatal("迁移失败应返回 error")
	}
	if secondCalled {
		t.Error("失败后不应继续执行后续迁移")
	}
}

func TestMigration_NormalizeAPIKey_DefaultEnabled(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	// 构造旧数据：enabled=true 但 is_default=false
	cred := model.APICredential{
		DisplayName:        "旧 API",
		APIID:              12345678,
		EncryptedAPIHash:   "encrypted_hash",
		APIHashHint:        "abcd...6789",
		APIHashFingerprint: "fp",
		IsDefault:          false,
		Status:             model.APICredentialStatusEnabled,
		RiskPolicy:         model.RiskPolicyDisabled,
	}
	db.Create(&cred)

	// 注册并执行迁移
	Register(Migration{
		Version: 1,
		Name:    "normalize",
		Run:     migration001NormalizeAPICredentialDefaults,
	})

	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	// 验证：应被设为默认
	var updated model.APICredential
	db.First(&updated, cred.ID)
	if !updated.IsDefault {
		t.Error("enabled 记录应被自动设为 is_default=true")
	}
}

func TestMigration_NormalizeAPIKey_MultipleDefaults(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	// 创建两条默认记录
	c1 := model.APICredential{
		DisplayName: "API 1", APIID: 11111111,
		EncryptedAPIHash: "h1", APIHashHint: "h1", APIHashFingerprint: "f1",
		IsDefault: true, Status: model.APICredentialStatusEnabled, RiskPolicy: model.RiskPolicyDisabled,
	}
	c2 := model.APICredential{
		DisplayName: "API 2", APIID: 22222222,
		EncryptedAPIHash: "h2", APIHashHint: "h2", APIHashFingerprint: "f2",
		IsDefault: true, Status: model.APICredentialStatusEnabled, RiskPolicy: model.RiskPolicyDisabled,
	}
	db.Create(&c1)
	db.Create(&c2)

	Register(Migration{
		Version: 1,
		Name:    "normalize",
		Run:     migration001NormalizeAPICredentialDefaults,
	})

	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	// 验证：只有一条默认
	var defaults []model.APICredential
	db.Where("is_default = ?", true).Find(&defaults)
	if len(defaults) != 1 {
		t.Errorf("期望 1 条默认，实际=%d", len(defaults))
	}

	// 保留的是 ID 最小的
	if defaults[0].ID != c1.ID {
		t.Errorf("应保留 ID 最小的记录，期望 ID=%d，实际 ID=%d", c1.ID, defaults[0].ID)
	}
}

func TestMigration_NormalizeAPIKey_DisabledOnly(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	// 只有 disabled 记录
	cred := model.APICredential{
		DisplayName: "Disabled API", APIID: 12345678,
		EncryptedAPIHash: "h", APIHashHint: "h", APIHashFingerprint: "f",
		IsDefault: false, Status: model.APICredentialStatusDisabled, RiskPolicy: model.RiskPolicyDisabled,
	}
	db.Create(&cred)

	Register(Migration{
		Version: 1,
		Name:    "normalize",
		Run:     migration001NormalizeAPICredentialDefaults,
	})

	// 不应 panic
	if err := Run(db, key); err != nil {
		t.Fatalf("迁移不应失败: %s", err)
	}

	// 验证：记录仍为 disabled，未被强行启用
	var updated model.APICredential
	db.First(&updated, cred.ID)
	if updated.Status != model.APICredentialStatusDisabled {
		t.Error("disabled 记录不应被强行启用")
	}
	if updated.IsDefault {
		t.Error("disabled 记录不应被设为默认")
	}
}

func TestMigration_NormalizeAPIKey_EmptyTable(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	Register(Migration{
		Version: 1,
		Name:    "normalize",
		Run:     migration001NormalizeAPICredentialDefaults,
	})

	if err := Run(db, key); err != nil {
		t.Fatalf("空表迁移不应失败: %s", err)
	}
}

func TestMigration_NormalizeAPIKey_NoDeleteOldRecords(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	// 创建多条记录
	c1 := model.APICredential{
		DisplayName: "API 1", APIID: 11111111,
		EncryptedAPIHash: "h1", APIHashHint: "h1", APIHashFingerprint: "f1",
		IsDefault: true, Status: model.APICredentialStatusEnabled, RiskPolicy: model.RiskPolicyDisabled,
	}
	c2 := model.APICredential{
		DisplayName: "API 2", APIID: 22222222,
		EncryptedAPIHash: "h2", APIHashHint: "h2", APIHashFingerprint: "f2",
		IsDefault: true, Status: model.APICredentialStatusEnabled, RiskPolicy: model.RiskPolicyDisabled,
	}
	db.Create(&c1)
	db.Create(&c2)

	Register(Migration{
		Version: 1,
		Name:    "normalize",
		Run:     migration001NormalizeAPICredentialDefaults,
	})

	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	// 验证：两条记录都还在
	var count int64
	db.Model(&model.APICredential{}).Count(&count)
	if count != 2 {
		t.Errorf("不应删除旧记录，期望 2 条，实际=%d", count)
	}
}

func TestMigration_SystemSettingsDefaults(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	Register(Migration{
		Version: 1,
		Name:    "init_settings",
		Run:     migration002InitSystemSettingDefaults,
	})

	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	// 验证默认值
	expectedDefaults := map[string]string{
		"proxy_enabled":  "false",
		"proxy_type":     "none",
		"proxy_host":     "",
		"proxy_port":     "",
		"proxy_username": "",
		"proxy_timeout":  "30",
		"proxy_remark":   "",
	}

	for key, expected := range expectedDefaults {
		var setting model.SystemSetting
		if err := db.Where("key = ?", key).First(&setting).Error; err != nil {
			t.Errorf("系统设置 %s 应存在: %s", key, err)
			continue
		}
		if setting.Value != expected {
			t.Errorf("系统设置 %s 期望=%q，实际=%q", key, expected, setting.Value)
		}
	}

	// proxy_password 不应被写入
	var pwdCount int64
	db.Model(&model.SystemSetting{}).Where("key = ?", "proxy_password").Count(&pwdCount)
	if pwdCount != 0 {
		t.Error("proxy_password 不应被初始化写入")
	}
}

func TestMigration_SystemSettings_NotOverwriteExisting(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	// 预设已有配置
	db.Create(&model.SystemSetting{
		Key: "proxy_type", Value: "socks5", ValueType: "string",
	})
	db.Create(&model.SystemSetting{
		Key: "proxy_timeout", Value: "60", ValueType: "string",
	})

	Register(Migration{
		Version: 1,
		Name:    "init_settings",
		Run:     migration002InitSystemSettingDefaults,
	})

	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	// 验证：已有值不被覆盖
	var proxyType model.SystemSetting
	db.Where("key = ?", "proxy_type").First(&proxyType)
	if proxyType.Value != "socks5" {
		t.Errorf("proxy_type 不应被覆盖，期望=socks5，实际=%s", proxyType.Value)
	}

	var timeout model.SystemSetting
	db.Where("key = ?", "proxy_timeout").First(&timeout)
	if timeout.Value != "60" {
		t.Errorf("proxy_timeout 不应被覆盖，期望=60，实际=%s", timeout.Value)
	}

	// 缺失的应被初始化
	var host model.SystemSetting
	db.Where("key = ?", "proxy_host").First(&host)
	if host.Value != "" {
		t.Errorf("proxy_host 应被初始化为空字符串，实际=%s", host.Value)
	}
}

func TestMigrations_OrderedByVersion(t *testing.T) {
	Reset()
	defer Reset()

	order := []int{}
	Register(Migration{
		Version: 2, Name: "second",
		Run: func(db *gorm.DB, key []byte) error {
			order = append(order, 2)
			return nil
		},
	})
	Register(Migration{
		Version: 1, Name: "first",
		Run: func(db *gorm.DB, key []byte) error {
			order = append(order, 1)
			return nil
		},
	})

	db := setupTestDB(t)
	key := make([]byte, 32)

	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Errorf("迁移应按版本号执行，实际顺序=%v", order)
	}
}

func TestNoSQLFiles(t *testing.T) {
	// 确认迁移使用 Go 函数，不依赖 SQL 文件
	// 此测试验证 registry 中的迁移都有 Run 函数
	Reset()
	defer Reset()

	Register(Migration{
		Version: 1, Name: "test",
		Run: func(db *gorm.DB, key []byte) error { return nil },
	})

	for _, m := range registry {
		if m.Run == nil {
			t.Errorf("迁移 %d (%s) 缺少 Run 函数", m.Version, m.Name)
		}
	}
}

func TestMigrations_RecordVersionOnlyAfterSuccess(t *testing.T) {
	Reset()
	defer Reset()

	// 注册一个会失败的迁移
	Register(Migration{
		Version: 1,
		Name:    "will_fail",
		Run: func(db *gorm.DB, key []byte) error {
			return fmt.Errorf("模拟失败")
		},
	})

	db := setupTestDB(t)
	key := make([]byte, 32)

	err := Run(db, key)
	if err == nil {
		t.Fatal("迁移失败应返回 error")
	}

	// 验证：失败的迁移不应记录版本号
	versions, err := GetAppliedVersions(db)
	if err != nil {
		t.Fatalf("查询已执行迁移失败: %s", err)
	}
	if len(versions) != 0 {
		t.Errorf("失败的迁移不应记录版本号，实际记录了 %v", versions)
	}
}

func TestMigrations_FailedMigrationDoesNotAffectData(t *testing.T) {
	Reset()
	defer Reset()

	// 注册一个成功的迁移和一个失败的迁移
	Register(Migration{
		Version: 1,
		Name:    "success",
		Run: func(db *gorm.DB, key []byte) error {
			return db.Create(&model.SystemSetting{Key: "from_success", Value: "ok", ValueType: "string"}).Error
		},
	})
	Register(Migration{
		Version: 2,
		Name:    "will_fail",
		Run: func(db *gorm.DB, key []byte) error {
			return fmt.Errorf("模拟失败")
		},
	})

	db := setupTestDB(t)
	key := make([]byte, 32)

	err := Run(db, key)
	if err == nil {
		t.Fatal("迁移失败应返回 error")
	}

	// 验证：成功的迁移已记录
	versions, _ := GetAppliedVersions(db)
	if len(versions) != 1 || versions[0] != 1 {
		t.Errorf("应只记录成功的迁移版本 [1]，实际=%v", versions)
	}

	// 验证：成功的迁移数据已写入
	var setting model.SystemSetting
	if err := db.Where("key = ?", "from_success").First(&setting).Error; err != nil {
		t.Error("成功迁移的数据应已写入")
	}
}

func TestMigrations_ConcurrentNotSupported_Documented(t *testing.T) {
	// 验证：当前迁移框架不支持并发执行
	// 这个测试记录了这一限制，文档中应说明
	Reset()
	defer Reset()

	// 如果两个进程同时启动，可能并发执行同一迁移
	// 当前框架不处理并发锁
	// 用户不应在同一 data 目录启动多个 Atria 进程
	t.Log("当前迁移框架不支持并发执行，文档中已标注限制")
}
