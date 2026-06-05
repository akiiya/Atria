package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/user/atria/internal/config"
	"github.com/user/atria/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// setupTestServer 创建测试用的服务器。
func setupTestServer(t *testing.T) (*Server, *gorm.DB) {
	t.Helper()

	// 创建内存数据库
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("创建测试数据库失败: %s", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(&model.Admin{}, &model.AuditLog{}, &model.APICredential{}, &model.TelegramAccount{}); err != nil {
		t.Fatalf("数据库迁移失败: %s", err)
	}

	// 创建测试配置
	cfg := &config.Config{
		AppName:        "Atria",
		Host:           "127.0.0.1",
		Port:           "8080",
		DataDir:        "./testdata",
		DatabaseDriver: "sqlite",
		DatabaseDSN:    ":memory:",
		SessionDir:     "./testdata/sessions",
		LogDir:         "./testdata/logs",
		SecretKeyFile:  "",
		CookieName:     "atria_session",
		CookieSecure:   false,
		CookieSameSite: "lax",
		CSRFEnabled:    true,
		CSRFHeaderName: "X-CSRF-Token",
		CSRFFieldName:  "csrf_token",
		SessionTTL:     24 * time.Hour,
	}

	// 测试密钥
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	srv := New(cfg, db, key)
	return srv, db
}

// setupTestRouter 创建测试用的路由器。
func setupTestRouter(t *testing.T) (*gin.Engine, *Server) {
	t.Helper()
	srv, _ := setupTestServer(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	srv.setupRoutes(r)

	return r, srv
}

// initAdmin 初始化管理员并返回 CSRF token 和 Cookie。
func initAdmin(t *testing.T, r *gin.Engine) (string, string) {
	t.Helper()

	// 获取初始化页面的 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	// 从响应中提取 CSRF cookie
	var csrfCookie string
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// 提交初始化表单
	w = httptest.NewRecorder()
	body := "username=admin&password=password123456&confirm_password=password123456&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/init", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 获取 session cookie
	var sessionCookie string
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_session" {
			sessionCookie = cookie.Value
		}
	}

	return csrfCookie, sessionCookie
}

// loginAdmin 登录管理员并返回新的 session cookie。
func loginAdmin(t *testing.T, r *gin.Engine) (string, string) {
	t.Helper()

	// 获取登录页面的 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	var csrfCookie string
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// 提交登录表单
	w = httptest.NewRecorder()
	body := "username=admin&password=password123456&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	var sessionCookie string
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_session" {
			sessionCookie = cookie.Value
		}
	}

	return csrfCookie, sessionCookie
}

func TestRouter_Uninitialized_GetRoot_RedirectsToInit(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "/login") && !strings.Contains(location, "/init") {
		t.Errorf("应重定向到 /login 或 /init，实际=%s", location)
	}
}

func TestRouter_Uninitialized_GetInit_Returns200(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际=%d", w.Code)
	}
}

func TestRouter_Uninitialized_GetLogin_RedirectsToInit(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "/init") {
		t.Errorf("应重定向到 /init，实际=%s", location)
	}
}

func TestRouter_PostInit_NoCSRF_Returns403(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	body := "username=admin&password=password123456&confirm_password=password123456"
	req, _ := http.NewRequest("POST", "/init", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("期望 403，实际=%d", w.Code)
	}
}

func TestRouter_Initialized_GetInit_RedirectsToLogin(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 先初始化管理员
	initAdmin(t, r)

	// 再访问 /init
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "/login") {
		t.Errorf("应重定向到 /login，实际=%s", location)
	}
}

func TestRouter_Initialized_NotLoggedIn_GetRoot_RedirectsToLogin(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 先初始化管理员
	initAdmin(t, r)

	// 未登录访问 /
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "/login") {
		t.Errorf("应重定向到 /login，实际=%s", location)
	}
}

func TestRouter_Initialized_GetLogin_Returns200(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 先初始化管理员
	initAdmin(t, r)

	// 访问 /login
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际=%d", w.Code)
	}
}

