package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/user/atria/internal/credential"
	"github.com/user/atria/internal/model"
)

// ===== SPA Shell 路由测试 =====

func TestAppShell_RendersVueRoot(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/app", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际 %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `id="app"`) {
		t.Error("应包含 Vue root div")
	}
	if !strings.Contains(body, "csrf-token") {
		t.Error("应包含 CSRF token meta")
	}
}

func TestAppShell_FallbackRoute_Dashboard(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /app/dashboard 应重定向到 /app/#/dashboard（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/app/dashboard", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/app/#/dashboard" {
		t.Errorf("应重定向到 /app/#/dashboard，实际 %s", loc)
	}
}

func TestAppShell_FallbackRoute_Chats(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /app/chats 应重定向到 /app/#/chats（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/app/chats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/app/#/chats" {
		t.Errorf("应重定向到 /app/#/chats，实际 %s", loc)
	}
}

func TestAppShell_FallbackRoute_ChatDetail(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /app/chats/test_peer 应重定向到 /app/#/chats/test_peer（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/app/chats/test_peer", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/app/#/chats/test_peer" {
		t.Errorf("应重定向到 /app/#/chats/test_peer，实际 %s", loc)
	}
}

func TestAppShell_FallbackRoute_Accounts(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /app/accounts 应重定向到 /app/#/accounts（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/app/accounts", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/app/#/accounts" {
		t.Errorf("应重定向到 /app/#/accounts，实际 %s", loc)
	}
}

func TestAppShell_FallbackRoute_AccountDetail(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /app/accounts/1 应重定向到 /app/#/accounts/1（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/app/accounts/1", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/app/#/accounts/1" {
		t.Errorf("应重定向到 /app/#/accounts/1，实际 %s", loc)
	}
}

func TestAppShell_FallbackRoute_Settings(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /app/settings 应重定向到 /app/#/settings（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/app/settings", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/app/#/settings" {
		t.Errorf("应重定向到 /app/#/settings，实际 %s", loc)
	}
}

func TestAppShell_DoesNotCaptureAPI(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// API 应返回 JSON，不是 Vue shell
	if strings.Contains(body, `id="app"`) {
		t.Error("/api/me 不应返回 Vue shell")
	}
	if !strings.Contains(body, `"ok"`) {
		t.Error("/api/me 应返回 JSON")
	}
}

func TestAppShell_DoesNotCaptureHealthz(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, `id="app"`) {
		t.Error("/healthz 不应返回 Vue shell")
	}
	if !strings.Contains(body, `"ok"`) {
		t.Error("/healthz 应返回 JSON")
	}
}

// ===== 旧路由重定向测试 =====

func TestLegacyRoutesRedirectToApp(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	tests := []struct {
		path     string
		expected string
	}{
		{"/chats", "/app/#/chats"},
		{"/chats/test_peer", "/app/#/chats/test_peer"},
		{"/accounts", "/app/#/accounts"},
		{"/accounts/1", "/app/#/accounts/1"},
		{"/accounts/login", "/app/#/accounts/login"},
		{"/settings", "/app/#/settings"},
		{"/audit", "/app/#/audit"},
		{"/contacts", "/app/#/contacts"},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", tt.path, nil)
		req.Header.Set("Cookie", "atria_session="+sessionCookie)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("%s: 期望 302，实际 %d", tt.path, w.Code)
			continue
		}
		loc := w.Header().Get("Location")
		if loc != tt.expected {
			t.Errorf("%s: 期望重定向到 %s，实际 %s", tt.path, tt.expected, loc)
		}
	}
}

// ===== JSON API 测试 =====

func TestAPI_Me_ReturnsCurrentAccount(t *testing.T) {
	r, srv := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Aronn AT", "aronn_test", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"ok":true`) && !strings.Contains(body, `"ok": true`) {
		t.Errorf("应返回 ok:true，实际: %s", body)
	}
}

func TestAPI_Me_DoesNotLeakSensitiveData(t *testing.T) {
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
	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	sensitive := []string{
		"abcdef0123456789",
		"encrypted_phone_data",
		"+8613800138000",
		"sessions/test.session",
		"phone_code_hash",
	}
	for _, s := range sensitive {
		if strings.Contains(body, s) {
			t.Errorf("/api/me 不应包含 %q", s)
		}
	}
}

func TestAPI_DashboardStats_ReturnsJSON(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dashboard/stats", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"api_key_count"`) {
		t.Errorf("应包含 api_key_count，实际: %s", body)
	}
	if !strings.Contains(body, `"account_count"`) {
		t.Errorf("应包含 account_count，实际: %s", body)
	}
	if !strings.Contains(body, `"session_count"`) {
		t.Errorf("应包含 session_count，实际: %s", body)
	}
	if !strings.Contains(body, `"audit_today"`) {
		t.Errorf("应包含 audit_today，实际: %s", body)
	}
}

