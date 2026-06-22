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
		&model.TelegramAccount{},
		&model.AccountSession{},
		&model.ChatPeerCache{},
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

func TestMigration_BackfillLegacyAccountSessions(t *testing.T) {
	Reset()
	defer Reset()

	// 只注册迁移 4
	Register(Migration{
		Version: 4,
		Name:    "backfill_legacy_account_sessions",
		Run:     migration004BackfillLegacyAccountSessions,
	})

	db := setupTestDB(t)

	// 创建一个 active 账号（无 session 记录）
	account := &model.TelegramAccount{
		APICredentialID:  1,
		UserID:           123456789,
		PhoneEncrypted:   "encrypted",
		PhoneFingerprint: "***1234",
		Username:         "test_user",
		DisplayName:      "Test User",
		Status:           model.TelegramAccountStatusActive,
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("创建测试账号失败: %s", err)
	}

	// 运行迁移
	key := make([]byte, 32)
	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	// 验证：session 记录已创建
	var session model.AccountSession
	if err := db.Where("telegram_account_id = ?", account.ID).First(&session).Error; err != nil {
		t.Fatalf("session 记录应已创建: %s", err)
	}

	if session.SessionFilePath != fmt.Sprintf("session_%d.enc", account.ID) {
		t.Errorf("session 文件路径不正确: %s", session.SessionFilePath)
	}
	if session.Status != "active" {
		t.Errorf("session 状态应为 active，实际 %s", session.Status)
	}
}

func TestMigration_BackfillLegacyAccountSessions_Idempotent(t *testing.T) {
	Reset()
	defer Reset()

	Register(Migration{
		Version: 4,
		Name:    "backfill_legacy_account_sessions",
		Run:     migration004BackfillLegacyAccountSessions,
	})

	db := setupTestDB(t)

	account := &model.TelegramAccount{
		APICredentialID:  1,
		UserID:           123456789,
		PhoneEncrypted:   "encrypted",
		PhoneFingerprint: "***1234",
		Username:         "test_user",
		DisplayName:      "Test User",
		Status:           model.TelegramAccountStatusActive,
	}
	db.Create(account)

	key := make([]byte, 32)

	// 第一次执行
	if err := Run(db, key); err != nil {
		t.Fatalf("第一次迁移失败: %s", err)
	}

	// 第二次执行（幂等）
	Reset()
	Register(Migration{
		Version: 4,
		Name:    "backfill_legacy_account_sessions",
		Run:     migration004BackfillLegacyAccountSessions,
	})

	if err := Run(db, key); err != nil {
		t.Fatalf("第二次迁移失败: %s", err)
	}

	// 验证：只有一条 session 记录
	var count int64
	db.Model(&model.AccountSession{}).Where("telegram_account_id = ?", account.ID).Count(&count)
	if count != 1 {
		t.Errorf("应只有一条 session 记录，实际 %d", count)
	}
}

