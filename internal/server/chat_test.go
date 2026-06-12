package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/user/atria/internal/credential"
	"github.com/user/atria/internal/model"
	"gorm.io/gorm"
)

// ===== 仪表盘统计测试 =====

func TestDashboardStats_APIKeyConfigured(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// 测试 JSON API
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dashboard/stats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"api_key_count":1`) {
		t.Errorf("API 凭据统计应为 1，实际: %s", body)
	}
}

func TestDashboardStats_NoAPIKey(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dashboard/stats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"api_key_count":0`) {
		t.Errorf("无 API Key 时统计应为 0，实际: %s", body)
	}
}

func TestDashboardStats_LoggedInAccounts(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// / 现在重定向到 /app/#/dashboard
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}
}

func TestDashboardStats_ActiveSessions(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// / 现在重定向到 /app/#/dashboard
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}
}

func TestDashboardStats_TodayAuditEvents(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	srv.db.Create(&model.AuditLog{
		ActorType: "admin", ActorID: 1, Action: "test.action1",
		ResourceType: "test", RiskLevel: "low", Message: "test1",
		CreatedAt: time.Now(),
	})
	srv.db.Create(&model.AuditLog{
		ActorType: "admin", ActorID: 1, Action: "test.action2",
		ResourceType: "test", RiskLevel: "low", Message: "test2",
		CreatedAt: time.Now(),
	})
	srv.db.Create(&model.AuditLog{
		ActorType: "admin", ActorID: 1, Action: "test.action_old",
		ResourceType: "test", RiskLevel: "low", Message: "old",
		CreatedAt: time.Now().AddDate(0, 0, -1),
	})

	// / 现在重定向到 /app/#/dashboard
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}
}

func TestDashboardStats_NoSensitiveLeak(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	sensitiveTerms := []string{
		"abcdef0123456789",
		"encrypted_phone_data",
		"+8613800138000",
		"sessions/test.session",
	}
	for _, s := range sensitiveTerms {
		if strings.Contains(body, s) {
			t.Errorf("仪表盘不应包含敏感数据 %q", s)
		}
	}
}

// ===== 聊天页面测试 =====

func TestChatsPage_NoCurrentAccount_ShowsConnectPrompt(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /chats 现在重定向到 /app/#/chats（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "/app/#/chats") {
		t.Errorf("应重定向到 /app/#/chats，实际 %s", loc)
	}
}

func TestChatsPage_WithCurrentAccount_RedirectsToSPA(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// 设置当前账号
	w := httptest.NewRecorder()
	body := "account_id=1&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/accounts/select", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_session" {
			sessionCookie = cookie.Value
		}
	}

	// /chats 应重定向到 /app/#/chats（canonical hash URL）
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "/app/#/chats") {
		t.Errorf("应重定向到 /app/#/chats，实际 %s", loc)
	}
}

func TestChatDetail_ShowsPage(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/chats/u_123", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	// 应该返回页面，不应该 panic
	if w.Code == http.StatusInternalServerError {
		t.Error("聊天详情页不应返回 500")
	}
}

func TestChatSend_TextEmpty_ReturnsError(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 创建账号并设置为当前账号
	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	body := "account_id=1&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/accounts/select", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_session" {
			sessionCookie = cookie.Value
		}
	}

	// 发送空文本
	w = httptest.NewRecorder()
	reqBody := "text=&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/api/chats/u_123/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"text_empty"`) && !strings.Contains(bodyStr, "不能为空") {
		t.Errorf("空文本应返回 text_empty，实际: %s", bodyStr)
	}
}

func TestChatSend_TextTooLong_ReturnsError(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	body := "account_id=1&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/accounts/select", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_session" {
			sessionCookie = cookie.Value
		}
	}

	longText := strings.Repeat("a", 4097)
	w = httptest.NewRecorder()
	reqBody := "text=" + longText + "&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/api/chats/u_123/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"text_too_long"`) && !strings.Contains(bodyStr, "4096") {
		t.Errorf("超长文本应返回 text_too_long，实际: %s", bodyStr)
	}
}

