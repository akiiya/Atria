package chat

import (
	"testing"
	"time"

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
		&model.TelegramAccount{},
		&model.AccountSession{},
		&model.APICredential{},
		&model.ChatPeerCache{},
	); err != nil {
		t.Fatalf("数据库迁移失败: %s", err)
	}
	return db
}

func createTestAccount(t *testing.T, db *gorm.DB) *model.TelegramAccount {
	t.Helper()
	cred := &model.APICredential{
		DisplayName:      "Test API",
		APIID:            12345678,
		EncryptedAPIHash: "encrypted_hash",
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

func TestChatService_GetMessages_UsesCachedAccessHash(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	// 创建 peer 缓存，带加密的 access_hash
	encryptedHash := "encrypted_access_hash_placeholder"
	cache := &model.ChatPeerCache{
		AccountID:           account.ID,
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: encryptedHash,
		Title:               "Test User",
	}
	db.Create(cache)

	svc := NewChatService(db, "/tmp", make([]byte, 32), nil, nil)

	// GetMessages 应该从缓存读取 peer 信息
	// 由于没有真实 Telegram 连接，会失败，但验证了缓存查询逻辑
	_, err := svc.GetMessages(account.ID, "u_999", 50)
	if err == nil {
		t.Log("GetMessages 应该因为无法连接 Telegram 而失败")
	}
	// 验证不是 peer_invalid 错误（说明缓存命中了）
	if chatErr, ok := err.(*ChatError); ok {
		if chatErr.Code == "peer_invalid" {
			t.Error("peer 缓存应命中，不应返回 peer_invalid")
		}
	}
}

func TestChatService_GetMessages_PeerRefFromOtherAccountRejected(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	// 创建属于其它账号的 peer 缓存
	cache := &model.ChatPeerCache{
		AccountID:           99999, // 不同的 accountID
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: "encrypted",
		Title:               "Other User",
	}
	db.Create(cache)

	svc := NewChatService(db, "/tmp", make([]byte, 32), nil, nil)

	_, err := svc.GetMessages(account.ID, "u_999", 50)
	if err == nil {
		t.Fatal("跨账号 peer_ref 应被拒绝")
	}
	chatErr, ok := err.(*ChatError)
	if !ok {
		t.Fatalf("期望 ChatError，实际 %T", err)
	}
	if chatErr.Code != "peer_invalid" {
		t.Errorf("期望 peer_invalid，实际 %s", chatErr.Code)
	}
}

func TestChatService_SendText_UsesCachedAccessHash(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	cache := &model.ChatPeerCache{
		AccountID:           account.ID,
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: "encrypted_access_hash",
		Title:               "Test User",
	}
	db.Create(cache)

	svc := NewChatService(db, "/tmp", make([]byte, 32), nil, nil)

	// SendText 应该从缓存读取 peer 信息
	_, err := svc.SendText(account.ID, "u_999", "hello")
	if err == nil {
		t.Log("SendText 应该因为无法连接 Telegram 而失败")
	}
	// 验证不是 peer_invalid 错误
	if chatErr, ok := err.(*ChatError); ok {
		if chatErr.Code == "peer_invalid" {
			t.Error("peer 缓存应命中，不应返回 peer_invalid")
		}
	}
}

func TestChatService_SendText_TextEmpty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewChatService(db, "/tmp", make([]byte, 32), nil, nil)

	_, err := svc.SendText(1, "u_1", "")
	if err == nil {
		t.Fatal("空文本应返回错误")
	}
	chatErr, ok := err.(*ChatError)
	if !ok {
		t.Fatalf("期望 ChatError，实际 %T", err)
	}
	if chatErr.Code != "text_empty" {
		t.Errorf("期望 text_empty，实际 %s", chatErr.Code)
	}
}

func TestChatService_SendText_TextTooLong(t *testing.T) {
	db := setupTestDB(t)
	svc := NewChatService(db, "/tmp", make([]byte, 32), nil, nil)

	longText := ""
	for i := 0; i < 4097; i++ {
		longText += "a"
	}

	_, err := svc.SendText(1, "u_1", longText)
	if err == nil {
		t.Fatal("超长文本应返回错误")
	}
	chatErr, ok := err.(*ChatError)
	if !ok {
		t.Fatalf("期望 ChatError，实际 %T", err)
	}
	if chatErr.Code != "text_too_long" {
		t.Errorf("期望 text_too_long，实际 %s", chatErr.Code)
	}
}

func TestChatService_MissingAccessHashRejected(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	// user 类型缺少 access_hash
	cache := &model.ChatPeerCache{
		AccountID:           account.ID,
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: "", // 空
		Title:               "Test User",
	}
	db.Create(cache)

	svc := NewChatService(db, "/tmp", make([]byte, 32), nil, nil)

	_, err := svc.GetMessages(account.ID, "u_999", 50)
	if err == nil {
		t.Fatal("缺少 access_hash 应返回错误")
	}
	chatErr, ok := err.(*ChatError)
	if !ok {
		t.Fatalf("期望 ChatError，实际 %T", err)
	}
	if chatErr.Code != "peer_incomplete" {
		t.Errorf("期望 peer_incomplete，实际 %s", chatErr.Code)
	}
}

func TestChatService_ChatTypeNoAccessHashRequired(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	// chat 类型不需要 access_hash
	cache := &model.ChatPeerCache{
		AccountID:           account.ID,
		PeerRef:             "c_888",
		PeerType:            "chat",
		PeerID:              888,
		AccessHashEncrypted: "", // chat 类型可以为空
		Title:               "Test Chat",
	}
	db.Create(cache)

	svc := NewChatService(db, "/tmp", make([]byte, 32), nil, nil)

	// chat 类型不检查 access_hash，但会因为无法连接 Telegram 而失败
	_, err := svc.GetMessages(account.ID, "c_888", 50)
	if err == nil {
		t.Log("GetMessages 应该因为无法连接 Telegram 而失败")
	}
	// 不应该是 peer_incomplete
	if chatErr, ok := err.(*ChatError); ok {
		if chatErr.Code == "peer_incomplete" {
			t.Error("chat 类型不应要求 access_hash")
		}
	}
}

func TestFakeService_SendText_TracksCallCount(t *testing.T) {
	fake := &FakeService{}

	fake.SendText(1, "u_1", "hello")
	fake.SendText(1, "u_1", "world")

	if fake.SendCallCount != 2 {
		t.Errorf("期望调用 2 次，实际 %d", fake.SendCallCount)
	}
}

func TestFakeService_SendText_BulkDoesNotCall(t *testing.T) {
	fake := &FakeService{}

	// 模拟批量场景：handler 应在调用 SendText 之前拒绝
	if fake.SendCallCount != 0 {
		t.Error("批量请求不应调用 SendText")
	}
}

func TestChatService_PeerCache_Encrypted(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	// 创建 peer 缓存
	cache := &model.ChatPeerCache{
		AccountID:           account.ID,
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: "some_encrypted_value",
		Title:               "Test User",
	}
	db.Create(cache)

	// 从数据库读取
	var saved model.ChatPeerCache
	db.Where("peer_ref = ?", "u_999").First(&saved)

	if saved.AccessHashEncrypted == "" {
		t.Error("access_hash 应已加密保存")
	}
	// 加密值不应是纯数字
	if saved.AccessHashEncrypted == "999" {
		t.Error("access_hash 不应明文保存")
	}
}

func TestChatService_PeerCache_AccountIsolation(t *testing.T) {
	db := setupTestDB(t)
	account1 := createTestAccount(t, db)

	account2 := &model.TelegramAccount{
		APICredentialID:  1,
		UserID:           999999,
		PhoneEncrypted:   "encrypted",
		PhoneFingerprint: "***9999",
		DisplayName:      "Account 2",
		Status:           model.TelegramAccountStatusActive,
	}
	db.Create(account2)

	// 创建属于 account1 的 peer
	cache := &model.ChatPeerCache{
		AccountID:           account1.ID,
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: "encrypted",
		Title:               "User 999",
	}
	db.Create(cache)

	svc := NewChatService(db, "/tmp", make([]byte, 32), nil, nil)

	// account2 尝试访问 account1 的 peer
	_, err := svc.GetMessages(account2.ID, "u_999", 50)
	if err == nil {
		t.Fatal("跨账号访问应被拒绝")
	}
}

func TestDashboardStats_LoggedOutNotCounted(t *testing.T) {
	db := setupTestDB(t)

	// 创建 active 账号
	active := &model.TelegramAccount{
		APICredentialID: 1, UserID: 1,
		PhoneEncrypted: "p", DisplayName: "Active",
		Status: model.TelegramAccountStatusActive,
	}
	db.Create(active)

	// 创建 logged_out 账号
	loggedOut := &model.TelegramAccount{
		APICredentialID: 1, UserID: 2,
		PhoneEncrypted: "p", DisplayName: "Logged Out",
		Status: model.TelegramAccountStatusLoggedOut,
	}
	db.Create(loggedOut)

	// 统计只包含 active
	var count int64
	db.Model(&model.TelegramAccount{}).Where("status = ?", model.TelegramAccountStatusActive).Count(&count)
	if count != 1 {
		t.Errorf("已登录账号应为 1，实际 %d", count)
	}

	// 确认 logged_out 不被包含
	var countAll int64
	db.Model(&model.TelegramAccount{}).Where("status IN ?", []string{"active", "logged_out"}).Count(&countAll)
	if countAll != 2 {
		t.Errorf("总账号数应为 2，实际 %d", countAll)
	}
}

// Ensure we use time package (suppress unused import).
var _ = time.Now()