func TestRouter_PostLogin_NoCSRF_Returns403(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 先初始化管理员
	initAdmin(t, r)

	// POST /login 没有 CSRF
	w := httptest.NewRecorder()
	body := "username=admin&password=password123456"
	req, _ := http.NewRequest("POST", "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("期望 403，实际=%d", w.Code)
	}
}

func TestRouter_PostLogin_WrongPassword_NoLeak(t *testing.T) {
	r, srv := setupTestRouter(t)

	// 先初始化管理员
	csrfCookie, _ := initAdmin(t, r)

	// 获取登录页面的 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// POST 错误密码
	w = httptest.NewRecorder()
	body := "username=admin&password=wrong_password&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 检查响应不泄露具体错误
	resp := w.Body.String()
	if strings.Contains(resp, "用户不存在") || strings.Contains(resp, "密码错误") {
		t.Error("错误响应不应泄露具体原因")
	}

	// 检查审计日志
	var count int64
	srv.db.Model(&model.AuditLog{}).Where("action = ?", "admin.login_failed").Count(&count)
	if count == 0 {
		t.Error("登录失败应写入审计日志")
	}
}

func TestRouter_PostLogin_Success_SetsCookie(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 先初始化管理员
	csrfCookie, _ := initAdmin(t, r)

	// 获取登录页面的 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// POST 正确密码
	w = httptest.NewRecorder()
	body := "username=admin&password=password123456&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}

	// 检查设置了 session cookie
	hasSessionCookie := false
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_session" && cookie.Value != "" {
			hasSessionCookie = true
			// Cookie 不应包含明文用户名
			if strings.Contains(cookie.Value, "admin") {
				t.Error("Cookie 不应包含明文用户名")
			}
		}
	}

	if !hasSessionCookie {
		t.Error("登录成功应设置 session cookie")
	}
}

func TestRouter_LoggedIn_GetRoot_Returns200(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化并登录
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 访问 /
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际=%d", w.Code)
	}
}

func TestRouter_LoggedIn_GetSettings_Returns200(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化并登录
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 访问 /settings
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际=%d", w.Code)
	}
}

func TestRouter_NotLoggedIn_GetSettings_RedirectsToLogin(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化管理员
	initAdmin(t, r)

	// 未登录访问 /settings
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "/login") {
		t.Errorf("应重定向到 /login，实际=%s", location)
	}
}

func TestRouter_PostLogout_NoCSRF_Returns403(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化并登录
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// POST /logout 没有 CSRF
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/logout", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("期望 403，实际=%d", w.Code)
	}
}

func TestRouter_PostLogout_WithCSRF_ClearsCookie(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化并登录
	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// POST /logout 带 CSRF
	w := httptest.NewRecorder()
	body := "csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}

	// 检查 cookie 被清除
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_session" && cookie.MaxAge >= 0 {
			t.Error("登出应清除 session cookie")
		}
	}
}

func TestRouter_NotLoggedIn_GetCredentials_RedirectsToLogin(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化管理员
	initAdmin(t, r)

	// 未登录访问 /credentials
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/credentials", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "/login") {
		t.Errorf("应重定向到 /login，实际=%s", location)
	}
}

func TestRouter_LoggedIn_GetCredentials_Returns200(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化并登录
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 访问 /credentials
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/credentials", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际=%d", w.Code)
	}
}

func TestRouter_GetNonexistent_Returns404(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/nonexistent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望 404，实际=%d", w.Code)
	}
}

func TestRouter_AuditLogs_Initialized(t *testing.T) {
	r, srv := setupTestRouter(t)

	// 初始化管理员
	initAdmin(t, r)

	// 检查审计日志
	var count int64
	srv.db.Model(&model.AuditLog{}).Where("action = ?", "admin.initialized").Count(&count)
	if count == 0 {
		t.Error("初始化应写入 admin.initialized 审计日志")
	}
}

func TestRouter_AuditLogs_LoginSuccess(t *testing.T) {
	r, srv := setupTestRouter(t)

	// 初始化并登录
	initAdmin(t, r)
	loginAdmin(t, r)

	// 检查审计日志
	var count int64
	srv.db.Model(&model.AuditLog{}).Where("action = ?", "admin.login_success").Count(&count)
	if count == 0 {
		t.Error("登录成功应写入 admin.login_success 审计日志")
	}
}

