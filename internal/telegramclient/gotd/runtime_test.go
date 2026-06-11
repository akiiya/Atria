package gotd

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/security"
	"github.com/user/atria/internal/telegramclient"

	"gorm.io/gorm"
)

func setupRuntimeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("创建测试数据库失败: %s", err)
	}
	if err := db.AutoMigrate(
		&model.TelegramAccount{},
		&model.AccountSession{},
		&model.APICredential{},
		&model.ChatPeerCache{},
		&model.ChatMessageCache{},
		&model.TelegramUpdateState{},
	); err != nil {
		t.Fatalf("数据库迁移失败: %s", err)
	}
	return db
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

func TestRuntimeManager_StatusStoppedByDefault(t *testing.T) {
	db := setupRuntimeTestDB(t)
	bus := telegramclient.NewEventBus(newTestLogger())
	defer bus.Close()

	mgr := NewRuntimeManager(db, testKey, bus, newTestLogger())

	status := mgr.Status(999)
	if status.State != telegramclient.RuntimeStateStopped {
		t.Errorf("期望 stopped，实际 %s", status.State)
	}
}

func TestRuntimeManager_StartAccount_NoSession(t *testing.T) {
	db := setupRuntimeTestDB(t)
	bus := telegramclient.NewEventBus(newTestLogger())
	defer bus.Close()

	mgr := NewRuntimeManager(db, testKey, bus, newTestLogger())

	// 没有账号时应返回错误
	err := mgr.StartAccount(999)
	if err == nil {
		t.Fatal("没有账号时应返回错误")
	}
}

func TestRuntimeManager_StartAccount_Idempotent(t *testing.T) {
	db := setupRuntimeTestDB(t)
	bus := telegramclient.NewEventBus(newTestLogger())
	defer bus.Close()

	mgr := NewRuntimeManager(db, testKey, bus, newTestLogger())

	// 创建账号和 session
	account := createRuntimeTestAccount(t, db)

	// 第一次启动（会因为无法连接 Telegram 而最终停止，但 StartAccount 本身应成功）
	err := mgr.StartAccount(account.ID)
	if err != nil {
		t.Fatalf("StartAccount 失败: %s", err)
	}

	// 等待一小段时间让 runtime 启动
	time.Sleep(100 * time.Millisecond)

	// 第二次启动应返回 nil（幂等）
	err = mgr.StartAccount(account.ID)
	if err != nil {
		t.Errorf("重复 StartAccount 应返回 nil，实际 %s", err)
	}

	// 清理
	mgr.StopAll()
	time.Sleep(100 * time.Millisecond)
}

func TestRuntimeManager_StopAccount(t *testing.T) {
	db := setupRuntimeTestDB(t)
	bus := telegramclient.NewEventBus(newTestLogger())
	defer bus.Close()

	mgr := NewRuntimeManager(db, testKey, bus, newTestLogger())

	account := createRuntimeTestAccount(t, db)

	mgr.StartAccount(account.ID)
	time.Sleep(100 * time.Millisecond)

	// 停止
	err := mgr.StopAccount(account.ID)
	if err != nil {
		t.Fatalf("StopAccount 失败: %s", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 停止后状态应为 stopped
	status := mgr.Status(account.ID)
	if status.State != telegramclient.RuntimeStateStopped {
		t.Errorf("期望 stopped，实际 %s", status.State)
	}
}

func TestRuntimeManager_SubscribeAndPublish(t *testing.T) {
	bus := telegramclient.NewEventBus(newTestLogger())
	defer bus.Close()

	sink, ch := telegramclient.NewChannelSink(10)
	_, err := bus.Subscribe(1, sink)
	if err != nil {
		t.Fatalf("Subscribe 失败: %s", err)
	}

	// 发布事件
	bus.Publish(1, telegramclient.UpdateEvent{
		EventID:   "test_runtime",
		AccountID: 1,
		Type:      telegramclient.EventMessageNew,
		CreatedAt: time.Now(),
	})

	select {
	case event := <-ch:
		if event.EventID != "test_runtime" {
			t.Errorf("期望 EventID=test_runtime，实际 %s", event.EventID)
		}
	case <-time.After(time.Second):
		t.Fatal("超时")
	}
}

func TestRuntimeManager_NoDuplicateRuntimeForSameAccount(t *testing.T) {
	db := setupRuntimeTestDB(t)
	bus := telegramclient.NewEventBus(newTestLogger())
	defer bus.Close()

	mgr := NewRuntimeManager(db, testKey, bus, newTestLogger())
	account := createRuntimeTestAccount(t, db)

	// 启动两次
	mgr.StartAccount(account.ID)
	time.Sleep(50 * time.Millisecond)
	mgr.StartAccount(account.ID)
	time.Sleep(50 * time.Millisecond)

	// 只停止一次应该就够了
	mgr.StopAccount(account.ID)
	time.Sleep(100 * time.Millisecond)

	status := mgr.Status(account.ID)
	if status.State != telegramclient.RuntimeStateStopped {
		t.Errorf("期望 stopped，实际 %s", status.State)
	}
}

// testKey 是测试用的加密密钥。
var testKey = func() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return key
}()

// createRuntimeTestAccount 创建测试账号。
func createRuntimeTestAccount(t *testing.T, db *gorm.DB) *model.TelegramAccount {
	t.Helper()

	encryptedHash, _, err := security.EncryptAPIHash(testKey, "abcdef0123456789abcdef0123456789")
	if err != nil {
		t.Fatalf("加密 API Hash 失败: %s", err)
	}

	cred := &model.APICredential{
		DisplayName:      "Test API",
		APIID:            12345678,
		EncryptedAPIHash: encryptedHash,
		APIHashHint:      "abcd...6789",
		IsDefault:        true,
		Status:           model.APICredentialStatusEnabled,
	}
	db.Create(cred)

	account := &model.TelegramAccount{
		APICredentialID:  cred.ID,
		UserID:           123456789,
		PhoneEncrypted:   "encrypted_phone",
		PhoneFingerprint: "***1234",
		Username:         "test_user",
		DisplayName:      "Test User",
		Status:           model.TelegramAccountStatusActive,
	}
	db.Create(account)

	session := &model.AccountSession{
		TelegramAccountID:  account.ID,
		SessionFilePath:    "test.session",
		SessionFingerprint: "test_fp",
		EncryptionVersion:  1,
		Status:             "active",
	}
	db.Create(session)

	return account
}
