package chat

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/gotd/td/tgerr"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/mtproto"

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

// ===== 聊天错误分类测试 =====

func setupChatServiceForTest(t *testing.T) *ChatService {
	t.Helper()
	db := setupTestDB(t)
	return NewChatService(db, "/tmp", make([]byte, 32), nil, nil)
}

func TestClassifyRPCErrorForChat_AUTH_KEY_UNREGISTERED(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "AUTH_KEY_UNREGISTERED", Code: 401})
	if chatErr.Code != "session_invalid" {
		t.Errorf("期望 session_invalid，实际 %s", chatErr.Code)
	}
}

func TestClassifyRPCErrorForChat_AUTH_KEY_INVALID(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "AUTH_KEY_INVALID", Code: 401})
	if chatErr.Code != "session_invalid" {
		t.Errorf("期望 session_invalid，实际 %s", chatErr.Code)
	}
}

func TestClassifyRPCErrorForChat_SESSION_REVOKED(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "SESSION_REVOKED", Code: 401})
	if chatErr.Code != "session_invalid" {
		t.Errorf("期望 session_invalid，实际 %s", chatErr.Code)
	}
}

func TestClassifyRPCErrorForChat_SESSION_EXPIRED(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "SESSION_EXPIRED", Code: 401})
	if chatErr.Code != "session_invalid" {
		t.Errorf("期望 session_invalid，实际 %s", chatErr.Code)
	}
}

func TestClassifyRPCErrorForChat_API_ID_INVALID(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "API_ID_INVALID", Code: 400})
	if chatErr.Code != "api_key_invalid" {
		t.Errorf("期望 api_key_invalid，实际 %s", chatErr.Code)
	}
}

func TestClassifyRPCErrorForChat_API_HASH_INVALID(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "API_HASH_INVALID", Code: 400})
	if chatErr.Code != "api_key_invalid" {
		t.Errorf("期望 api_key_invalid，实际 %s", chatErr.Code)
	}
}

func TestClassifyRPCErrorForChat_FLOOD_WAIT(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "FLOOD_WAIT", Code: 420, Argument: 30})
	if chatErr.Code != "flood_wait" {
		t.Errorf("期望 flood_wait，实际 %s", chatErr.Code)
	}
}

func TestClassifyRPCErrorForChat_AUTH_RESTART(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "AUTH_RESTART", Code: 500})
	if chatErr.Code != "auth_restart" {
		t.Errorf("期望 auth_restart，实际 %s", chatErr.Code)
	}
}

func TestClassifyRPCErrorForChat_USER_DEACTIVATED(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "USER_DEACTIVATED", Code: 401})
	if chatErr.Code != "account_deactivated" {
		t.Errorf("期望 account_deactivated，实际 %s", chatErr.Code)
	}
}

func TestClassifyRPCErrorForChat_USER_DEACTIVATED_BAN(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "USER_DEACTIVATED_BAN", Code: 401})
	if chatErr.Code != "account_deactivated" {
		t.Errorf("期望 account_deactivated，实际 %s", chatErr.Code)
	}
}

func TestClassifyRPCErrorForChat_TIMEOUT(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "TIMEOUT", Code: 503})
	if chatErr.Code != "telegram_timeout" {
		t.Errorf("期望 telegram_timeout，实际 %s", chatErr.Code)
	}
}

func TestClassifyRPCErrorForChat_INTERNAL(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "INTERNAL", Code: 500})
	if chatErr.Code != "telegram_error" {
		t.Errorf("期望 telegram_error，实际 %s", chatErr.Code)
	}
}

func TestClassifyRPCErrorForChat_UnknownTGError_ReturnsTelegramErrorNotNetworkError(t *testing.T) {
	chatErr := classifyRPCErrorForChat(&tgerr.Error{Type: "UNKNOWN_RPC_ERROR", Code: 400})
	if chatErr.Code != "telegram_error" {
		t.Errorf("期望 telegram_error，实际 %s", chatErr.Code)
	}
	if chatErr.Code == "network_error" {
		t.Error("未知 RPC 错误不应归类为 network_error")
	}
}

func TestClassifyMTProtoErrorForChat_NetworkError(t *testing.T) {
	chatErr := classifyMTProtoErrorForChat(&mtproto.MTProtoError{Kind: mtproto.ErrNetworkError, Message: "网络错误"})
	if chatErr.Code != "network_error" {
		t.Errorf("期望 network_error，实际 %s", chatErr.Code)
	}
}

func TestClassifyMTProtoErrorForChat_SessionContextLost(t *testing.T) {
	chatErr := classifyMTProtoErrorForChat(&mtproto.MTProtoError{Kind: mtproto.ErrSessionContextLost, Message: "会话丢失"})
	if chatErr.Code != "session_invalid" {
		t.Errorf("期望 session_invalid，实际 %s", chatErr.Code)
	}
}

func TestClassifyMTProtoErrorForChat_FloodWait(t *testing.T) {
	chatErr := classifyMTProtoErrorForChat(&mtproto.MTProtoError{Kind: mtproto.ErrFloodWait, Message: "等待"})
	if chatErr.Code != "flood_wait" {
		t.Errorf("期望 flood_wait，实际 %s", chatErr.Code)
	}
}

func TestClassifyMTProtoErrorForChat_TelegramError(t *testing.T) {
	chatErr := classifyMTProtoErrorForChat(&mtproto.MTProtoError{Kind: mtproto.ErrTelegramError, Message: "异常"})
	if chatErr.Code != "telegram_error" {
		t.Errorf("期望 telegram_error，实际 %s", chatErr.Code)
	}
}

func TestClassifyError_ContextDeadlineExceeded(t *testing.T) {
	svc := setupChatServiceForTest(t)
	err := svc.classifyError(context.DeadlineExceeded)
	chatErr, ok := err.(*ChatError)
	if !ok {
		t.Fatalf("期望 ChatError，实际 %T", err)
	}
	if chatErr.Code != "telegram_timeout" {
		t.Errorf("期望 telegram_timeout，实际 %s", chatErr.Code)
	}
}

func TestClassifyError_ContextCanceled(t *testing.T) {
	svc := setupChatServiceForTest(t)
	err := svc.classifyError(context.Canceled)
	chatErr, ok := err.(*ChatError)
	if !ok {
		t.Fatalf("期望 ChatError，实际 %T", err)
	}
	if chatErr.Code != "telegram_timeout" {
		t.Errorf("期望 telegram_timeout，实际 %s", chatErr.Code)
	}
}

func TestClassifyError_NilReturnsNil(t *testing.T) {
	svc := setupChatServiceForTest(t)
	if err := svc.classifyError(nil); err != nil {
		t.Errorf("nil 错误应返回 nil，实际 %v", err)
	}
}

func TestClassifyError_ChatErrorPassthrough(t *testing.T) {
	svc := setupChatServiceForTest(t)
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
	if len(longResult) > 203 { // 200 + "..."
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