func TestRouter_AuditLogs_Logout(t *testing.T) {
	r, srv := setupTestRouter(t)

	// 初始化并登录
	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 登出
	w := httptest.NewRecorder()
	body := "csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 检查审计日志
	var count int64
	srv.db.Model(&model.AuditLog{}).Where("action = ?", "admin.logout").Count(&count)
	if count == 0 {
		t.Error("登出应写入 admin.logout 审计日志")
	}
}

func TestRouter_AuditLogs_PasswordChanged(t *testing.T) {
	r, srv := setupTestRouter(t)

	// 初始化并登录
	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 修改密码
	w := httptest.NewRecorder()
	body := "current_password=password123456&new_password=new_password_123456&confirm_new_password=new_password_123456&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/settings/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 检查审计日志
	var count int64
	srv.db.Model(&model.AuditLog{}).Where("action = ?", "admin.password_changed").Count(&count)
	if count == 0 {
		t.Error("修改密码应写入 admin.password_changed 审计日志")
	}
}

func TestRouter_Cookie_NotContainsPlaintextPassword(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化并登录
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// Cookie 不应包含明文密码
	if strings.Contains(sessionCookie, "password123456") {
		t.Error("Cookie 不应包含明文密码")
	}
}

func TestRouter_Cookie_NotContainsPlaintextUsername(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化并登录
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// Cookie 不应包含明文用户名
	if strings.Contains(sessionCookie, "admin") {
		t.Error("Cookie 不应包含明文用户名")
	}
}

// ===== 模板内容验证测试 =====

func TestRouter_Uninitialized_InitPage_HasInitForm(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际=%d", w.Code)
	}

	body := w.Body.String()

	// 必须包含初始化相关内容
	mustContain := []string{
		"初始化管理员",
		"管理员用户名",
		"管理员密码",
		"确认密码",
		"form",
		`action="/init"`,
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("/init 页面缺少 %q", s)
		}
	}
}

func TestRouter_Uninitialized_InitPage_NotLoginForm(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// 不得包含登录相关内容
	mustNotContain := []string{
		`action="/login"`,
		`action="/logout"`,
		"credential-switcher",
		`href="/accounts"`,
		`href="/credentials"`,
		`href="/audit"`,
		`href="/settings"`,
		"账号会话",
		"API 凭据",
		"审计日志",
		"系统设置",
		"sidebar",
	}
	for _, s := range mustNotContain {
		if strings.Contains(body, s) {
			t.Errorf("/init 页面不应包含 %q", s)
		}
	}
}

func TestRouter_Initialized_LoginPage_HasLoginForm(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 先初始化管理员
	initAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际=%d", w.Code)
	}

	body := w.Body.String()

	// 必须包含登录相关内容
	mustContain := []string{
		"用户名",
		"密码",
		"登录",
		`action="/login"`,
		"form",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("/login 页面缺少 %q", s)
		}
	}
}

func TestRouter_Initialized_LoginPage_NotInitForm(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 先初始化管理员
	initAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// 不得包含初始化相关内容
	mustNotContain := []string{
		"确认密码",
		"初始化管理员",
		`action="/init"`,
		"credential-switcher",
		`href="/accounts"`,
		`href="/credentials"`,
		`href="/audit"`,
		"sidebar",
	}
	for _, s := range mustNotContain {
		if strings.Contains(body, s) {
			t.Errorf("/login 页面不应包含 %q", s)
		}
	}
}

func TestRouter_LoggedIn_Dashboard_HasSidebar(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化并登录
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际=%d", w.Code)
	}

	body := w.Body.String()

	// 已登录后台必须包含 sidebar 导航
	mustContain := []string{
		`href="/"`,
		`href="/accounts"`,
		`href="/credentials"`,
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("后台页面缺少导航 %q", s)
		}
	}
}

// ===== App Layout 结构验证测试 =====

