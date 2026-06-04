package account

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/mtproto"
	"github.com/user/atria/internal/security"

	"gorm.io/gorm"
)

// mockClient 是用于测试的 MTProto 客户端 mock。
type mockClient struct {
	syncProfileFunc  func(ctx context.Context, req mtproto.SyncProfileRequest) (*mtproto.AccountProfile, error)
	checkSessionFunc func(ctx context.Context, req mtproto.CheckSessionRequest) (*mtproto.SessionStatus, error)
	logoutFunc       func(ctx context.Context, req mtproto.LogoutRequest) error
}

func (m *mockClient) StartLogin(ctx context.Context, req mtproto.StartLoginRequest) (*mtproto.LoginStep, error) {
	return nil, nil
}

func (m *mockClient) SubmitCode(ctx context.Context, req mtproto.SubmitCodeRequest) (*mtproto.LoginStep, error) {
	return nil, nil
}

func (m *mockClient) SubmitPassword(ctx context.Context, req mtproto.SubmitPasswordRequest) (*mtproto.LoginStep, error) {
	return nil, nil
}

func (m *mockClient) SyncProfile(ctx context.Context, req mtproto.SyncProfileRequest) (*mtproto.AccountProfile, error) {
	if m.syncProfileFunc != nil {
		return m.syncProfileFunc(ctx, req)
	}
	return &mtproto.AccountProfile{
		UserID:    123456789,
		Phone:     "+8613800138000",
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	}, nil
}

func (m *mockClient) CheckSession(ctx context.Context, req mtproto.CheckSessionRequest) (*mtproto.SessionStatus, error) {
	if m.checkSessionFunc != nil {
		return m.checkSessionFunc(ctx, req)
	}
	return &mtproto.SessionStatus{
		Valid:     true,
		Status:    "active",
		Message:   "Session 有效",
		CheckedAt: time.Now(),
	}, nil
}

func (m *mockClient) Logout(ctx context.Context, req mtproto.LogoutRequest) error {
	if m.logoutFunc != nil {
		return m.logoutFunc(ctx, req)
	}
	return nil
}

func setupTestDB(t *testing.T) (*gorm.DB, []byte) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("创建测试数据库失败: %s", err)
	}

	if err := db.AutoMigrate(&model.Admin{}, &model.APICredential{}, &model.TelegramAccount{}, &model.AccountSession{}, &model.AccountSyncSnapshot{}, &model.AuditLog{}); err != nil {
		t.Fatalf("数据库迁移失败: %s", err)
	}

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	return db, key
}

// createTestCredential 创建测试用 API 凭据。
func createTestCredential(t *testing.T, db *gorm.DB, key []byte, status model.APICredentialStatus) *model.APICredential {
	t.Helper()
	encrypted, _, err := security.EncryptAPIHash(key, "abcdef0123456789abcdef0123456789")
	if err != nil {
		t.Fatalf("加密 API Hash 失败: %s", err)
	}
	cred := &model.APICredential{
		DisplayName:        "Test",
		APIID:              12345,
		EncryptedAPIHash:   encrypted,
		APIHashFingerprint: "test1234",
		Status:             status,
		RiskPolicy:         model.RiskPolicyDisabled,
	}
	if err := db.Create(cred).Error; err != nil {
		t.Fatalf("创建凭据失败: %s", err)
	}
	return cred
}

// createTestAccount 创建测试用账号。
func createTestAccount(t *testing.T, svc *Service, ctx context.Context, credID uint) *model.TelegramAccount {
	t.Helper()
	profile := &mtproto.AccountProfile{
		UserID:    123456789,
		Phone:     "+8613800138000",
		Username:  "testuser",
		FirstName: "Test",
	}
	account, err := svc.CompleteLogin(ctx, CompleteLoginInput{
		APICredentialID: credID,
		Profile:         profile,
		SessionData:     []byte("test session data"),
		ActorID:         1,
	})
	if err != nil {
		t.Fatalf("创建账号失败: %s", err)
	}
	return account
}

func TestService_CompleteLogin_CreateNew(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	client := &mockClient{}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()

	profile := &mtproto.AccountProfile{
		UserID:    123456789,
		Phone:     "+8613800138000",
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	}

	account, err := svc.CompleteLogin(ctx, CompleteLoginInput{
		APICredentialID: 1,
		Profile:         profile,
		SessionData:     []byte("test session data"),
		ActorID:         1,
	})
	if err != nil {
		t.Fatalf("创建账号失败: %s", err)
	}

	if account.UserID != 123456789 {
		t.Errorf("UserID 不匹配，实际=%d", account.UserID)
	}
	if account.Username != "testuser" {
		t.Errorf("Username 不匹配，实际=%s", account.Username)
	}
}