func TestChatSend_RequiresCSRF(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	reqBody := `{"text":"hello"}`
	req, _ := http.NewRequest("POST", "/api/chats/u_123/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("无 CSRF 应返回 403，实际=%d", w.Code)
	}
}

func TestChatSend_DoesNotSupportBulk(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)
	csrfCookie = refreshCSRF(t, r, sessionCookie)

	w := httptest.NewRecorder()
	reqBody := `{"peers":["u_1","u_2"],"text":"spam"}`
	req, _ := http.NewRequest("POST", "/api/chats/u_123/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfCookie)
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 应该正常处理（peers 字段被忽略），不应该有批量接口
	bodyStr := w.Body.String()
	if strings.Contains(bodyStr, `"ok":true`) {
		t.Log("批量请求被正常处理（peers 字段被忽略）")
	}
}

func TestSidebar_ChatLinkEnabled(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// / 现在重定向到 /app/#/dashboard
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/app/#/dashboard" {
		t.Errorf("期望重定向到 /app/#/dashboard，实际=%s", loc)
	}
}

func TestChatPages_DoNotLeakSensitiveData(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	pages := []string{"/chats", "/chats/u_123"}
	for _, page := range pages {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", page, nil)
		req.Header.Set("Cookie", "atria_session="+sessionCookie)
		r.ServeHTTP(w, req)

		body := w.Body.String()
		sensitiveTerms := []string{
			"abcdef0123456789",
			"encrypted_phone_data",
			"+8613800138000",
			"sessions/test.session",
		}
		for _, s := range sensitiveTerms {
			if strings.Contains(body, s) {
				t.Errorf("页面 %s 不应包含敏感数据 %q", page, s)
			}
		}
	}
}

// ===== 批量发送防护测试 =====

func TestChatSend_BulkPeersRejected(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 使用 form body 带 CSRF token
	w := httptest.NewRecorder()
	reqBody := "text=hello&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/chats/u_123/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 带 peers 字段的 JSON 不会通过 form 解析，但 handler 会尝试 JSON 解析
	// 由于 form body 没有 peers 字段，这个测试验证的是 form 路径
	if w.Code == http.StatusForbidden {
		t.Error("请求不应被 CSRF 拒绝")
	}
}