func TestRouter_LoggedIn_Dashboard_HasAppLayoutStructure(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际=%d", w.Code)
	}

	body := w.Body.String()

	// 必须包含 app layout 关键结构
	mustContain := []string{
		"app-layout",
		"app-main",
		"topbar",
		"topbar-right",
		"sidebar-brand",
		"brand-name",
		"page-header",
		"page-heading",
		"page-actions",
		"credential-switcher",
		"app-content",
		"sidebar",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("后台页面缺少结构 %q", s)
		}
	}
}

func TestRouter_LoggedIn_Dashboard_CredentialSwitcherInPageHeader(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// credential-switcher 必须在 page-header 内部
	pageHeaderIdx := strings.Index(body, "page-header")
	credentialIdx := strings.Index(body, "credential-switcher")
	pageBodyIdx := strings.Index(body, "page-body")

	if pageHeaderIdx < 0 || credentialIdx < 0 {
		t.Fatal("页面缺少 page-header / credential-switcher")
	}

	// 如果有 page-body，credential-switcher 应在 page-header 之后、page-body 之前
	// 否则 credential-switcher 应在 page-header 之后
	if pageBodyIdx > 0 {
		if credentialIdx < pageHeaderIdx || credentialIdx > pageBodyIdx {
			t.Error("credential-switcher 应在 page-header 和 page-body 之间")
		}
	} else {
		if credentialIdx < pageHeaderIdx {
			t.Error("credential-switcher 应在 page-header 之后")
		}
	}
}

func TestRouter_LoggedIn_Dashboard_NoCredentialSwitcherInTopbar(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// topbar 内不应包含 credential-switcher
	topbarStart := strings.Index(body, "topbar")
	topbarEnd := strings.Index(body, "</header>")
	credentialIdx := strings.Index(body, "credential-switcher")

	if topbarStart >= 0 && topbarEnd > 0 && credentialIdx >= 0 {
		if credentialIdx > topbarStart && credentialIdx < topbarEnd {
			t.Error("credential-switcher 不应在 topbar 内")
		}
	}
}

func TestRouter_LoginPage_NoCredentialSwitcher(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, "credential-switcher") {
		t.Error("/login 页面不应包含 credential-switcher")
	}
}

func TestRouter_InitPage_NoCredentialSwitcher(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, "credential-switcher") {
		t.Error("/init 页面不应包含 credential-switcher")
	}
}

func TestRouter_LoggedIn_Dashboard_NoSensitiveData(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// 不得包含敏感数据
	sensitiveTerms := []string{
		"api_hash",
		"api_id",
		"session_data",
		"password123456",
	}
	for _, s := range sensitiveTerms {
		if strings.Contains(body, s) {
			t.Errorf("页面不应包含敏感数据 %q", s)
		}
	}
}

func TestRouter_LoggedIn_Settings_HasAppLayout(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际=%d", w.Code)
	}

	body := w.Body.String()

	// settings 页面也必须包含 app layout 结构
	mustContain := []string{
		"topbar",
		"app-content",
		"sidebar",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("settings 页面缺少结构 %q", s)
		}
	}
}

func TestRouter_LoggedIn_Dashboard_HasStatCards(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际=%d", w.Code)
	}

	body := w.Body.String()

	// 必须包含 stat card 结构
	mustContain := []string{
		"stat-card",
		"stat-icon",
		"stat-content",
		"stat-value",
		"stat-label",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("仪表盘页面缺少结构 %q", s)
		}
	}
}

func TestRouter_LoggedIn_Dashboard_HasBrandAndPageActions(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// 必须包含 brand 和 page-actions
	mustContain := []string{
		"sidebar-brand",
		"brand-name",
		"page-actions",
		"credential-switcher",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("仪表盘页面缺少结构 %q", s)
		}
	}
}

func TestRouter_Uninitialized_InitPage_NoSidebarBrand(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, "sidebar-brand") {
		t.Error("/init 页面不应包含 sidebar-brand")
	}
}

func TestRouter_Initialized_LoginPage_NoSidebarBrand(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, "sidebar-brand") {
		t.Error("/login 页面不应包含 sidebar-brand")
	}
}