func TestService_CompleteLogin_UpdateExisting(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	client := &mockClient{}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()

	profile := &mtproto.AccountProfile{
		UserID:    123456789,
		Phone:     "+8613800138000",
		Username:  "testuser",
		FirstName: "Test",
	}

	account1, err := svc.CompleteLogin(ctx, CompleteLoginInput{
		APICredentialID: 1,
		Profile:         profile,
		SessionData:     []byte("session v1"),
		ActorID:         1,
	})
	if err != nil {
		t.Fatalf("第一次登录失败: %s", err)
	}

	profile2 := &mtproto.AccountProfile{
		UserID:    123456789,
		Phone:     "+8613800138000",
		Username:  "testuser_updated",
		FirstName: "Test",
	}

	account2, err := svc.CompleteLogin(ctx, CompleteLoginInput{
		APICredentialID: 1,
		Profile:         profile2,
		SessionData:     []byte("session v2"),
		ActorID:         1,
	})
	if err != nil {
		t.Fatalf("第二次登录失败: %s", err)
	}

	if account1.ID != account2.ID {
		t.Errorf("应该是同一账号，实际 ID1=%d, ID2=%d", account1.ID, account2.ID)
	}

	if account2.Username != "testuser_updated" {
		t.Errorf("Username 应更新，实际=%s", account2.Username)
	}
}

func TestService_CompleteLogin_PhoneEncrypted(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	client := &mockClient{}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()

	phone := "+8613800138000"
	profile := &mtproto.AccountProfile{
		UserID: 123456789,
		Phone:  phone,
	}

	account, err := svc.CompleteLogin(ctx, CompleteLoginInput{
		APICredentialID: 1,
		Profile:         profile,
		SessionData:     []byte("test session data"),
		ActorID:         1,
	})
	if err != nil {
		t.Fatalf("创建账号失败: %s", err)
	}

	if account.PhoneEncrypted == phone {
		t.Error("手机号不应以明文保存")
	}
	if account.PhoneEncrypted == "" {
		t.Error("手机号加密后不应为空")
	}
}

func TestService_CompleteLogin_SessionPathNoPhone(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	client := &mockClient{}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()

	profile := &mtproto.AccountProfile{
		UserID: 123456789,
		Phone:  "+8613800138000",
	}

	account, err := svc.CompleteLogin(ctx, CompleteLoginInput{
		APICredentialID: 1,
		Profile:         profile,
		SessionData:     []byte("test session data"),
		ActorID:         1,
	})
	if err != nil {
		t.Fatalf("创建账号失败: %s", err)
	}

	if account.Session != nil && strings.Contains(account.Session.SessionFilePath, "+8613800138000") {
		t.Error("Session 文件路径不应包含手机号")
	}
}

func TestService_SyncProfile_Success(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusEnabled)

	client := &mockClient{
		syncProfileFunc: func(ctx context.Context, req mtproto.SyncProfileRequest) (*mtproto.AccountProfile, error) {
			return &mtproto.AccountProfile{
				UserID:    123456789,
				Phone:     "+8613800138000",
				Username:  "updated_user",
				FirstName: "Updated",
				LastName:  "User",
			}, nil
		},
	}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	result, err := svc.SyncProfile(ctx, SyncProfileInput{
		AccountID: account.ID,
		ActorID:   1,
	})
	if err != nil {
		t.Fatalf("同步资料失败: %s", err)
	}

	if result.Account.Username != "updated_user" {
		t.Errorf("Username 应更新，实际=%s", result.Account.Username)
	}
	if result.Account.LastSyncAt == nil {
		t.Error("LastSyncAt 应被设置")
	}
}

func TestService_SyncProfile_DisabledCredential(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusDisabled)

	client := &mockClient{}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	_, err := svc.SyncProfile(ctx, SyncProfileInput{
		AccountID: account.ID,
		ActorID:   1,
	})
	if err == nil {
		t.Error("凭据禁用时同步应失败")
	}
}

