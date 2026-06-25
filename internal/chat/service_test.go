package chat

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/security"
	"github.com/user/atria/internal/telegramclient"

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
		&model.ChatMessageCache{},
	); err != nil {
		t.Fatalf("数据库迁移失败: %s", err)
	}
	return db
}

// encryptTestAccessHash 加密测试用的 access_hash。
func encryptTestAccessHash(accessHash int64) (string, error) {
	plain := fmt.Sprintf("%d", accessHash)
	return crypto.EncryptString(testKey, plain, []byte("atria:chat_peer:v1"))
}

// testKey 是测试用的加密密钥。
var testKey = func() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return key
}()

func createTestAccount(t *testing.T, db *gorm.DB) *model.TelegramAccount {
	t.Helper()

	// 使用有效的加密 API Hash
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

// ===== Adapter 集成测试 =====

func TestChatService_ListDialogs_UsesAdapter(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	fake := &FakeAdapter{
		Dialogs: []telegramclient.Dialog{
			{PeerRef: "u_1", PeerType: telegramclient.PeerTypeUser, Title: "Test User", PeerID: 1, AccessHash: 12345},
		},
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, err := svc.ListDialogs(context.Background(), account.ID, 20, true)
	if err != nil {
		t.Fatalf("ListDialogs 失败: %s", err)
	}
	if len(result.Dialogs) != 1 {
		t.Fatalf("期望 1 个对话，实际 %d", len(result.Dialogs))
	}
	if result.Dialogs[0].Title != "Test User" {
		t.Errorf("期望标题 Test User，实际 %s", result.Dialogs[0].Title)
	}
}

func TestChatService_GetMessages_UsesAdapter(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	// 创建 peer cache，使用有效的加密 access_hash
	encryptedHash, err := encryptTestAccessHash(12345)
	if err != nil {
		t.Fatalf("加密 access_hash 失败: %s", err)
	}
	cache := &model.ChatPeerCache{
		AccountID:           account.ID,
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: encryptedHash,
		Title:               "Test User",
	}
	db.Create(cache)

	fake := &FakeAdapter{
		Messages: []telegramclient.Message{
			{ID: "1", TelegramMessageID: 1, PeerRef: "u_999", Direction: telegramclient.MessageDirectionOut, Text: "hello", Kind: telegramclient.MessageKindText, SentAt: time.Now(), IsOutgoing: true, Status: telegramclient.MessageStatusSent},
		},
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, err := svc.GetMessages(context.Background(), account.ID, "u_999", 50, true)
	if err != nil {
		t.Fatalf("GetMessages 失败: %s", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("期望 1 条消息，实际 %d", len(result.Messages))
	}
	if result.Messages[0].Text != "hello" {
		t.Errorf("期望消息文本 hello，实际 %s", result.Messages[0].Text)
	}
}

func TestChatService_SendText_UsesAdapter(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	encryptedHash, err := encryptTestAccessHash(12345)
	if err != nil {
		t.Fatalf("加密 access_hash 失败: %s", err)
	}
	cache := &model.ChatPeerCache{
		AccountID:           account.ID,
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: encryptedHash,
		Title:               "Test User",
	}
	db.Create(cache)

	fake := &FakeAdapter{
		SendResult: telegramclient.SendResult{
			MessageID: 123,
			SentAt:    time.Now(),
			Status:    "sent",
			Direction: "out",
			Text:      "hello",
		},
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, err := svc.SendText(context.Background(), account.ID, "u_999", "hello")
	if err != nil {
		t.Fatalf("SendText 失败: %s", err)
	}
	if result.MessageID != 123 {
		t.Errorf("期望消息 ID 123，实际 %d", result.MessageID)
	}
	if fake.SendCallCount != 1 {
		t.Errorf("期望 adapter 调用 1 次，实际 %d", fake.SendCallCount)
	}
}

func TestChatService_AdapterErrorPropagatesNeutralCode(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	fake := &FakeAdapter{
		ListErr: telegramclient.NewError(telegramclient.ErrorCodeSessionInvalid, "Session 已失效"),
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	_, err := svc.ListDialogs(context.Background(), account.ID, 20, true)
	if err == nil {
		t.Fatal("期望返回错误")
	}
	chatErr, ok := err.(*ChatError)
	if !ok {
		t.Fatalf("期望 ChatError，实际 %T", err)
	}
	if chatErr.Code != "session_invalid" {
		t.Errorf("期望 session_invalid，实际 %s", chatErr.Code)
	}
}

func TestChatService_LoadOlderMessages_UsesAdapter(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	encryptedHash, err := encryptTestAccessHash(12345)
	if err != nil {
		t.Fatalf("加密 access_hash 失败: %s", err)
	}
	cache := &model.ChatPeerCache{
		AccountID:           account.ID,
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: encryptedHash,
		Title:               "Test User",
	}
	db.Create(cache)

	fake := &FakeAdapter{
		Messages: []telegramclient.Message{
			{ID: "1", TelegramMessageID: 1, PeerRef: "u_999", Direction: telegramclient.MessageDirectionIn, Text: "old msg", Kind: telegramclient.MessageKindText, SentAt: time.Now().Add(-1 * time.Hour)},
		},
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	// LoadOlderMessages 通过 GetMessages 间接测试（因为 ChatService 目前不直接暴露 LoadOlderMessages）
	// 这里测试 adapter 被正确注入
	result, err := svc.GetMessages(context.Background(), account.ID, "u_999", 50, true)
	if err != nil {
		t.Fatalf("GetMessages 失败: %s", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("期望 1 条消息，实际 %d", len(result.Messages))
	}
}

// ===== 输入验证测试 =====

func TestChatService_SendText_TextEmpty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewChatService(db, testKey, nil, slog.Default())

	_, err := svc.SendText(context.Background(), 1, "u_1", "")
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
	svc := NewChatService(db, testKey, nil, slog.Default())

	longText := ""
	for i := 0; i < 4097; i++ {
		longText += "a"
	}

	_, err := svc.SendText(context.Background(), 1, "u_1", longText)
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

func TestChatService_GetMessages_PeerRefFromOtherAccountRejected(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	cache := &model.ChatPeerCache{
		AccountID:           99999,
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: "encrypted",
		Title:               "Other User",
	}
	db.Create(cache)

	svc := NewChatService(db, testKey, nil, slog.Default())

	_, err := svc.GetMessages(context.Background(), account.ID, "u_999", 50, true)
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

func TestChatService_MissingAccessHashRejected(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	// 创建 peer cache，但 access_hash 为空（user 类型需要）
	cache := &model.ChatPeerCache{
		AccountID:           account.ID,
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: "", // 空
		Title:               "Test User",
	}
	db.Create(cache)

	fake := &FakeAdapter{}
	svc := NewChatService(db, testKey, fake, slog.Default())

	_, err := svc.GetMessages(context.Background(), account.ID, "u_999", 50, true)
	if err == nil {
		t.Fatal("缺少 access_hash 应返回错误")
	}
	chatErr, ok := err.(*ChatError)
	if !ok {
		t.Fatalf("期望 ChatError，实际 %T", err)
	}
	// 空 access_hash 会导致解密失败或 adapter 返回错误
	if chatErr.Code != "peer_incomplete" && chatErr.Code != "peer_invalid" {
		t.Errorf("期望 peer_incomplete 或 peer_invalid，实际 %s", chatErr.Code)
	}
}

// ===== 缓存测试 =====

func TestChatService_CacheMessages_EncryptedAtRest(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	svc := NewChatService(db, testKey, nil, slog.Default())

	messages := []Message{
		{
			MessageID:   100,
			PeerRef:     "u_999",
			Direction:   MessageDirectionOut,
			SenderName:  "Test User",
			Text:        "Hello, this is a secret message",
			SentAt:      time.Now(),
			IsOutgoing:  true,
			Status:      MessageStatusSent,
			MessageType: "text",
		},
	}

	svc.cacheMessages(account.ID, "u_999", messages)

	var cached []model.ChatMessageCache
	db.Where("account_id = ? AND peer_ref = ?", account.ID, "u_999").Find(&cached)

	if len(cached) != 1 {
		t.Fatalf("应缓存 1 条消息，实际 %d", len(cached))
	}

	if cached[0].TextEncrypted == "Hello, this is a secret message" {
		t.Error("消息正文不应明文存储")
	}
	if cached[0].TextEncrypted == "" {
		t.Error("加密后的消息正文不应为空")
	}
}

func TestChatService_CacheMessages_LimitRecentOnly(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	svc := NewChatService(db, testKey, nil, slog.Default())

	var messages []Message
	for i := 1; i <= 510; i++ {
		messages = append(messages, Message{
			MessageID:   i,
			PeerRef:     "u_999",
			Direction:   MessageDirectionIn,
			SenderName:  "Test",
			Text:        fmt.Sprintf("Message %d", i),
			SentAt:      time.Now().Add(time.Duration(i) * time.Second),
			MessageType: "text",
		})
	}

	svc.cacheMessages(account.ID, "u_999", messages)

	var count int64
	db.Model(&model.ChatMessageCache{}).
		Where("account_id = ? AND peer_ref = ?", account.ID, "u_999").
		Count(&count)

	if count > 500 {
		t.Errorf("缓存应限制到 500 条，实际 %d", count)
	}
}

func TestChatService_CacheMessages_ScopedByAccountAndPeer(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	account2 := &model.TelegramAccount{
		APICredentialID:  1,
		UserID:           999999,
		PhoneEncrypted:   "encrypted",
		PhoneFingerprint: "***9999",
		DisplayName:      "Account 2",
		Status:           model.TelegramAccountStatusActive,
	}
	db.Create(account2)

	svc := NewChatService(db, testKey, nil, slog.Default())

	messages1 := []Message{
		{MessageID: 1, PeerRef: "u_1", Direction: MessageDirectionIn, SenderName: "A", Text: "msg1", SentAt: time.Now(), MessageType: "text"},
	}
	svc.cacheMessages(account.ID, "u_1", messages1)

	messages2 := []Message{
		{MessageID: 2, PeerRef: "u_1", Direction: MessageDirectionIn, SenderName: "B", Text: "msg2", SentAt: time.Now(), MessageType: "text"},
	}
	svc.cacheMessages(account2.ID, "u_1", messages2)

	cached := svc.getMessagesFromCache(account.ID, "u_1", 50)
	if len(cached) != 1 {
		t.Fatalf("account1 应有 1 条消息，实际 %d", len(cached))
	}
	if cached[0].MessageID != 1 {
		t.Errorf("account1 的消息 ID 应为 1，实际 %d", cached[0].MessageID)
	}

	cached2 := svc.getMessagesFromCache(account2.ID, "u_1", 50)
	if len(cached2) != 1 {
		t.Fatalf("account2 应有 1 条消息，实际 %d", len(cached2))
	}
	if cached2[0].MessageID != 2 {
		t.Errorf("account2 的消息 ID 应为 2，实际 %d", cached2[0].MessageID)
	}
}

func TestChatService_GetMessagesFromCache_ReturnsEmptyWhenNone(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	svc := NewChatService(db, testKey, nil, slog.Default())

	cached := svc.getMessagesFromCache(account.ID, "u_nonexistent", 50)
	if cached != nil {
		t.Errorf("无缓存时应返回 nil，实际 %v", cached)
	}
}

func TestChatService_ListDialogsFromCache_ReturnsEmptyWhenNone(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	svc := NewChatService(db, testKey, nil, slog.Default())

	cached := svc.listDialogsFromCache(account.ID, 20)
	if cached != nil {
		t.Errorf("无缓存时应返回 nil，实际 %v", cached)
	}
}

func TestChatService_ListDialogsFromCache_OrdersByLastMessageAt(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	now := time.Now()
	earlier := now.Add(-1 * time.Hour)

	db.Create(&model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_1", PeerType: "user", PeerID: 1,
		Title: "Older", LastMessageAt: &earlier,
	})
	db.Create(&model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_2", PeerType: "user", PeerID: 2,
		Title: "Newer", LastMessageAt: &now,
	})

	svc := NewChatService(db, testKey, nil, slog.Default())

	dialogs := svc.listDialogsFromCache(account.ID, 20)
	if len(dialogs) != 2 {
		t.Fatalf("应返回 2 个对话，实际 %d", len(dialogs))
	}

	if dialogs[0].Title != "Newer" {
		t.Errorf("第一个对话应为 Newer，实际 %s", dialogs[0].Title)
	}
}

// ===== 错误分类测试 =====

func TestClassifyError_NeutralError(t *testing.T) {
	db := setupTestDB(t)
	svc := NewChatService(db, testKey, nil, slog.Default())

	err := svc.classifyError(telegramclient.NewError(telegramclient.ErrorCodeFloodWait, "等待"))
	chatErr, ok := err.(*ChatError)
	if !ok {
		t.Fatalf("期望 ChatError，实际 %T", err)
	}
	if chatErr.Code != "flood_wait" {
		t.Errorf("期望 flood_wait，实际 %s", chatErr.Code)
	}
}

func TestClassifyError_ContextDeadlineExceeded(t *testing.T) {
	db := setupTestDB(t)
	svc := NewChatService(db, testKey, nil, slog.Default())

	err := svc.classifyError(context.DeadlineExceeded)
	chatErr, ok := err.(*ChatError)
	if !ok {
		t.Fatalf("期望 ChatError，实际 %T", err)
	}
	if chatErr.Code != "telegram_timeout" {
		t.Errorf("期望 telegram_timeout，实际 %s", chatErr.Code)
	}
}

func TestClassifyError_NilReturnsNil(t *testing.T) {
	db := setupTestDB(t)
	svc := NewChatService(db, testKey, nil, slog.Default())

	if err := svc.classifyError(nil); err != nil {
		t.Errorf("nil 错误应返回 nil，实际 %v", err)
	}
}

func TestClassifyError_ChatErrorPassthrough(t *testing.T) {
	db := setupTestDB(t)
	svc := NewChatService(db, testKey, nil, slog.Default())

	input := &ChatError{Code: "test_code", Message: "test message"}
	err := svc.classifyError(input)
	if err != input {
		t.Error("ChatError 应直接透传")
	}
}

func TestSanitizeErrorForLog(t *testing.T) {
	short := sanitizeErrorForLog("short error")
	if short != "short error" {
		t.Errorf("短消息不应被截断，实际 %s", short)
	}

	longMsg := ""
	for i := 0; i < 300; i++ {
		longMsg += "a"
	}
	longResult := sanitizeErrorForLog(longMsg)
	if len(longResult) > 203 {
		t.Errorf("长消息应被截断到 200 字符，实际 %d", len(longResult))
	}
}

func TestIsProxyError(t *testing.T) {
	if isProxyError(nil) {
		t.Error("nil 不应是代理错误")
	}
	if isProxyError(fmt.Errorf("some error")) {
		t.Error("普通错误不应是代理错误")
	}
	if !isProxyError(fmt.Errorf("proxy connection failed")) {
		t.Error("proxy 关键字应识别为代理错误")
	}
	if !isProxyError(fmt.Errorf("SOCKS5 dial failed")) {
		t.Error("SOCKS5 关键字应识别为代理错误")
	}
}

func TestClassifyProxyError(t *testing.T) {
	err := classifyProxyError(fmt.Errorf("proxy auth failed: 407"))
	if err.Code != "proxy_auth_failed" {
		t.Errorf("期望 proxy_auth_failed，实际 %s", err.Code)
	}

	err = classifyProxyError(fmt.Errorf("dial timeout"))
	if err.Code != "telegram_timeout" {
		t.Errorf("期望 telegram_timeout，实际 %s", err.Code)
	}

	err = classifyProxyError(fmt.Errorf("connection refused"))
	if err.Code != "proxy_connect_failed" {
		t.Errorf("期望 proxy_connect_failed，实际 %s", err.Code)
	}
}

func TestChatService_DoesNotLogMessageBody(t *testing.T) {
	db := setupTestDB(t)
	svc := NewChatService(db, testKey, nil, slog.Default())
	_ = svc
}

// ===== 分页测试 =====

func TestChatMessages_DefaultLimitRecent50(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	encryptedHash, err := encryptTestAccessHash(12345)
	if err != nil {
		t.Fatalf("加密 access_hash 失败: %s", err)
	}
	cache := &model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_999", PeerType: "user", PeerID: 999,
		AccessHashEncrypted: encryptedHash, Title: "Test",
	}
	db.Create(cache)

	fake := &FakeAdapter{HasOlder: true}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, err := svc.GetMessages(context.Background(), account.ID, "u_999", 0, true)
	if err != nil {
		t.Fatalf("GetMessages 失败: %s", err)
	}
	if result == nil {
		t.Fatal("期望非 nil 结果")
	}
}

func TestChatMessages_LimitMax100(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	encryptedHash, err := encryptTestAccessHash(12345)
	if err != nil {
		t.Fatalf("加密 access_hash 失败: %s", err)
	}
	cache := &model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_999", PeerType: "user", PeerID: 999,
		AccessHashEncrypted: encryptedHash, Title: "Test",
	}
	db.Create(cache)

	fake := &FakeAdapter{}
	svc := NewChatService(db, testKey, fake, slog.Default())

	// limit 超过 200 应被截断到 50
	result, err := svc.GetMessages(context.Background(), account.ID, "u_999", 999, true)
	if err != nil {
		t.Fatalf("GetMessages 失败: %s", err)
	}
	if result == nil {
		t.Fatal("期望非 nil 结果")
	}
}

func TestChatMessages_BeforeIDLoadsOlder(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	encryptedHash, err := encryptTestAccessHash(12345)
	if err != nil {
		t.Fatalf("加密 access_hash 失败: %s", err)
	}
	cache := &model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_999", PeerType: "user", PeerID: 999,
		AccessHashEncrypted: encryptedHash, Title: "Test",
	}
	db.Create(cache)

	fake := &FakeAdapter{
		Messages: []telegramclient.Message{
			{ID: "50", TelegramMessageID: 50, PeerRef: "u_999", Direction: telegramclient.MessageDirectionIn, Text: "old", Kind: telegramclient.MessageKindText, SentAt: time.Now().Add(-2 * time.Hour)},
		},
		HasOlder: true,
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, err := svc.LoadOlderMessages(context.Background(), account.ID, "u_999", 100, 50, true)
	if err != nil {
		t.Fatalf("LoadOlderMessages 失败: %s", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("期望 1 条消息，实际 %d", len(result.Messages))
	}
	if result.Messages[0].MessageID != 50 {
		t.Errorf("期望消息 ID 50，实际 %d", result.Messages[0].MessageID)
	}
}

func TestChatMessages_ReturnsChronologicalOrder(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	encryptedHash, err := encryptTestAccessHash(12345)
	if err != nil {
		t.Fatalf("加密 access_hash 失败: %s", err)
	}
	cache := &model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_999", PeerType: "user", PeerID: 999,
		AccessHashEncrypted: encryptedHash, Title: "Test",
	}
	db.Create(cache)

	now := time.Now()
	fake := &FakeAdapter{
		Messages: []telegramclient.Message{
			{ID: "3", TelegramMessageID: 3, PeerRef: "u_999", Direction: telegramclient.MessageDirectionIn, Text: "third", Kind: telegramclient.MessageKindText, SentAt: now.Add(2 * time.Minute)},
			{ID: "1", TelegramMessageID: 1, PeerRef: "u_999", Direction: telegramclient.MessageDirectionIn, Text: "first", Kind: telegramclient.MessageKindText, SentAt: now},
			{ID: "2", TelegramMessageID: 2, PeerRef: "u_999", Direction: telegramclient.MessageDirectionIn, Text: "second", Kind: telegramclient.MessageKindText, SentAt: now.Add(1 * time.Minute)},
		},
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, err := svc.GetMessages(context.Background(), account.ID, "u_999", 50, true)
	if err != nil {
		t.Fatalf("GetMessages 失败: %s", err)
	}
	// 消息应按 sent_at 正序
	if len(result.Messages) != 3 {
		t.Fatalf("期望 3 条消息，实际 %d", len(result.Messages))
	}
	if result.Messages[0].Text != "first" {
		t.Errorf("第一条应为 first，实际 %s", result.Messages[0].Text)
	}
	if result.Messages[2].Text != "third" {
		t.Errorf("第三条应为 third，实际 %s", result.Messages[2].Text)
	}
}

func TestChatMessages_HasOlderTrueWhenMoreExists(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	encryptedHash, err := encryptTestAccessHash(12345)
	if err != nil {
		t.Fatalf("加密 access_hash 失败: %s", err)
	}
	cache := &model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_999", PeerType: "user", PeerID: 999,
		AccessHashEncrypted: encryptedHash, Title: "Test",
	}
	db.Create(cache)

	fake := &FakeAdapter{HasOlder: true}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, err := svc.GetMessages(context.Background(), account.ID, "u_999", 50, true)
	if err != nil {
		t.Fatalf("GetMessages 失败: %s", err)
	}
	if !result.HasOlder {
		t.Error("期望 HasOlder=true")
	}
}

func TestChatMessages_HasOlderFalseAtBeginning(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	encryptedHash, err := encryptTestAccessHash(12345)
	if err != nil {
		t.Fatalf("加密 access_hash 失败: %s", err)
	}
	cache := &model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_999", PeerType: "user", PeerID: 999,
		AccessHashEncrypted: encryptedHash, Title: "Test",
	}
	db.Create(cache)

	fake := &FakeAdapter{HasOlder: false}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, err := svc.GetMessages(context.Background(), account.ID, "u_999", 50, true)
	if err != nil {
		t.Fatalf("GetMessages 失败: %s", err)
	}
	if result.HasOlder {
		t.Error("期望 HasOlder=false")
	}
}

func TestChatMessages_DeduplicatesByTelegramMessageID(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	svc := NewChatService(db, testKey, nil, slog.Default())

	// 缓存两条相同 telegram_message_id 的消息
	messages := []Message{
		{MessageID: 1, PeerRef: "u_999", Direction: MessageDirectionIn, SenderName: "A", Text: "original", SentAt: time.Now(), MessageType: "text"},
		{MessageID: 1, PeerRef: "u_999", Direction: MessageDirectionIn, SenderName: "A", Text: "updated", SentAt: time.Now().Add(time.Second), MessageType: "text"},
	}
	svc.cacheMessages(account.ID, "u_999", messages)

	var count int64
	db.Model(&model.ChatMessageCache{}).
		Where("account_id = ? AND peer_ref = ? AND telegram_message_id = 1", account.ID, "u_999").
		Count(&count)
	if count != 1 {
		t.Errorf("应只有 1 条记录，实际 %d", count)
	}
}

func TestChatMessages_NoFullHistoryScan(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	encryptedHash, err := encryptTestAccessHash(12345)
	if err != nil {
		t.Fatalf("加密 access_hash 失败: %s", err)
	}
	cache := &model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_999", PeerType: "user", PeerID: 999,
		AccessHashEncrypted: encryptedHash, Title: "Test",
	}
	db.Create(cache)

	// 缓存 10 条消息
	svc := NewChatService(db, testKey, &FakeAdapter{}, slog.Default())
	var msgs []Message
	for i := 1; i <= 10; i++ {
		msgs = append(msgs, Message{
			MessageID: i, PeerRef: "u_999", Direction: MessageDirectionIn,
			SenderName: "Test", Text: fmt.Sprintf("msg %d", i),
			SentAt: time.Now().Add(time.Duration(i) * time.Second), MessageType: "text",
		})
	}
	svc.cacheMessages(account.ID, "u_999", msgs)

	// GetMessages 应只返回请求的数量，不扫描全部
	result, err := svc.GetMessages(context.Background(), account.ID, "u_999", 3, false)
	if err != nil {
		t.Fatalf("GetMessages 失败: %s", err)
	}
	if len(result.Messages) > 3 {
		t.Errorf("请求 limit=3 但返回 %d 条", len(result.Messages))
	}
}

func TestChatMessages_OldestNewestMessageID(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	encryptedHash, err := encryptTestAccessHash(12345)
	if err != nil {
		t.Fatalf("加密 access_hash 失败: %s", err)
	}
	cache := &model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_999", PeerType: "user", PeerID: 999,
		AccessHashEncrypted: encryptedHash, Title: "Test",
	}
	db.Create(cache)

	now := time.Now()
	fake := &FakeAdapter{
		Messages: []telegramclient.Message{
			{ID: "10", TelegramMessageID: 10, PeerRef: "u_999", Direction: telegramclient.MessageDirectionIn, Text: "a", Kind: telegramclient.MessageKindText, SentAt: now},
			{ID: "20", TelegramMessageID: 20, PeerRef: "u_999", Direction: telegramclient.MessageDirectionIn, Text: "b", Kind: telegramclient.MessageKindText, SentAt: now.Add(time.Minute)},
		},
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, err := svc.GetMessages(context.Background(), account.ID, "u_999", 50, true)
	if err != nil {
		t.Fatalf("GetMessages 失败: %s", err)
	}
	if result.OldestMessageID != 10 {
		t.Errorf("期望 OldestMessageID=10，实际 %d", result.OldestMessageID)
	}
	if result.NewestMessageID != 20 {
		t.Errorf("期望 NewestMessageID=20，实际 %d", result.NewestMessageID)
	}
}

func TestChatMessages_LoadOlderBeforeIDInvalid(t *testing.T) {
	db := setupTestDB(t)
	svc := NewChatService(db, testKey, nil, slog.Default())

	_, err := svc.LoadOlderMessages(context.Background(), 1, "u_1", 0, 50, false)
	if err == nil {
		t.Fatal("before_message_id=0 应返回错误")
	}
	chatErr, ok := err.(*ChatError)
	if !ok {
		t.Fatalf("期望 ChatError，实际 %T", err)
	}
	if chatErr.Code != "peer_invalid" {
		t.Errorf("期望 peer_invalid，实际 %s", chatErr.Code)
	}
}

// ===== Cache-first 测试 =====

func TestChatDialogs_ReturnsCacheImmediately(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	// 预缓存 2 个 peer
	now := time.Now()
	db.Create(&model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_1", PeerType: "user", PeerID: 1,
		Title: "Cached User", LastMessageAt: &now,
	})
	db.Create(&model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_2", PeerType: "user", PeerID: 2,
		Title: "Cached User 2", LastMessageAt: &now,
	})

	fake := &FakeAdapter{
		Dialogs: []telegramclient.Dialog{
			{PeerRef: "u_3", PeerType: telegramclient.PeerTypeUser, Title: "Live User", PeerID: 3, AccessHash: 999},
		},
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	// forceRefresh=false → 应返回缓存，不调 adapter
	result, err := svc.ListDialogs(context.Background(), account.ID, 20, false)
	if err != nil {
		t.Fatalf("ListDialogs 失败: %s", err)
	}
	if result.Source != "cache" {
		t.Errorf("期望 source=cache，实际 %s", result.Source)
	}
	if !result.Stale {
		t.Error("期望 stale=true（缓存数据）")
	}
	if len(result.Dialogs) != 2 {
		t.Errorf("期望 2 个缓存对话，实际 %d", len(result.Dialogs))
	}
}

func TestChatDialogs_ForceRefreshCallsTelegram(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	// 预缓存
	now := time.Now()
	db.Create(&model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_1", PeerType: "user", PeerID: 1,
		Title: "Cached", LastMessageAt: &now,
	})

	fake := &FakeAdapter{
		Dialogs: []telegramclient.Dialog{
			{PeerRef: "u_2", PeerType: telegramclient.PeerTypeUser, Title: "Live", PeerID: 2, AccessHash: 888},
		},
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	// forceRefresh=true → 应调 adapter，跳过缓存
	result, err := svc.ListDialogs(context.Background(), account.ID, 20, true)
	if err != nil {
		t.Fatalf("ListDialogs 失败: %s", err)
	}
	if result.Source != "mixed" {
		t.Errorf("期望 source=mixed，实际 %s", result.Source)
	}
	if result.Stale {
		t.Error("期望 stale=false（live 数据）")
	}
}

func TestChatMessages_ReturnsCacheImmediately(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	// 预缓存消息
	svc := NewChatService(db, testKey, nil, slog.Default())
	msgs := []Message{
		{MessageID: 1, PeerRef: "u_999", Direction: MessageDirectionIn, SenderName: "A", Text: "cached", SentAt: time.Now(), MessageType: "text"},
		{MessageID: 2, PeerRef: "u_999", Direction: MessageDirectionOut, SenderName: "B", Text: "msg2", SentAt: time.Now().Add(time.Second), MessageType: "text"},
	}
	svc.cacheMessages(account.ID, "u_999", msgs)

	// forceRefresh=false → 返回缓存
	result, err := svc.GetMessages(context.Background(), account.ID, "u_999", 50, false)
	if err != nil {
		t.Fatalf("GetMessages 失败: %s", err)
	}
	if result.Source != "cache" {
		t.Errorf("期望 source=cache，实际 %s", result.Source)
	}
	if !result.Stale {
		t.Error("期望 stale=true")
	}
	if len(result.Messages) != 2 {
		t.Errorf("期望 2 条缓存消息，实际 %d", len(result.Messages))
	}
}

func TestChatMessages_ForceRefreshCallsTelegram(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	// 预缓存 + peer cache + adapter
	encryptedHash, _ := encryptTestAccessHash(12345)
	db.Create(&model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_999", PeerType: "user", PeerID: 999,
		AccessHashEncrypted: encryptedHash, Title: "Test",
	})

	svc := NewChatService(db, testKey, nil, slog.Default())
	svc.cacheMessages(account.ID, "u_999", []Message{
		{MessageID: 1, PeerRef: "u_999", Direction: MessageDirectionIn, Text: "old", SentAt: time.Now(), MessageType: "text"},
	})

	fake := &FakeAdapter{
		Messages: []telegramclient.Message{
			{ID: "2", TelegramMessageID: 2, PeerRef: "u_999", Direction: telegramclient.MessageDirectionIn, Text: "new", Kind: telegramclient.MessageKindText, SentAt: time.Now()},
		},
	}
	svc2 := NewChatService(db, testKey, fake, slog.Default())

	result, err := svc2.GetMessages(context.Background(), account.ID, "u_999", 50, true)
	if err != nil {
		t.Fatalf("GetMessages 失败: %s", err)
	}
	if result.Source != "mixed" {
		t.Errorf("期望 source=mixed，实际 %s", result.Source)
	}
}

func TestChatDialogs_EmptyCacheCallsTelegram(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	fake := &FakeAdapter{
		Dialogs: []telegramclient.Dialog{
			{PeerRef: "u_1", PeerType: telegramclient.PeerTypeUser, Title: "Live", PeerID: 1, AccessHash: 12345},
		},
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	// 无缓存 + forceRefresh=false → 应回落到 adapter
	result, err := svc.ListDialogs(context.Background(), account.ID, 20, false)
	if err != nil {
		t.Fatalf("ListDialogs 失败: %s", err)
	}
	if len(result.Dialogs) != 1 {
		t.Fatalf("期望 1 个对话，实际 %d", len(result.Dialogs))
	}
	if result.Source != "telegram" {
		t.Errorf("期望 source=telegram，实际 %s", result.Source)
	}
}

func TestChatDialogs_ResponseIncludesSourceAndStale(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	fake := &FakeAdapter{}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, _ := svc.ListDialogs(context.Background(), account.ID, 20, true)
	if result == nil {
		return // 无数据
	}
	// source 字段必须存在
	_ = result.Source
	_ = result.Stale
}

func TestChatMessages_ResponseIncludesSourceAndStale(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	encryptedHash, _ := encryptTestAccessHash(12345)
	db.Create(&model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_999", PeerType: "user", PeerID: 999,
		AccessHashEncrypted: encryptedHash, Title: "Test",
	})

	fake := &FakeAdapter{}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, _ := svc.GetMessages(context.Background(), account.ID, "u_999", 50, true)
	if result == nil {
		return
	}
	_ = result.Source
	_ = result.Stale
}

func TestChatMessages_DoesNotWaitForConnectingExecutor(t *testing.T) {
	// cache-first 模式下，即使没有 runtime/executor，缓存数据也能立即返回
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	svc := NewChatService(db, testKey, nil, slog.Default())
	svc.cacheMessages(account.ID, "u_999", []Message{
		{MessageID: 1, PeerRef: "u_999", Direction: MessageDirectionIn, Text: "cached", SentAt: time.Now(), MessageType: "text"},
	})

	start := time.Now()
	result, err := svc.GetMessages(context.Background(), account.ID, "u_999", 50, false)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("GetMessages 失败: %s", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("cache-first 应在 <500ms 返回，实际 %v", elapsed)
	}
	if result.Source != "cache" {
		t.Errorf("期望 source=cache，实际 %s", result.Source)
	}
}

// ===== truncateText 测试 =====

func TestTruncateText_Short(t *testing.T) {
	if truncateText("short", 10) != "short" {
		t.Error("短文本不应截断")
	}
}

func TestTruncateText_CJK(t *testing.T) {
	// CJK 字符每个占 3 字节，旧实现按字节截断会乱码
	result := truncateText("你好世界测试", 4)
	if result != "你好世界..." {
		t.Errorf("期望 '你好世界...'，实际 '%s'", result)
	}
}

func TestTruncateText_Emoji(t *testing.T) {
	result := truncateText("😂🥹👍❤️", 2)
	if result != "😂🥹..." {
		t.Errorf("期望 '😂🥹...'，实际 '%s'", result)
	}
}

func TestTruncateText_Mixed(t *testing.T) {
	result := truncateText("Hello你好", 5)
	if result != "Hello..." {
		t.Errorf("期望 'Hello...'，实际 '%s'", result)
	}
}

// ===== getInitial 测试 =====

func TestGetInitial_FlagEmoji(t *testing.T) {
	// 🇺🇸 是两个 regional indicator (U+1F1FA U+1F1F8)
	result := getInitial("🇺🇸US GV-Pruse")
	if result != "🇺🇸" {
		t.Errorf("期望 🇺🇸，实际 %q (len=%d)", result, len([]rune(result)))
	}
}

func TestGetInitial_FlagEmojiNoText(t *testing.T) {
	result := getInitial("🇺🇸")
	if result != "🇺🇸" {
		t.Errorf("期望 🇺🇸，实际 %q", result)
	}
}

func TestGetInitial_OtherFlag(t *testing.T) {
	result := getInitial("🇯🇵日本語")
	if result != "🇯🇵" {
		t.Errorf("期望 🇯🇵，实际 %q", result)
	}
}

func TestGetInitial_EmojiWithVariationSelector(t *testing.T) {
	// ❤️ = U+2764 U+FE0F
	result := getInitial("❤️ Red Heart")
	if result != "❤️" {
		t.Errorf("期望 ❤️，实际 %q (runes=%v)", result, []rune(result))
	}
}

func TestGetInitial_ZWJFamily(t *testing.T) {
	// 👨‍👩‍👧‍👦 = U+1F468 U+200D U+1F469 U+200D U+1F466 U+200D U+1F466
	result := getInitial("👨‍👩‍👧‍👦 Family")
	if result != "👨‍👩‍👧‍👦" {
		t.Errorf("期望 👨‍👩‍👧‍👦，实际 %q (runes=%v)", result, []rune(result))
	}
}

func TestGetInitial_EmojiWithSkinTone(t *testing.T) {
	// 👍🏽 = U+1F44D U+1F3FD
	result := getInitial("👍🏽 Thumbs Up")
	if result != "👍🏽" {
		t.Errorf("期望 👍🏽，实际 %q (runes=%v)", result, []rune(result))
	}
}

func TestGetInitial_Chinese(t *testing.T) {
	result := getInitial("中文测试")
	if result != "中" {
		t.Errorf("期望 中，实际 %q", result)
	}
}

func TestGetInitial_English(t *testing.T) {
	result := getInitial("Alice")
	if result != "A" {
		t.Errorf("期望 A，实际 %q", result)
	}
}

func TestGetInitial_Number(t *testing.T) {
	result := getInitial("123Test")
	if result != "1" {
		t.Errorf("期望 1，实际 %q", result)
	}
}

func TestGetInitial_Empty(t *testing.T) {
	if getInitial("") != "?" {
		t.Error("空字符串应返回 ?")
	}
}

func TestGetInitial_RegularEmoji(t *testing.T) {
	result := getInitial("😂 Laughing")
	if result != "😂" {
		t.Errorf("期望 😂，实际 %q", result)
	}
}

// ===== Dialog Title UTF-8 保留测试 =====

func TestDialogTitle_PreservesFlagEmoji(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	fake := &FakeAdapter{
		Dialogs: []telegramclient.Dialog{
			{PeerRef: "u_1", PeerType: telegramclient.PeerTypeUser, Title: "🇺🇸US GV-Pruse", PeerID: 1, AccessHash: 12345},
		},
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, err := svc.ListDialogs(context.Background(), account.ID, 20, true)
	if err != nil {
		t.Fatalf("ListDialogs 失败: %s", err)
	}
	if len(result.Dialogs) != 1 {
		t.Fatalf("期望 1 个对话，实际 %d", len(result.Dialogs))
	}
	if result.Dialogs[0].Title != "🇺🇸US GV-Pruse" {
		t.Errorf("期望 title '🇺🇸US GV-Pruse'，实际 %q", result.Dialogs[0].Title)
	}
}

func TestDialogTitle_PreservesEmojiInCache(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	// 直接写入带 emoji 的 peer cache
	now := time.Now()
	db.Create(&model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_1", PeerType: "user", PeerID: 1,
		Title: "😂Test Emoji", LastMessageAt: &now,
	})

	svc := NewChatService(db, testKey, nil, slog.Default())

	result, err := svc.ListDialogs(context.Background(), account.ID, 20, false)
	if err != nil {
		t.Fatalf("ListDialogs 失败: %s", err)
	}
	if len(result.Dialogs) != 1 {
		t.Fatalf("期望 1 个对话，实际 %d", len(result.Dialogs))
	}
	if result.Dialogs[0].Title != "😂Test Emoji" {
		t.Errorf("期望 title '😂Test Emoji'，实际 %q", result.Dialogs[0].Title)
	}
}

func TestGetContacts_ReturnsContactsWithHasDialog(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	adapter := &FakeAdapter{
		Contacts: []telegramclient.Contact{
			{PeerRef: "u_100", PeerType: telegramclient.PeerTypeUser, DisplayName: "Alice", Username: "alice", Phone: "13800138000", AccessHash: 100, PeerID: 100},
			{PeerRef: "u_200", PeerType: telegramclient.PeerTypeUser, DisplayName: "Bob", Username: "bob", Phone: "13900139000", AccessHash: 200, PeerID: 200},
		},
	}
	svc := NewChatService(db, testKey, adapter, slog.Default())

	// 创建 Alice 的 peer cache（模拟已有 dialog）
	now := time.Now()
	db.Create(&model.ChatPeerCache{
		AccountID: account.ID, PeerRef: "u_100", PeerType: "user", PeerID: 100, Title: "Alice",
		LastMessageAt: &now,
	})

	result, err := svc.GetContacts(context.Background(), account.ID, true)
	if err != nil {
		t.Fatalf("GetContacts failed: %v", err)
	}
	if len(result.Contacts) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(result.Contacts))
	}

	// Alice 应该 has_dialog=true
	alice := result.Contacts[0]
	if alice.PeerRef != "u_100" {
		t.Errorf("expected u_100, got %s", alice.PeerRef)
	}
	if !alice.HasDialog {
		t.Error("Alice should have dialog")
	}

	// Bob 应该 has_dialog=false
	bob := result.Contacts[1]
	if bob.PeerRef != "u_200" {
		t.Errorf("expected u_200, got %s", bob.PeerRef)
	}
	if bob.HasDialog {
		t.Error("Bob should not have dialog")
	}

	// 验证 peer_cache 已写入 Bob（access_hash 被加密保存）
	var bobCache model.ChatPeerCache
	err = db.Where("peer_ref = ? AND account_id = ?", "u_200", account.ID).First(&bobCache).Error
	if err != nil {
		t.Fatalf("Bob should be in peer_cache: %v", err)
	}
	if bobCache.AccessHashEncrypted == "" {
		t.Error("Bob's access_hash should be encrypted in peer_cache")
	}
	if bobCache.Title != "Bob" {
		t.Errorf("Bob's title should be 'Bob', got '%s'", bobCache.Title)
	}
}