func TestAPI_Accounts_ReturnsSafeList(t *testing.T) {
	r, srv := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/accounts", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"accounts"`) {
		t.Errorf("应包含 accounts，实际: %s", body)
	}
	sensitive := []string{"encrypted_phone_data", "+8613800138000"}
	for _, s := range sensitive {
		if strings.Contains(body, s) {
			t.Errorf("/api/accounts 不应包含 %q", s)
		}
	}
}

func TestAPI_AccountDetail_ReturnsSafeData(t *testing.T) {
	r, srv := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	acc := createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/accounts/"+fmt.Sprintf("%d", acc.ID), nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"display_name"`) {
		t.Errorf("应包含 display_name，实际: %s", body)
	}
	sensitive := []string{"encrypted_phone_data", "+8613800138000"}
	for _, s := range sensitive {
		if strings.Contains(body, s) {
			t.Errorf("/api/accounts/:id 不应包含 %q", s)
		}
	}
}

func TestAPI_Settings_ReturnsSafeData(t *testing.T) {
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
	req, _ := http.NewRequest("GET", "/api/settings", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"ok"`) {
		t.Errorf("应返回 JSON，实际: %s", body)
	}
	if strings.Contains(body, "abcdef0123456789") {
		t.Error("/api/settings 不应返回 api_hash 明文")
	}
}

func TestAPI_Audit_ReturnsList(t *testing.T) {
	r, srv := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建审计日志
	srv.db.Create(&model.AuditLog{
		ActorType: "admin", ActorID: 1, Action: "test.action",
		ResourceType: "test", RiskLevel: "low", Message: "test",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"logs"`) {
		t.Errorf("应包含 logs，实际: %s", body)
	}
}

// ===== 前端结构测试 =====

func TestFrontend_AppShellExists(t *testing.T) {
	checkFileExists(t, "frontend/src/components/AppShell.vue")
}

func TestFrontend_SidebarExists(t *testing.T) {
	checkFileExists(t, "frontend/src/components/Sidebar.vue")
}

func TestFrontend_TopbarExists(t *testing.T) {
	checkFileExists(t, "frontend/src/components/Topbar.vue")
}

func TestFrontend_DashboardViewExists(t *testing.T) {
	checkFileExists(t, "frontend/src/features/dashboard/DashboardView.vue")
}

func TestFrontend_AccountsViewExists(t *testing.T) {
	checkFileExists(t, "frontend/src/features/accounts/AccountsView.vue")
}

func TestFrontend_AccountDetailViewExists(t *testing.T) {
	checkFileExists(t, "frontend/src/features/accounts/AccountDetailView.vue")
}

func TestFrontend_AccountLoginViewExists(t *testing.T) {
	checkFileExists(t, "frontend/src/features/accounts/AccountLoginView.vue")
}

func TestFrontend_ChatViewExists(t *testing.T) {
	checkFileExists(t, "frontend/src/features/chat/ChatView.vue")
}

func TestFrontend_SettingsViewExists(t *testing.T) {
	checkFileExists(t, "frontend/src/features/settings/SettingsView.vue")
}

func TestFrontend_AuditViewExists(t *testing.T) {
	checkFileExists(t, "frontend/src/features/audit/AuditView.vue")
}

func TestFrontend_ContactsViewExists(t *testing.T) {
	checkFileExists(t, "frontend/src/features/contacts/ContactsView.vue")
}

// projectRoot 返回项目根目录。
func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func checkFileExists(t *testing.T, relPath string) {
	t.Helper()
	fullPath := filepath.Join(projectRoot(), relPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Errorf("文件不存在: %s (完整路径: %s)", relPath, fullPath)
	}
}

// ===== 前端技术栈测试 =====

func TestVue_UsesRouter(t *testing.T) {
	checkFileContains(t, "frontend/src/router/index.ts", "vue-router")
}

func TestVue_UsesPinia(t *testing.T) {
	checkFileContains(t, "frontend/src/main.ts", "pinia")
}

func TestVue_UsesTanStackQuery(t *testing.T) {
	checkFileContains(t, "frontend/src/main.ts", "@tanstack/vue-query")
}

func TestVue_UsesTanStackVirtual(t *testing.T) {
	checkFileContains(t, "frontend/package.json", "@tanstack/vue-virtual")
}

func TestVue_NoCDN(t *testing.T) {
	content := readFileContent(t, "frontend/index.html")
	if strings.Contains(content, "cdn.") || strings.Contains(content, "unpkg.com") || strings.Contains(content, "jsdelivr") {
		t.Error("不应使用 CDN")
	}
}