func TestService_CheckSession_Success(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusEnabled)

	client := &mockClient{
		checkSessionFunc: func(ctx context.Context, req mtproto.CheckSessionRequest) (*mtproto.SessionStatus, error) {
			return &mtproto.SessionStatus{
				Valid:     true,
				Status:    "active",
				Message:   "Session 有效",
				CheckedAt: time.Now(),
			}, nil
		},
	}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	result, err := svc.CheckSession(ctx, CheckSessionInput{
		AccountID: account.ID,
		ActorID:   1,
	})
	if err != nil {
		t.Fatalf("检测 Session 失败: %s", err)
	}

	if !result.Valid {
		t.Error("Session 应该有效")
	}
	if result.SessionStatus != "active" {
		t.Errorf("状态应为 active，实际=%s", result.SessionStatus)
	}
}

func TestService_CheckSession_Invalid(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusEnabled)

	client := &mockClient{
		checkSessionFunc: func(ctx context.Context, req mtproto.CheckSessionRequest) (*mtproto.SessionStatus, error) {
			return &mtproto.SessionStatus{
				Valid:     false,
				Status:    "invalid",
				Message:   "Session 已失效",
				CheckedAt: time.Now(),
			}, nil
		},
	}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	result, err := svc.CheckSession(ctx, CheckSessionInput{
		AccountID: account.ID,
		ActorID:   1,
	})
	if err != nil {
		t.Fatalf("检测 Session 失败: %s", err)
	}

	if result.Valid {
		t.Error("Session 应该无效")
	}
	if result.SessionStatus != "invalid" {
		t.Errorf("状态应为 invalid，实际=%s", result.SessionStatus)
	}
}

// ===== Phase 4.3: RemoteLogout 和 DeleteLocalSession 测试 =====

func TestService_RemoteLogout_Success(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusEnabled)

	logoutCalled := false
	client := &mockClient{
		logoutFunc: func(ctx context.Context, req mtproto.LogoutRequest) error {
			logoutCalled = true
			return nil
		},
	}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	// 记录 Session 文件路径
	sessionPath := account.Session.SessionFilePath

	err := svc.RemoteLogout(ctx, RemoteLogoutInput{
		AccountID: account.ID,
		ActorID:   1,
	})
	if err != nil {
		t.Fatalf("远端 Logout 失败: %s", err)
	}

	// 检查 Logout 被调用
	if !logoutCalled {
		t.Error("mtproto.Client.Logout 应被调用")
	}

	// 检查本地 Session 文件被删除
	if sessionStore.Exists(sessionPath) {
		t.Error("本地 Session 文件应被删除")
	}

	// 检查账号状态
	updated, _ := svc.GetAccount(ctx, account.ID)
	if updated.Status != model.TelegramAccountStatusLoggedOut {
		t.Errorf("账号状态应为 logged_out，实际=%s", updated.Status)
	}

	// 检查 Session 状态
	if updated.Session != nil && updated.Session.Status != "deleted" {
		t.Errorf("Session 状态应为 deleted，实际=%s", updated.Session.Status)
	}
}

func TestService_RemoteLogout_FloodWait(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusEnabled)

	client := &mockClient{
		logoutFunc: func(ctx context.Context, req mtproto.LogoutRequest) error {
			return &mtproto.FloodWaitError{
				Wait:    60 * time.Second,
				Message: "请等待 60 秒后重试",
			}
		},
	}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	// 记录 Session 文件路径
	sessionPath := account.Session.SessionFilePath

	err := svc.RemoteLogout(ctx, RemoteLogoutInput{
		AccountID: account.ID,
		ActorID:   1,
	})
	if err == nil {
		t.Error("FLOOD_WAIT 时应返回错误")
	}

	// 检查本地 Session 文件未被删除
	if !sessionStore.Exists(sessionPath) {
		t.Error("FLOOD_WAIT 时不应删除本地 Session 文件")
	}
}

func TestService_RemoteLogout_DisabledCredential(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusDisabled)

	client := &mockClient{}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	err := svc.RemoteLogout(ctx, RemoteLogoutInput{
		AccountID: account.ID,
		ActorID:   1,
	})
	if err == nil {
		t.Error("凭据禁用时远端 Logout 应失败")
	}
}