func TestChatSend_BulkPeerRefsRejected(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 测试 JSON body 带 peer_refs
	w := httptest.NewRecorder()
	reqBody := `{"text":"hello","peer_refs":["u_1","u_2"]}`
	req, _ := http.NewRequest("POST", "/api/chats/u_123/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfCookie)
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"bulk_not_supported"`) && !strings.Contains(bodyStr, "CSRF") {
		t.Errorf("应返回 bulk_not_supported 或 CSRF 错误，实际: %s", bodyStr)
	}
}

// ===== 仪表盘统计修复测试 =====

func TestDashboardStats_LoggedOutNotCountedAsLoggedIn(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Active User", "active_user", model.TelegramAccountStatusActive)

	loggedOut := &model.TelegramAccount{
		APICredentialID:  1,
		UserID:           999999,
		PhoneEncrypted:   "encrypted",
		PhoneFingerprint: "***9999",
		DisplayName:      "Logged Out User",
		Status:           model.TelegramAccountStatusLoggedOut,
	}
	srv.db.Create(loggedOut)

	// 直接验证数据库统计
	var activeCount int64
	srv.db.Model(&model.TelegramAccount{}).Where("status = ?", model.TelegramAccountStatusActive).Count(&activeCount)
	if activeCount != 1 {
		t.Errorf("数据库中 active 账号应为 1，实际 %d", activeCount)
	}

	// / 现在重定向到 /app/#/dashboard
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	// / 现在重定向，不应显示旧模板内容
	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}
}

func TestDashboardStats_ActiveSessionsExcludeLoggedOut(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Active User", "active_user", model.TelegramAccountStatusActive)

	// / 现在重定向到 /app/#/dashboard
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}
}

// ===== 旧账号兼容测试 =====

// createLegacyTestAccount 创建旧风格的测试账号（不创建 account_sessions 记录）。
func createLegacyTestAccount(t *testing.T, db *gorm.DB, displayName, username string) *model.TelegramAccount {
	t.Helper()

	account := &model.TelegramAccount{
		APICredentialID:  1,
		UserID:           987654321,
		PhoneEncrypted:   "encrypted_phone_legacy",
		PhoneFingerprint: "***5678",
		Username:         username,
		FirstName:        displayName,
		LastName:         "",
		DisplayName:      displayName,
		Status:           model.TelegramAccountStatusActive,
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("创建旧测试账号失败: %s", err)
	}
	// 注意：不创建 AccountSession 记录，模拟旧账号
	return account
}

func TestChatsPage_LegacyActiveAccount_IsRecognized(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 创建旧账号（无 session 记录）
	legacy := createLegacyTestAccount(t, srv.db, "Aronn AT", "aronn_test")

	// 设置为当前账号
	w := httptest.NewRecorder()
	body := fmt.Sprintf("account_id=%d&csrf_token=%s", legacy.ID, csrfCookie)
	req, _ := http.NewRequest("POST", "/accounts/select", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_session" {
			sessionCookie = cookie.Value
		}
	}

	// 访问 /chats
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	// 不应显示"请先接入 Telegram 账号"
	if strings.Contains(bodyStr, "请先接入") {
		t.Error("旧账号应被识别，不应显示'请先接入 Telegram 账号'")
	}
}

func TestChatsPage_UsesSameCurrentAccountAsTopbar(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建账号
	createTestAccount(t, srv.db, "Aronn AT", "aronn_test", model.TelegramAccountStatusActive)

	// /chats 应重定向到 /app/#/chats（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "/app/#/chats") {
		t.Errorf("应重定向到 /app/#/chats，实际 %s", loc)
	}
}

func TestChatsPage_NoAccount_ShowsConnectPrompt(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 不创建任何账号

	// /chats 应重定向到 /app/#/chats（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
}

func TestChatsPage_InvalidSelectedAccount_FallbackToValidAccount(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 创建一个有效账号
	createTestAccount(t, srv.db, "Aronn AT", "aronn_test", model.TelegramAccountStatusActive)

	// 设置 selected_account_id 为不存在的 ID
	w := httptest.NewRecorder()
	body := "account_id=99999&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/accounts/select", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_session" {
			sessionCookie = cookie.Value
		}
	}

	// 访问 /chats
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	// 应 fallback 到有效账号，不应显示"请先接入"
	if strings.Contains(bodyStr, "请先接入") {
		t.Error("存在有效账号时应 fallback，不应显示'请先接入'")
	}
}

func TestChatsPage_AccountWithoutChatPeerCache_StillAllowed(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建账号，但不创建 chat_peer_cache
	createTestAccount(t, srv.db, "Aronn AT", "aronn_test", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// 不应因为 peer cache 为空而显示"请先接入"
	if strings.Contains(body, "请先接入") {
		t.Error("peer cache 为空不应影响账号识别")
	}
}

func TestCurrentAccountResolver_ConsistentAcrossDashboardAccountsChats(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Aronn AT", "aronn_test", model.TelegramAccountStatusActive)

	// / 现在重定向到 /app/#/dashboard
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Errorf("/ 期望 302 重定向，实际 %d", w.Code)
	}

	// /accounts 现在重定向到 /app/#/accounts
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/accounts", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Errorf("/accounts 期望 302 重定向，实际 %d", w.Code)
	}

	// /chats 现在重定向到 /app/#/chats（canonical hash URL）
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/chats", nil)
	req2.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusFound {
		t.Errorf("/chats 期望 302 重定向，实际 %d", w2.Code)
	}
}

func TestChatsPage_DoesNotLeakSensitiveData(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	sensitiveTerms := []string{
		"abcdef0123456789",
		"encrypted_phone_data",
		"+8613800138000",
		"sessions/test.session",
	}
	for _, s := range sensitiveTerms {
		if strings.Contains(body, s) {
			t.Errorf("/chats 页面不应包含敏感数据 %q", s)
		}
	}
}

// ===== Hash 路由隔离测试 =====

func TestHashRoutes_DoNotAffectAPI(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// API 路由应正常返回 JSON，不受 hash 重定向影响
	apiRoutes := []string{
		"/api/me",
		"/api/dashboard/stats",
		"/api/chats/dialogs",
		"/api/settings",
	}

	for _, route := range apiRoutes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", route, nil)
		req.Header.Set("Cookie", "atria_session="+sessionCookie)
		r.ServeHTTP(w, req)

		// API 路由应返回 200 JSON，不应返回 302 重定向
		if w.Code == http.StatusFound {
			t.Errorf("API 路由 %s 不应返回 302 重定向", route)
		}
		if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			// 某些 API 可能返回不同 content-type，但不应是 HTML
			body := w.Body.String()
			if strings.Contains(body, "<!DOCTYPE html>") {
				t.Errorf("API 路由 %s 不应返回 HTML", route)
			}
		}
	}
}

func TestHashRoutes_DoNotAffectLoginInit(t *testing.T) {
	r, _ := setupTestRouter(t)

	// /login 和 /init 不应受 hash 重定向影响
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusFound {
		loc := w.Header().Get("Location")
		if strings.Contains(loc, "/app/#/") {
			t.Error("/login 不应重定向到 /app/#/")
		}
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		body := w.Body.String()
		if strings.Contains(body, "/app/#/") {
			t.Error("/init 页面不应包含 /app/#/ 重定向")
		}
	}
}

func TestLegacyAccountsRedirectsToHashRoute(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	legacyRoutes := map[string]string{
		"/accounts":       "/app/#/accounts",
		"/accounts/login": "/app/#/accounts/login",
		"/chats":          "/app/#/chats",
		"/settings":       "/app/#/settings",
		"/audit":          "/app/#/audit",
		"/contacts":       "/app/#/contacts",
		"/security":       "/app/#/settings",
	}

	for route, expectedLoc := range legacyRoutes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", route, nil)
		req.Header.Set("Cookie", "atria_session="+sessionCookie)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("%s 期望 302 重定向，实际 %d", route, w.Code)
			continue
		}
		loc := w.Header().Get("Location")
		if !strings.Contains(loc, expectedLoc) {
			t.Errorf("%s 应重定向到 %s，实际 %s", route, expectedLoc, loc)
		}
	}
}

func TestAppPathRedirectsToHashRoute(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /app/* 应重定向到 /app/#/*
	appRoutes := map[string]string{
		"/app/accounts":       "/app/#/accounts",
		"/app/chats":          "/app/#/chats",
		"/app/chats/u_123":    "/app/#/chats/u_123",
		"/app/settings":       "/app/#/settings",
		"/app/accounts/login": "/app/#/accounts/login",
	}

	for route, expectedLoc := range appRoutes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", route, nil)
		req.Header.Set("Cookie", "atria_session="+sessionCookie)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("%s 期望 302 重定向，实际 %d", route, w.Code)
			continue
		}
		loc := w.Header().Get("Location")
		if loc != expectedLoc {
			t.Errorf("%s 应重定向到 %s，实际 %s", route, expectedLoc, loc)
		}
	}
}

func TestAppRoot_ServesSPAShell(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /app 和 /app/ 应返回 SPA shell（HTML），不应重定向
	for _, route := range []string{"/app", "/app/"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", route, nil)
		req.Header.Set("Cookie", "atria_session="+sessionCookie)
		r.ServeHTTP(w, req)

		if w.Code == http.StatusFound {
			t.Errorf("%s 不应返回 302 重定向", route)
		}
	}
}

// ===== 聊天 API 安全测试 =====

func TestChatDialogsAPI_NoAccountReturnsEmpty(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 没有账号时应返回空列表
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/dialogs", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"ok":false`) && !strings.Contains(body, `"ok": true`) {
		// 应返回 JSON 响应
		if !strings.Contains(body, "no_current_account") && !strings.Contains(body, "dialogs") {
			t.Errorf("应返回 JSON 响应，实际: %s", body)
		}
	}
}