func TestVue_NoTailwind(t *testing.T) {
	content := readFileContent(t, "frontend/package.json")
	if strings.Contains(content, "tailwindcss") {
		t.Error("不应使用 Tailwind")
	}
}

func TestVue_NoExternalUIFramework(t *testing.T) {
	content := readFileContent(t, "frontend/package.json")
	frameworks := []string{"element-plus", "ant-design-vue", "naive-ui", "vuetify"}
	for _, fw := range frameworks {
		if strings.Contains(content, fw) {
			t.Errorf("不应使用外部 UI 框架: %s", fw)
		}
	}
}

func TestVue_NoDangerousVHtmlForMessages(t *testing.T) {
	// 检查消息组件不使用 v-html 渲染用户消息
	content := readFileContent(t, "frontend/src/features/chat/MessageBubble.vue")
	// v-html 只用于安全的 linkify 内容，不用于原始消息
	if strings.Contains(content, "v-html=\"message.text\"") || strings.Contains(content, "v-html=\"msg\"") {
		t.Error("消息正文不应直接使用 v-html")
	}
}

func TestVue_UsesTypeScript(t *testing.T) {
	checkFileContains(t, "frontend/tsconfig.json", "compilerOptions")
}

func checkFileContains(t *testing.T, path, substr string) {
	t.Helper()
	content := readFileContent(t, path)
	if !strings.Contains(content, substr) {
		t.Errorf("文件 %s 应包含 %q", path, substr)
	}
}

func readFileContent(t *testing.T, relPath string) string {
	t.Helper()
	fullPath := filepath.Join(projectRoot(), relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("读取文件失败 %s: %v", relPath, err)
	}
	return string(data)
}

// ===== 视觉风格防回归测试 =====

func TestStyleTokens_PreserveLegacyColors(t *testing.T) {
	content := readFileContent(t, "frontend/src/styles/variables.css")

	requiredColors := map[string]string{
		"sidebar background (dark)":  "#0f172a",
		"sidebar background (light)": "#1e293b",
		"primary blue (light)":       "#2563eb",
		"primary blue (dark)":        "#3b82f6",
		"text primary (dark)":        "#f1f5f9",
		"border (dark)":              "#334155",
		"danger red (light)":         "#ef4444",
		"success green (light)":      "#10b981",
	}

	for name, color := range requiredColors {
		if !strings.Contains(content, color) {
			t.Errorf("样式变量应包含 %s (%s)", name, color)
		}
	}
}

func TestStyleTokens_PreserveSidebarWidth(t *testing.T) {
	content := readFileContent(t, "frontend/src/styles/variables.css")
	if !strings.Contains(content, "240px") {
		t.Error("sidebar 宽度应为 240px")
	}
}

func TestStyleTokens_PreserveTopbarHeight(t *testing.T) {
	content := readFileContent(t, "frontend/src/styles/variables.css")
	if !strings.Contains(content, "56px") {
		t.Error("topbar 高度应为 56px")
	}
}

func TestStyleTokens_PreserveCardStyle(t *testing.T) {
	content := readFileContent(t, "frontend/src/styles/shell.css")
	if !strings.Contains(content, ".card") {
		t.Error("应包含 card 样式")
	}
	if !strings.Contains(content, "border-radius") {
		t.Error("card 应有 border-radius")
	}
}

func TestStyleTokens_PreserveButtonStyle(t *testing.T) {
	content := readFileContent(t, "frontend/src/styles/shell.css")
	if !strings.Contains(content, ".btn-primary") {
		t.Error("应包含 btn-primary 样式")
	}
}

func TestStyleTokens_PreserveInputStyle(t *testing.T) {
	content := readFileContent(t, "frontend/src/styles/shell.css")
	if !strings.Contains(content, ".form-input") {
		t.Error("应包含 form-input 样式")
	}
}

func TestStyleTokens_PreserveBadgeStyle(t *testing.T) {
	content := readFileContent(t, "frontend/src/styles/shell.css")
	if !strings.Contains(content, ".badge") {
		t.Error("应包含 badge 样式")
	}
	if !strings.Contains(content, "badge-success") {
		t.Error("应包含 badge-success")
	}
}

func TestStyleTokens_PreserveAlertStyle(t *testing.T) {
	content := readFileContent(t, "frontend/src/styles/shell.css")
	if !strings.Contains(content, ".alert-error") {
		t.Error("应包含 alert-error")
	}
	if !strings.Contains(content, ".alert-success") {
		t.Error("应包含 alert-success")
	}
}

// ===== 路由行为测试 =====