func TestService_DeleteLocalSession_Success(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusEnabled)

	logoutCalled := false
	client := &mockClient{
		logoutFunc: func(ctx context.Context, req mtproto.LogoutRequest) error {
			logoutCalled = true
			return nil
		},
	}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	// 记录 Session 文件路径
	sessionPath := account.Session.SessionFilePath

	err := svc.DeleteLocalSession(ctx, DeleteLocalSessionInput{
		AccountID: account.ID,
		ActorID:   1,
	})
	if err != nil {
		t.Fatalf("本地删除 Session 失败: %s", err)
	}

	// 检查 Logout 未被调用
	if logoutCalled {
		t.Error("本地删除 Session 不应调用 mtproto.Client.Logout")
	}

	// 检查本地 Session 文件被删除
	if sessionStore.Exists(sessionPath) {
		t.Error("本地 Session 文件应被删除")
	}

	// 检查账号状态
	updated, _ := svc.GetAccount(ctx, account.ID)
	if updated.Status != model.TelegramAccountStatusLoggedOut {
		t.Errorf("账号状态应为 logged_out，实际=%s", updated.Status)
	}

	// 检查 Session 状态
	if updated.Session != nil && updated.Session.Status != "deleted" {
		t.Errorf("Session 状态应为 deleted，实际=%s", updated.Session.Status)
	}
}

func TestService_DeleteLocalSession_AccountStillExists(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusEnabled)

	client := &mockClient{}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	svc.DeleteLocalSession(ctx, DeleteLocalSessionInput{
		AccountID: account.ID,
		ActorID:   1,
	})

	// 账号基础记录应仍存在
	_, err := svc.GetAccount(ctx, account.ID)
	if err != nil {
		t.Error("本地删除 Session 后账号基础记录应仍存在")
	}
}

// ===== 审计日志测试 =====

func TestService_AuditLog_RemoteLogout(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusEnabled)

	client := &mockClient{}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	svc.RemoteLogout(ctx, RemoteLogoutInput{
		AccountID: account.ID,
		ActorID:   1,
	})

	var count int64
	db.Model(&model.AuditLog{}).Where("action = ?", "account.remote_logout").Count(&count)
	if count == 0 {
		t.Error("应写入 account.remote_logout 审计日志")
	}
}

func TestService_AuditLog_DeleteLocalSession(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusEnabled)

	client := &mockClient{}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	svc.DeleteLocalSession(ctx, DeleteLocalSessionInput{
		AccountID: account.ID,
		ActorID:   1,
	})

	var count int64
	db.Model(&model.AuditLog{}).Where("action = ?", "account.local_session_deleted").Count(&count)
	if count == 0 {
		t.Error("应写入 account.local_session_deleted 审计日志")
	}
}

func TestService_AuditLog_SyncProfile(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusEnabled)

	client := &mockClient{}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	svc.SyncProfile(ctx, SyncProfileInput{
		AccountID: account.ID,
		ActorID:   1,
	})

	var count int64
	db.Model(&model.AuditLog{}).Where("action = ?", "account.profile_synced").Count(&count)
	if count == 0 {
		t.Error("应写入 account.profile_synced 审计日志")
	}
}

func TestService_AuditLog_CheckSession(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	cred := createTestCredential(t, db, key, model.APICredentialStatusEnabled)

	client := &mockClient{}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()
	account := createTestAccount(t, svc, ctx, cred.ID)

	svc.CheckSession(ctx, CheckSessionInput{
		AccountID: account.ID,
		ActorID:   1,
	})

	var count int64
	db.Model(&model.AuditLog{}).Where("action = ?", "account.session_checked").Count(&count)
	if count == 0 {
		t.Error("应写入 account.session_checked 审计日志")
	}
}

func TestService_AuditLog_NoPlaintextPhone(t *testing.T) {
	db, key := setupTestDB(t)
	dir := t.TempDir()
	sessionStore := mtproto.NewFileSessionStore(dir, key)
	client := &mockClient{}
	svc := NewService(db, key, sessionStore, client)
	ctx := context.Background()

	phone := "+8613800138000"
	profile := &mtproto.AccountProfile{
		UserID:   123456789,
		Phone:    phone,
		Username: "testuser",
	}

	svc.CompleteLogin(ctx, CompleteLoginInput{
		APICredentialID: 1,
		Profile:         profile,
		SessionData:     []byte("test session data"),
		ActorID:         1,
	})

	var logs []model.AuditLog
	db.Where("action = ?", "account.created").Find(&logs)
	for _, log := range logs {
		if strings.Contains(log.MetadataJSON, phone) {
			t.Error("审计日志不应包含明文手机号")
		}
	}
}