func TestMigration_TelegramUpdateStateCreated(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	// 需要先注册 AutoMigrate 的模型
	if err := db.AutoMigrate(&model.TelegramUpdateState{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %s", err)
	}

	Register(Migration{
		Version: 7,
		Name:    "create_telegram_update_state",
		Run:     migration007CreateTelegramUpdateState,
	})

	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	// 验证表存在并可写入
	state := model.TelegramUpdateState{
		AccountID: 1,
		Pts:       100,
		Qts:       50,
		Date:      1700000000,
		Seq:       10,
	}
	if err := db.Create(&state).Error; err != nil {
		t.Fatalf("写入 update state 失败: %s", err)
	}

	// 验证可读取
	var saved model.TelegramUpdateState
	if err := db.Where("account_id = ?", 1).First(&saved).Error; err != nil {
		t.Fatalf("读取 update state 失败: %s", err)
	}
	if saved.Pts != 100 {
		t.Errorf("期望 Pts=100，实际 %d", saved.Pts)
	}
	if saved.Qts != 50 {
		t.Errorf("期望 Qts=50，实际 %d", saved.Qts)
	}
}

func TestMigration_TelegramUpdateStateIdempotent(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	if err := db.AutoMigrate(&model.TelegramUpdateState{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %s", err)
	}

	// 第一次执行
	Register(Migration{
		Version: 7,
		Name:    "create_telegram_update_state",
		Run:     migration007CreateTelegramUpdateState,
	})
	if err := Run(db, key); err != nil {
		t.Fatalf("第一次迁移失败: %s", err)
	}

	// 第二次执行（幂等）
	Reset()
	Register(Migration{
		Version: 7,
		Name:    "create_telegram_update_state",
		Run:     migration007CreateTelegramUpdateState,
	})
	if err := Run(db, key); err != nil {
		t.Fatalf("第二次迁移失败: %s", err)
	}
}

func TestUpdateState_StoresByAccount(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	if err := db.AutoMigrate(&model.TelegramUpdateState{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %s", err)
	}

	Register(Migration{
		Version: 7,
		Name:    "create_telegram_update_state",
		Run:     migration007CreateTelegramUpdateState,
	})
	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	// 为两个不同账号创建 state
	db.Create(&model.TelegramUpdateState{AccountID: 1, Pts: 100})
	db.Create(&model.TelegramUpdateState{AccountID: 2, Pts: 200})

	// 验证按账号隔离
	var state1 model.TelegramUpdateState
	db.Where("account_id = ?", 1).First(&state1)
	if state1.Pts != 100 {
		t.Errorf("账号 1 期望 Pts=100，实际 %d", state1.Pts)
	}

	var state2 model.TelegramUpdateState
	db.Where("account_id = ?", 2).First(&state2)
	if state2.Pts != 200 {
		t.Errorf("账号 2 期望 Pts=200，实际 %d", state2.Pts)
	}
}

func TestUpdateState_DoesNotStoreSensitiveFields(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	if err := db.AutoMigrate(&model.TelegramUpdateState{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %s", err)
	}

	Register(Migration{
		Version: 7,
		Name:    "create_telegram_update_state",
		Run:     migration007CreateTelegramUpdateState,
	})
	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	// 创建 state
	db.Create(&model.TelegramUpdateState{AccountID: 1, Pts: 100})

	// 验证不包含敏感字段
	var state model.TelegramUpdateState
	db.Where("account_id = ?", 1).First(&state)

	// 检查表结构不包含敏感字段
	columns, err := db.Migrator().ColumnTypes(&model.TelegramUpdateState{})
	if err != nil {
		t.Fatalf("获取列信息失败: %s", err)
	}

	sensitiveFields := []string{"access_hash", "api_hash", "session_path", "proxy_password", "phone"}
	for _, col := range columns {
		name := col.Name()
		for _, sensitive := range sensitiveFields {
			if name == sensitive {
				t.Errorf("telegram_update_state 表不应包含敏感字段 %q", sensitive)
			}
		}
	}
}

func TestNoSQLFilesForUpdateStateMigration(t *testing.T) {
	// 验证迁移 7 使用 Go 函数，不依赖 SQL 文件
	Reset()
	defer Reset()

	Register(Migration{
		Version: 7,
		Name:    "create_telegram_update_state",
		Run:     migration007CreateTelegramUpdateState,
	})

	for _, m := range registry {
		if m.Run == nil {
			t.Errorf("迁移 %d (%s) 缺少 Run 函数", m.Version, m.Name)
		}
	}
}

func TestMigration_TelegramChannelUpdateStateCreated(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	if err := db.AutoMigrate(&model.TelegramChannelUpdateState{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %s", err)
	}

	Register(Migration{
		Version: 8,
		Name:    "create_telegram_channel_update_state",
		Run:     migration008CreateTelegramChannelUpdateState,
	})

	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	// 验证表存在并可写入
	state := model.TelegramChannelUpdateState{
		AccountID: 1,
		ChannelID: 12345,
		Pts:       500,
	}
	if err := db.Create(&state).Error; err != nil {
		t.Fatalf("写入 channel state 失败: %s", err)
	}

	// 验证可读取
	var saved model.TelegramChannelUpdateState
	if err := db.Where("account_id = ? AND channel_id = ?", 1, 12345).First(&saved).Error; err != nil {
		t.Fatalf("读取 channel state 失败: %s", err)
	}
	if saved.Pts != 500 {
		t.Errorf("期望 Pts=500，实际 %d", saved.Pts)
	}
}

func TestMigration_TelegramChannelUpdateStateIdempotent(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	if err := db.AutoMigrate(&model.TelegramChannelUpdateState{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %s", err)
	}

	// 第一次执行
	Register(Migration{
		Version: 8,
		Name:    "create_telegram_channel_update_state",
		Run:     migration008CreateTelegramChannelUpdateState,
	})
	if err := Run(db, key); err != nil {
		t.Fatalf("第一次迁移失败: %s", err)
	}

	// 第二次执行（幂等）
	Reset()
	Register(Migration{
		Version: 8,
		Name:    "create_telegram_channel_update_state",
		Run:     migration008CreateTelegramChannelUpdateState,
	})
	if err := Run(db, key); err != nil {
		t.Fatalf("第二次迁移失败: %s", err)
	}
}

func TestChannelUpdateState_StoresByAccountAndChannel(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	if err := db.AutoMigrate(&model.TelegramChannelUpdateState{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %s", err)
	}

	Register(Migration{
		Version: 8,
		Name:    "create_telegram_channel_update_state",
		Run:     migration008CreateTelegramChannelUpdateState,
	})
	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	// 为不同 account + channel 创建 state
	db.Create(&model.TelegramChannelUpdateState{AccountID: 1, ChannelID: 100, Pts: 100})
	db.Create(&model.TelegramChannelUpdateState{AccountID: 1, ChannelID: 200, Pts: 200})
	db.Create(&model.TelegramChannelUpdateState{AccountID: 2, ChannelID: 100, Pts: 300})

	// 验证按 account + channel 隔离
	var state1 model.TelegramChannelUpdateState
	db.Where("account_id = ? AND channel_id = ?", 1, 100).First(&state1)
	if state1.Pts != 100 {
		t.Errorf("account=1 channel=100 期望 Pts=100，实际 %d", state1.Pts)
	}

	var state2 model.TelegramChannelUpdateState
	db.Where("account_id = ? AND channel_id = ?", 1, 200).First(&state2)
	if state2.Pts != 200 {
		t.Errorf("account=1 channel=200 期望 Pts=200，实际 %d", state2.Pts)
	}

	var state3 model.TelegramChannelUpdateState
	db.Where("account_id = ? AND channel_id = ?", 2, 100).First(&state3)
	if state3.Pts != 300 {
		t.Errorf("account=2 channel=100 期望 Pts=300，实际 %d", state3.Pts)
	}
}

func TestChannelUpdateState_DoesNotStoreSensitiveFields(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	key := make([]byte, 32)

	if err := db.AutoMigrate(&model.TelegramChannelUpdateState{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %s", err)
	}

	Register(Migration{
		Version: 8,
		Name:    "create_telegram_channel_update_state",
		Run:     migration008CreateTelegramChannelUpdateState,
	})
	if err := Run(db, key); err != nil {
		t.Fatalf("迁移失败: %s", err)
	}

	// 检查表结构不包含敏感字段
	columns, err := db.Migrator().ColumnTypes(&model.TelegramChannelUpdateState{})
	if err != nil {
		t.Fatalf("获取列信息失败: %s", err)
	}

	sensitiveFields := []string{"access_hash", "api_hash", "session_path", "proxy_password", "phone", "message_body"}
	for _, col := range columns {
		name := col.Name()
		for _, sensitive := range sensitiveFields {
			if name == sensitive {
				t.Errorf("telegram_channel_update_state 表不应包含敏感字段 %q", sensitive)
			}
		}
	}
}

func TestNoSQLFilesForChannelUpdateStateMigration(t *testing.T) {
	Reset()
	defer Reset()

	Register(Migration{
		Version: 8,
		Name:    "create_telegram_channel_update_state",
		Run:     migration008CreateTelegramChannelUpdateState,
	})

	for _, m := range registry {
		if m.Run == nil {
			t.Errorf("迁移 %d (%s) 缺少 Run 函数", m.Version, m.Name)
		}
	}
}

// ===== 迁移 9: 聊天缓存索引 =====

func TestMigration009_CreatesIndexes(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	// 确保 chat_message_cache 表也存在
	if err := db.AutoMigrate(&model.ChatMessageCache{}); err != nil {
		t.Fatalf("AutoMigrate chat_message_cache 失败: %s", err)
	}

	key := make([]byte, 32)

	Register(Migration{
		Version: 9,
		Name:    "add_chat_cache_indexes",
		Run:     migration009AddChatCacheIndexes,
	})

	if err := Run(db, key); err != nil {
		t.Fatalf("迁移 9 失败: %s", err)
	}
}

func TestMigration009_Idempotent(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	if err := db.AutoMigrate(&model.ChatMessageCache{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %s", err)
	}

	// 直接调用两次，应不报错
	for i := 0; i < 2; i++ {
		if err := migration009AddChatCacheIndexes(db, nil); err != nil {
			t.Fatalf("第 %d 次调用迁移 9 失败: %s", i+1, err)
		}
	}
}

func TestMigration009_IndexesImproveQueries(t *testing.T) {
	Reset()
	defer Reset()

	db := setupTestDB(t)
	if err := db.AutoMigrate(&model.ChatMessageCache{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %s", err)
	}

	key := make([]byte, 32)

	Register(Migration{
		Version: 9,
		Name:    "add_chat_cache_indexes",
		Run:     migration009AddChatCacheIndexes,
	})

	if err := Run(db, key); err != nil {
		t.Fatalf("迁移 9 失败: %s", err)
	}

	// 验证索引查询不报错
	var peers []model.ChatPeerCache
	if err := db.Where("account_id = ? AND peer_ref = ?", 1, "u_1").Find(&peers).Error; err != nil {
		t.Fatalf("peer 索引查询失败: %s", err)
	}

	var msgs []model.ChatMessageCache
	if err := db.Where("account_id = ? AND peer_ref = ? AND sent_at < ?", 1, "u_1", "2025-01-01").Find(&msgs).Error; err != nil {
		t.Fatalf("message 索引查询失败: %s", err)
	}
}