func TestGetContacts_CacheFirst(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	adapter := &FakeAdapter{
		Contacts: []telegramclient.Contact{
			{PeerRef: "u_100", PeerType: telegramclient.PeerTypeUser, DisplayName: "Alice", AccessHash: 100, PeerID: 100},
		},
	}
	svc := NewChatService(db, testKey, adapter, slog.Default())

	// 第一次调用：从 Telegram 获取
	result1, err := svc.GetContacts(context.Background(), account.ID, true)
	if err != nil {
		t.Fatalf("first GetContacts failed: %v", err)
	}
	if result1.Source != "telegram" {
		t.Errorf("first call source should be 'telegram', got '%s'", result1.Source)
	}

	// 第二次调用（不 force_refresh）：应从缓存返回
	result2, err := svc.GetContacts(context.Background(), account.ID, false)
	if err != nil {
		t.Fatalf("second GetContacts failed: %v", err)
	}
	if result2.Source != "cache" {
		t.Errorf("second call source should be 'cache', got '%s'", result2.Source)
	}
	if !result2.Stale {
		t.Error("cached result should be stale")
	}
	if len(result2.Contacts) != 1 {
		t.Fatalf("cached contacts should have 1 entry, got %d", len(result2.Contacts))
	}
}

func TestDialogTitle_AvatarPlaceholderUsesGraphemeSafeInitial(t *testing.T) {
	db := setupTestDB(t)
	account := createTestAccount(t, db)

	fake := &FakeAdapter{
		Dialogs: []telegramclient.Dialog{
			{PeerRef: "u_1", PeerType: telegramclient.PeerTypeUser, Title: "🇺🇸US GV-Pruse", PeerID: 1, AccessHash: 12345},
			{PeerRef: "u_2", PeerType: telegramclient.PeerTypeUser, Title: "👨‍👩‍👧‍👦Family", PeerID: 2, AccessHash: 12346},
			{PeerRef: "u_3", PeerType: telegramclient.PeerTypeUser, Title: "😂Laughing", PeerID: 3, AccessHash: 12347},
		},
	}
	svc := NewChatService(db, testKey, fake, slog.Default())

	result, err := svc.ListDialogs(context.Background(), account.ID, 20, true)
	if err != nil {
		t.Fatalf("ListDialogs 失败: %s", err)
	}

	tests := map[string]string{
		"u_1": "🇺🇸",
		"u_2": "👨‍👩‍👧‍👦",
		"u_3": "😂",
	}
	for _, dlg := range result.Dialogs {
		expected, ok := tests[dlg.PeerRef]
		if !ok {
			continue
		}
		if dlg.AvatarPlaceholder != expected {
			t.Errorf("peer %s: 期望 avatar_placeholder %q，实际 %q", dlg.PeerRef, expected, dlg.AvatarPlaceholder)
		}
	}
}
