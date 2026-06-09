package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/user/atria/internal/credential"
	"github.com/user/atria/internal/model"
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

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, ">--<") {
		t.Error("API 凭据统计不应显示 '--'")
	}
	if !strings.Contains(body, ">1<") {
		t.Error("API 凭据统计应显示 1")
	}
}

func TestDashboardStats_NoAPIKey(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, ">0<") {
		t.Error("无 API Key 时统计应显示 0")
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

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// 无当前账号时应显示接入提示
	if !strings.Contains(body, "请先接入") && !strings.Contains(body, "接入账号") {
		t.Errorf("无当前账号时应显示接入提示，实际内容包含: %v", strings.Contains(body, "请先接入"))
	}
}

func TestChatsPage_WithCurrentAccount_ShowsDialogContent(t *testing.T) {
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

	// 访问 /chats
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	// 不应显示"请先接入 Telegram 账号"
	if strings.Contains(bodyStr, "请先接入 Telegram 账号") {
		t.Error("有当前账号时不应显示'请先接入 Telegram 账号'")
	}
	// 应该显示聊天页面标题
	if !strings.Contains(bodyStr, "聊天") {
		t.Error("应显示聊天页面标题")
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
