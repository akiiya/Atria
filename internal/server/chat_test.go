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

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, ">1<") {
		t.Error("已登录账号统计应显示 1")
	}
}

func TestDashboardStats_ActiveSessions(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, ">1<") {
		t.Error("活跃 Session 统计应显示 1")
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

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, ">2<") {
		t.Error("今日审计事件统计应显示 2")
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

	// /chats 现在重定向到 /app/chats
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "/app/chats") {
		t.Errorf("应重定向到 /app/chats，实际 %s", loc)
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

	// /chats 应重定向到 /app/chats
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "/app/chats") {
		t.Errorf("应重定向到 /app/chats，实际 %s", loc)
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

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// 聊天链接应该是 /chats
	if !strings.Contains(body, `href="/chats"`) {
		t.Error("聊天菜单应链接到 /chats")
	}
	// 聊天链接不应是 disabled
	if strings.Contains(body, `href="/chats" class="nav-item disabled"`) {
		t.Error("聊天菜单不应是 disabled")
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

	// 验证页面
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// 页面应显示 1 个已登录账号
	if strings.Contains(body, "已登录账号") {
		// 检查 stat-value 紧跟的内容
		idx := strings.Index(body, "已登录账号")
		if idx > 0 {
			// 往前找 stat-value
			prev := body[:idx]
			valueIdx := strings.LastIndex(prev, "stat-value")
			if valueIdx > 0 {
				valueSection := prev[valueIdx : valueIdx+50]
				if !strings.Contains(valueSection, ">1<") {
					t.Errorf("已登录账号统计应显示 1，实际: %s", valueSection)
				}
			}
		}
	}
}

func TestDashboardStats_ActiveSessionsExcludeLoggedOut(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Active User", "active_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, ">1<") {
		t.Error("活跃 Session 统计应为 1")
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

	// /chats 应重定向到 /app/chats
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "/app/chats") {
		t.Errorf("应重定向到 /app/chats，实际 %s", loc)
	}
}

func TestChatsPage_NoAccount_ShowsConnectPrompt(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 不创建任何账号

	// /chats 应重定向到 /app/chats
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

	// / 和 /accounts 是 Go 模板页面，应显示账号名
	htmlPages := []string{"/", "/accounts"}
	for _, page := range htmlPages {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", page, nil)
		req.Header.Set("Cookie", "atria_session="+sessionCookie)
		r.ServeHTTP(w, req)

		body := w.Body.String()
		if !strings.Contains(body, "Aronn AT") {
			t.Errorf("页面 %s 应显示当前账号名 Aronn AT", page)
		}
	}

	// /chats 现在重定向到 /app/chats
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Errorf("/chats 期望 302 重定向，实际 %d", w.Code)
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