func TestLegacyRoutes_NoDoubleAppPrefix(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	routes := []string{"/accounts", "/chats", "/audit", "/settings", "/contacts"}
	for _, path := range routes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", path, nil)
		req.Header.Set("Cookie", "atria_session="+sessionCookie)
		r.ServeHTTP(w, req)

		if w.Code == http.StatusFound {
			loc := w.Header().Get("Location")
			if strings.Contains(loc, "/app/app") {
				t.Errorf("%s 重定向到 %s 包含重复 /app 前缀", path, loc)
			}
		}
	}
}

func TestAPI_Me_CurrentAccountMatchesAccountsList(t *testing.T) {
	r, srv := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Aronn AT", "aronn_test", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Aronn AT") {
		t.Errorf("/api/me 应包含 Aronn AT，实际: %s", body)
	}
	if !strings.Contains(body, "current_account") {
		t.Errorf("/api/me 应包含 current_account，实际: %s", body)
	}
}

func TestAPI_Me_InvalidCookieFallbackToActiveAccount(t *testing.T) {
	r, srv := setupTestRouter(t)
	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Aronn AT", "aronn_test", model.TelegramAccountStatusActive)

	// Set invalid selected_account_id
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

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "Aronn AT") {
		t.Errorf("selected_account_id 无效时应 fallback，实际: %s", bodyStr)
	}
}

func TestAPI_Me_NoAccountReturnsNullCurrentAccount(t *testing.T) {
	r, _ := setupTestRouter(t)
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// current_account should be null when no accounts exist
	if !strings.Contains(body, `"current_account":null`) && !strings.Contains(body, `"current_account": null`) {
		// It's also ok if current_account is just not present
		if strings.Contains(body, `"current_account":{`) {
			t.Errorf("无账号时 current_account 应为 null，实际: %s", body)
		}
	}
}

// ===== Vue Router 测试 =====

func TestVueRouter_NoDoubleAppPrefix(t *testing.T) {
	routerContent := readFileContent(t, "frontend/src/router/index.ts")
	// Routes should NOT contain /app prefix since createWebHistory('/app/') handles it
	if strings.Contains(routerContent, "path: '/app/") {
		t.Error("router routes 不应包含 /app/ 前缀，base 已设置")
	}
}

func TestVueRouter_UsesHashHistory(t *testing.T) {
	routerContent := readFileContent(t, "frontend/src/router/index.ts")
	if !strings.Contains(routerContent, "createWebHashHistory") {
		t.Error("应使用 createWebHashHistory")
	}
}

func TestSidebar_UsesRouterPush(t *testing.T) {
	sidebarContent := readFileContent(t, "frontend/src/components/Sidebar.vue")
	if !strings.Contains(sidebarContent, "router.push") {
		t.Error("Sidebar 应使用 router.push 导航")
	}
	// Sidebar paths should NOT have /app prefix
	if strings.Contains(sidebarContent, "path: '/app/") {
		t.Error("Sidebar path 不应包含 /app/ 前缀")
	}
}

func TestAppShell_HasRouterView(t *testing.T) {
	content := readFileContent(t, "frontend/src/components/AppShell.vue")
	if !strings.Contains(content, "<router-view") {
		t.Error("AppShell 应包含 router-view")
	}
}

func TestSidebar_ActiveStateFromRoute(t *testing.T) {
	content := readFileContent(t, "frontend/src/components/Sidebar.vue")
	if !strings.Contains(content, "useRoute") {
		t.Error("Sidebar 应使用 useRoute 获取当前路由")
	}
}

func TestVueRoutes_MapToDistinctViews(t *testing.T) {
	routerContent := readFileContent(t, "frontend/src/router/index.ts")
	views := []string{
		"DashboardView",
		"AccountsView",
		"AccountLoginView",
		"AccountDetailView",
		"ChatView",
		"ContactsView",
		"AuditView",
		"SettingsView",
	}
	for _, v := range views {
		if !strings.Contains(routerContent, v) {
			t.Errorf("router 应映射到 %s", v)
		}
	}
}

// ===== 主题测试 =====

func TestTheme_InitDoesNotForceLight(t *testing.T) {
	content := readFileContent(t, "frontend/src/stores/app.ts")
	// Should not hardcode 'light' as default
	if strings.Contains(content, "ref<'light' | 'dark' | 'system'>('light')") {
		t.Error("主题初始化不应强制为 light")
	}
}

func TestTheme_UsesLegacyStorageKey(t *testing.T) {
	content := readFileContent(t, "frontend/src/stores/app.ts")
	if !strings.Contains(content, "atria-theme") {
		t.Error("应使用旧系统 localStorage key 'atria-theme'")
	}
}

func TestTheme_AppliesToDocumentRoot(t *testing.T) {
	content := readFileContent(t, "frontend/src/stores/app.ts")
	if !strings.Contains(content, "document.documentElement") {
		t.Error("主题应应用到 document.documentElement")
	}
}