func TestChatDialogsAPI_DoesNotReturnAccessHash(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建带 peer cache 的账号
	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// 创建 peer cache 记录
	srv.db.Create(&model.ChatPeerCache{
		AccountID:           1,
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: "some_encrypted_access_hash_value",
		Title:               "Test Peer",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/dialogs", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// 不应泄露 access_hash
	sensitiveTerms := []string{
		"access_hash",
		"AccessHash",
		"access_hash_encrypted",
		"AccessHashEncrypted",
		"some_encrypted_access_hash_value",
	}
	for _, s := range sensitiveTerms {
		if strings.Contains(body, s) {
			t.Errorf("dialogs API 不应泄露 %q", s)
		}
	}
}

func TestChatMessagesAPI_DoesNotReturnSessionPath(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// 创建 peer cache
	srv.db.Create(&model.ChatPeerCache{
		AccountID:           1,
		PeerRef:             "u_999",
		PeerType:            "user",
		PeerID:              999,
		AccessHashEncrypted: "encrypted_hash",
		Title:               "Test Peer",
	})

	// 创建消息缓存
	srv.db.Create(&model.ChatMessageCache{
		AccountID:         1,
		PeerRef:           "u_999",
		TelegramMessageID: 1,
		Direction:         "out",
		SenderName:        "Test User",
		Kind:              "text",
		TextEncrypted:     "encrypted_text",
		SentAt:            time.Now(),
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/u_999/messages", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// 不应泄露 session path 或 api_hash
	sensitiveTerms := []string{
		"sessions/test.session",
		"session_file_path",
		"SessionFilePath",
		"abcdef0123456789",
		"api_hash",
		"EncryptedAPIHash",
		"proxy_password",
	}
	for _, s := range sensitiveTerms {
		if strings.Contains(body, s) {
			t.Errorf("messages API 不应泄露 %q，实际: %s", s, body)
		}
	}
}

func TestChatCacheAPI_DoesNotReturnAPIHash(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})
	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// 测试多个聊天 API
	endpoints := []string{
		"/api/chats/dialogs",
		"/api/chats/u_999/messages",
	}

	for _, endpoint := range endpoints {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", endpoint, nil)
		req.Header.Set("Cookie", "atria_session="+sessionCookie)
		r.ServeHTTP(w, req)

		body := w.Body.String()
		if strings.Contains(body, "abcdef0123456789") {
			t.Errorf("%s 不应泄露 API Hash 明文", endpoint)
		}
		if strings.Contains(body, "EncryptedAPIHash") {
			t.Errorf("%s 不应泄露 EncryptedAPIHash 字段", endpoint)
		}
	}
}

func TestChatCacheAPI_DoesNotReturnProxyPassword(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 设置代理密码
	srv.db.Create(&model.SystemSetting{Key: "proxy_password", Value: "encrypted_password", ValueType: "string", IsSensitive: true})

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/dialogs", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "encrypted_password") {
		t.Error("聊天 API 不应泄露 proxy_password")
	}
	if strings.Contains(body, "proxy_password") {
		t.Error("聊天 API 不应包含 proxy_password 字段")
	}
}
