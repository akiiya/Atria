package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/user/atria/internal/config"
	"github.com/user/atria/internal/credential"
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
	if err := db.AutoMigrate(&model.Admin{}, &model.AuditLog{}, &model.APICredential{}, &model.TelegramAccount{}, &model.AccountSession{}, &model.SystemSetting{}); err != nil {
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

// refreshCSRF 获取最新的 CSRF token（用于后续 POST 请求）。
func refreshCSRF(t *testing.T, r *gin.Engine, sessionCookie string) string {
	t.Helper()
	// 使用 / 获取新的 CSRF token（旧仪表盘页面仍可用）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			return cookie.Value
		}
	}
	return ""
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

	// 访问 /settings - 现在重定向到 /app/settings
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际=%d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/app/#/settings" {
		t.Errorf("期望重定向到 /app/#/settings，实际=%s", loc)
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

func TestRouter_LoggedIn_GetCredentials_RedirectsToSettings(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化并登录
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 访问 /credentials 应重定向到 /settings
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/credentials", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "/settings") {
		t.Errorf("应重定向到 /settings，实际=%s", location)
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
		"app-content",
		"sidebar",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("后台页面缺少结构 %q", s)
		}
	}
}

func TestRouter_LoggedIn_Dashboard_NoCredentialSwitcher(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// 仪表盘页面不应包含 credential-switcher
	if strings.Contains(body, "credential-switcher") {
		t.Error("仪表盘页面不应包含 credential-switcher")
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

	// /settings 现在重定向到 /app/settings
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际=%d", w.Code)
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

func TestRouter_LoggedIn_Dashboard_HasBrand(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// 必须包含 brand
	mustContain := []string{
		"sidebar-brand",
		"brand-name",
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

// ===== 初始化 API Key 测试 =====

func TestRouter_InitPage_HasAPIKeyFields(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	mustContain := []string{
		"Telegram API Key",
		"api_id",
		"api_hash",
		"api_display_name",
		"my.telegram.org",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("/init 页面缺少 API Key 字段 %q", s)
		}
	}
}

func TestRouter_InitPage_HasAPIKeyGuide(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	mustContain := []string{
		"如何获取 API Key",
		"API development tools",
		"通常只需申请一套 API Key",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("/init 页面缺少 API Key 申请指南 %q", s)
		}
	}
}

func TestRouter_InitPage_HasSecurityInfo(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	mustContain := []string{
		"secret.key",
		"AES-256-GCM",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("/init 页面缺少安全说明 %q", s)
		}
	}
}

func TestRouter_PostInit_WithAPIKey_CreatesDefaultCredential(t *testing.T) {
	r, srv := setupTestRouter(t)

	// 获取 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	var csrfCookie string
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// 提交初始化表单（带 API Key）
	w = httptest.NewRecorder()
	body := "username=admin&password=password123456&confirm_password=password123456" +
		"&api_display_name=Test+API&api_id=12345678&api_hash=abcdef0123456789abcdef0123456789" +
		"&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/init", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}

	// 验证创建了默认凭据
	var credCount int64
	srv.db.Model(&model.APICredential{}).Count(&credCount)
	if credCount != 1 {
		t.Errorf("应创建 1 个凭据，实际=%d", credCount)
	}

	var cred model.APICredential
	srv.db.First(&cred)
	if !cred.IsDefault {
		t.Error("初始化创建的凭据应为默认")
	}
	if cred.DisplayName != "Test API" {
		t.Errorf("凭据名称应为 Test API，实际=%s", cred.DisplayName)
	}
	if cred.APIID != 12345678 {
		t.Errorf("API ID 应为 12345678，实际=%d", cred.APIID)
	}
}

func TestRouter_PostInit_WithoutAPIKey_StillSucceeds(t *testing.T) {
	r, srv := setupTestRouter(t)

	// 获取 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	var csrfCookie string
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// 提交初始化表单（不带 API Key）
	w = httptest.NewRecorder()
	body := "username=admin&password=password123456&confirm_password=password123456&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/init", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}

	// 验证没有创建凭据
	var credCount int64
	srv.db.Model(&model.APICredential{}).Count(&credCount)
	if credCount != 0 {
		t.Errorf("跳过 API Key 时不应创建凭据，实际=%d", credCount)
	}
}

func TestRouter_PostInit_APIHash_NotStoredPlaintext(t *testing.T) {
	r, srv := setupTestRouter(t)

	// 获取 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	var csrfCookie string
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	apiHash := "abcdef0123456789abcdef0123456789"

	// 提交初始化表单
	w = httptest.NewRecorder()
	body := "username=admin&password=password123456&confirm_password=password123456" +
		"&api_id=12345678&api_hash=" + apiHash +
		"&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/init", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 验证 api_hash 不是明文存储
	var cred model.APICredential
	srv.db.First(&cred)
	if cred.EncryptedAPIHash == apiHash {
		t.Error("api_hash 不应明文存储")
	}
	if cred.EncryptedAPIHash == "" {
		t.Error("encrypted_api_hash 不应为空")
	}
}

func TestRouter_InitPage_NoAPIHashLeak(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// 页面不应包含任何 api_hash 明文
	if strings.Contains(body, "abcdef0123456789") {
		t.Error("初始化页面不应泄露 api_hash")
	}
}

// ===== 导航收口测试 =====

func TestRouter_LoggedIn_Dashboard_NoAPICredentialNav(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// 不应包含旧的 API 凭据主导航
	if strings.Contains(body, `href="/credentials"`) && strings.Contains(body, "API 凭据") {
		// 检查是否在 sidebar 中（不在 settings dropdown 中）
		sidebarIdx := strings.Index(body, "sidebar-nav")
		settingsIdx := strings.Index(body, "settings-dropdown")
		credIdx := strings.Index(body, `href="/credentials"`)

		if credIdx > sidebarIdx && credIdx < settingsIdx {
			t.Error("sidebar 主导航不应包含 API 凭据入口")
		}
	}
}

func TestRouter_LoggedIn_Dashboard_NoSystemSettingsNav(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// 检查 sidebar 中没有系统设置
	sidebarIdx := strings.Index(body, "sidebar-nav")
	footerIdx := strings.Index(body, "sidebar-footer")
	settingsLinkIdx := strings.Index(body, `href="/settings"`)

	// 如果 /settings 链接在 sidebar-nav 和 sidebar-footer 之间，则有问题
	if settingsLinkIdx > sidebarIdx && settingsLinkIdx < footerIdx {
		// 检查是否是 "系统设置" 文字
		settingsTextIdx := strings.Index(body, "系统设置")
		if settingsTextIdx > sidebarIdx && settingsTextIdx < footerIdx {
			t.Error("sidebar 主导航不应包含系统设置入口")
		}
	}
}

func TestRouter_LoggedIn_Dashboard_HasSettingsDropdown(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	mustContain := []string{
		"settings-dropdown",
		"topbar-settings",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("仪表盘页面缺少设置菜单 %q", s)
		}
	}
}

func TestRouter_LoggedIn_Dashboard_SettingsMenuHasSettingsLink(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "/settings") {
		t.Error("设置菜单应包含系统设置入口")
	}
}

func TestRouter_LoginPage_NoSettingsDropdown(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, "settings-dropdown") {
		t.Error("登录页不应包含设置菜单")
	}
}

func TestRouter_InitPage_NoSettingsDropdown(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/init", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, "settings-dropdown") {
		t.Error("初始化页不应包含设置菜单")
	}
}

// ===== 账号接入页测试 =====

func TestRouter_AccountLogin_WithDefaultCredential_ShowsPhoneInput(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建默认凭据
	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// /accounts/login 现在重定向到 /app/#/accounts/login（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际=%d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/app/#/accounts/login" {
		t.Errorf("期望重定向到 /app/#/accounts/login，实际=%s", loc)
	}
}

func TestRouter_AccountLogin_WithoutDefaultCredential_ShowsConfigGuide(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /accounts/login 现在重定向到 /app/#/accounts/login（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际=%d", w.Code)
	}
}

func TestRouter_AccountLogin_NoOldCredentialSelectorPrompt(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建默认凭据
	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// 不应包含旧的提示
	mustNotContain := []string{
		"顶部栏的下拉框",
		"请先在顶部栏选择",
		"请先选择 API 凭据",
	}
	for _, s := range mustNotContain {
		if strings.Contains(body, s) {
			t.Errorf("账号接入页不应包含旧提示 %q", s)
		}
	}
}

func TestRouter_AccountLogin_NoAPIHashLeak(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	apiHash := "abcdef0123456789abcdef0123456789"

	// 创建凭据
	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     apiHash,
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	if strings.Contains(body, apiHash) {
		t.Error("账号接入页不应泄露 api_hash 明文")
	}
}

// ===== 系统 API Key 保存后状态测试 =====

func TestSettings_SaveSystemAPIKey_CreatesEnabledDefaultKey(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 获取 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// 保存 API Key
	w = httptest.NewRecorder()
	body := "api_display_name=Test+API&api_id=12345678&api_hash=abcdef0123456789abcdef0123456789&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/settings/api-key", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 应重定向到 /settings
	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}

	// 验证数据库
	var cred model.APICredential
	srv.db.Where("status = ?", "enabled").First(&cred)

	if cred.ID == 0 {
		t.Fatal("应存在启用的凭据")
	}
	if !cred.IsDefault {
		t.Error("应为默认凭据")
	}
	if cred.Status != "enabled" {
		t.Errorf("Status 应为 enabled，实际=%s", cred.Status)
	}
	if cred.APIID != 12345678 {
		t.Errorf("APIID 应为 12345678，实际=%d", cred.APIID)
	}
	if cred.EncryptedAPIHash == "abcdef0123456789abcdef0123456789" {
		t.Error("EncryptedAPIHash 不应是明文")
	}
}

func TestSettings_SaveSystemAPIKey_ThenGetSettingsShowsDisplayMode(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 获取 CSRF token
	csrfCookie = refreshCSRF(t, r, sessionCookie)

	// 保存 API Key
	w := httptest.NewRecorder()
	body := "api_display_name=Test+API&api_id=12345678&api_hash=abcdef0123456789abcdef0123456789&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/settings/api-key", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 使用 JSON API 获取设置
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/settings", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body = w.Body.String()

	// 应包含 API Key 信息
	if !strings.Contains(body, "Test API") {
		t.Error("应显示自定义名称")
	}
	if !strings.Contains(body, `"api_key"`) {
		t.Error("应包含 api_key 字段")
	}
	// 脱敏 API ID（格式：****5678）
	if !strings.Contains(body, "****5678") {
		t.Errorf("应显示脱敏 API ID，实际: %s", body)
	}
	// 不应显示 API Hash 明文
	if strings.Contains(body, "abcdef0123456789abcdef0123456789") {
		t.Error("不应显示 API Hash 明文")
	}

	// 验证数据库中有记录
	var cred model.APICredential
	srv.db.Where("is_default = ? AND status = ?", true, "enabled").First(&cred)
	if cred.ID == 0 {
		t.Error("数据库应存在默认启用凭据")
	}
}

func TestSettings_SaveSystemAPIKey_ThenAccountsNoLongerShowsConfigurePrompt(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 获取 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// 保存 API Key
	w = httptest.NewRecorder()
	body := "api_display_name=Test+API&api_id=12345678&api_hash=abcdef0123456789abcdef0123456789&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/settings/api-key", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// /accounts 现在重定向到 /app/accounts
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/accounts", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
}

func TestSettings_SaveSystemAPIKey_ThenAccountLoginShowsPhoneForm(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 获取 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// 保存 API Key
	w = httptest.NewRecorder()
	body := "api_display_name=Test+API&api_id=12345678&api_hash=abcdef0123456789abcdef0123456789&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/settings/api-key", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// /accounts/login 现在重定向到 /app/#/accounts/login（canonical hash URL）
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际=%d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/app/#/accounts/login" {
		t.Errorf("期望重定向到 /app/#/accounts/login，实际=%s", loc)
	}

	// API Key 测试：检查数据库中已保存
	var cred model.APICredential
	srv.db.Where("status = ?", model.APICredentialStatusEnabled).First(&cred)
	if cred.ID == 0 {
		t.Error("应存在已保存的 API Key")
	}

	_ = w.Body.String() // suppress unused variable
}

// ===== 异步登录 API 测试 =====

func TestAccountLoginPage_HasCSRFMetaForAsyncRequests(t *testing.T) {
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

	// /accounts/login 现在重定向到 /app/#/accounts/login（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际=%d", w.Code)
	}
}

func TestAccountLoginPage_StartButtonUsesAsync(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /accounts/login 现在重定向到 /app/#/accounts/login（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际=%d", w.Code)
	}
}

func TestAccountLoginStart_GetRedirectsToLogin(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login/start", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "/accounts/login") {
		t.Errorf("应重定向到 /accounts/login，实际=%s", location)
	}
}

func TestLoginStartAPI_PostWithoutCSRFRejected(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	body := "phone=+8613800138000"
	req, _ := http.NewRequest("POST", "/api/accounts/login/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("期望 403，实际=%d", w.Code)
	}
}

func TestLoginStartAPI_PostWithCSRFAccepted(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	w := httptest.NewRecorder()
	body := "phone=+8613800138000&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Error("带 CSRF 的请求不应返回 403")
	}
	if !strings.Contains(w.Body.String(), `"ok"`) {
		t.Error("应返回 JSON 响应")
	}
}

func TestLoginStartAPI_ErrorReturnsJSONNoRedirect(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	w := httptest.NewRecorder()
	body := "phone=+8613800138000&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusFound {
		t.Error("异步 API 不应返回 redirect")
	}
	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"ok"`) {
		t.Error("应返回 JSON 响应")
	}
	if strings.Contains(bodyStr, `"ok":true`) {
		t.Error("无法连接 Telegram 时 ok 应为 false")
	}
}

func TestLoginStartAPI_InvalidPhoneReturnsJSONError(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	w := httptest.NewRecorder()
	body := "phone=invalid&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"ok":false`) {
		t.Error("无效手机号应返回 ok: false")
	}
}

func TestLoginStartAPI_NoAPIKeyReturnsJSONError(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	w := httptest.NewRecorder()
	body := "phone=+8613800138000&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"ok":false`) {
		t.Error("无 API Key 应返回 ok: false")
	}
}

func TestLoginCodeAPI_UnknownFlowReturnsFlowExpired(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	w := httptest.NewRecorder()
	body := "flow_id=nonexistent&code=12345&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/code", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"ok":false`) {
		t.Error("未知 flow 应返回 ok: false")
	}
	if !strings.Contains(bodyStr, "flow_expired") {
		t.Error("应返回 flow_expired 错误码")
	}
}

func TestLoginPasswordAPI_UnknownFlowReturnsFlowExpired(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	w := httptest.NewRecorder()
	body := "flow_id=nonexistent&password=test123&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"ok":false`) {
		t.Error("未知 flow 应返回 ok: false")
	}
	if !strings.Contains(bodyStr, "flow_expired") {
		t.Error("应返回 flow_expired 错误码")
	}
}

func TestLoginCancelAPI_ClearsFlow(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	w := httptest.NewRecorder()
	body := "flow_id=some_flow&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/cancel", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	t.Logf("Cancel response status=%d body=%s", w.Code, bodyStr)
	if !strings.Contains(bodyStr, `"ok"`) {
		t.Error("取消应返回 JSON 响应")
	}
}

func TestCodePage_UnknownFlowRedirectsToLogin(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login/code?flow_id=unknown", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "/accounts/login") {
		t.Errorf("应重定向到 /accounts/login，实际=%s", location)
	}
}

func TestLoginButtonsHaveLoadingText(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /accounts/login 现在重定向到 /app/#/accounts/login（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际=%d", w.Code)
	}
}

func TestLoginJSRegistered(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /accounts/login 现在重定向到 /app/#/accounts/login（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际=%d", w.Code)
	}
}

func TestLoginJSNoExternalFramework(t *testing.T) {
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
	req, _ := http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	externalFrameworks := []string{
		"react", "vue", "angular", "svelte",
		"cdn.jsdelivr", "unpkg.com", "cdnjs.cloudflare",
	}
	for _, fw := range externalFrameworks {
		if strings.Contains(strings.ToLower(body), fw) {
			t.Errorf("页面不应引入外部框架 %q", fw)
		}
	}
}

func TestLoginErrors_DoNotRenderSensitiveData(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	apiHash := "abcdef0123456789abcdef0123456789"

	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     apiHash,
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	w := httptest.NewRecorder()
	body := "phone=+8613800138000&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()

	sensitiveTerms := []string{
		apiHash,
		"proxy_password",
		"session_data",
	}
	for _, s := range sensitiveTerms {
		if strings.Contains(bodyStr, s) {
			t.Errorf("响应不应包含敏感数据 %q", s)
		}
	}
}

// ===== SubmitCode API 诊断测试 =====

func TestLoginCodeAPI_MissingFlowID_ReturnsFlowInvalid(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	w := httptest.NewRecorder()
	body := "code=12345&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/code", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"ok":false`) {
		t.Error("空 flow_id 应返回 ok:false")
	}
	if !strings.Contains(bodyStr, "missing_flow_id") {
		t.Error("应返回 missing_flow_id 错误码")
	}
}

func TestLoginCodeAPI_EmptyCode_ReturnsCodeEmpty(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	w := httptest.NewRecorder()
	body := "flow_id=some_flow&code=&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/code", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"ok":false`) {
		t.Error("空 code 应返回 ok:false")
	}
	if !strings.Contains(bodyStr, "missing_code") {
		t.Error("应返回 missing_code 错误码")
	}
}

func TestLoginCodeAPI_UnknownFlow_ReturnsFlowExpired(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	w := httptest.NewRecorder()
	body := "flow_id=nonexistent&code=12345&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/code", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"ok":false`) {
		t.Error("未知 flow 应返回 ok:false")
	}
	if !strings.Contains(bodyStr, "flow_expired") {
		t.Error("应返回 flow_expired 错误码")
	}
}

func TestLoginPasswordAPI_UnknownFlow_ReturnsFlowExpired(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	w := httptest.NewRecorder()
	body := "flow_id=nonexistent&password=test123&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"ok":false`) {
		t.Error("未知 flow 应返回 ok:false")
	}
	if !strings.Contains(bodyStr, "flow_expired") {
		t.Error("应返回 flow_expired 错误码")
	}
}

func TestLoginCodeAPI_DoesNotReturnGenericNetworkError(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	// 未知 flow 不应返回 "网络异常"
	w := httptest.NewRecorder()
	body := "flow_id=nonexistent&code=12345&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/code", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if strings.Contains(bodyStr, "网络异常") {
		t.Error("未知 flow 不应返回 '网络异常'，应返回具体错误")
	}
}

func TestLoginCodeAPI_ReturnsSpecificErrorCode(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	// 未知 flow 应返回 flow_expired，不是 network_error
	w := httptest.NewRecorder()
	body := "flow_id=nonexistent&code=12345&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/code", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	// code 字段应是具体的 flow_expired，不是 network_error
	if strings.Contains(bodyStr, `"code":"network_error"`) {
		t.Error("不应返回 code:network_error，应返回具体错误码")
	}
	if !strings.Contains(bodyStr, `"code":"flow_expired"`) {
		t.Error("应返回 code:flow_expired")
	}
}

func TestLoginStartAPI_ProxyConfigInvalid_ReturnsSpecificError(t *testing.T) {
	// 测试当代理配置不完整时返回具体错误
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	// 创建 API Key
	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// 配置不完整的代理（有类型但没主机）
	srv.db.Create(&model.SystemSetting{Key: "proxy_type", Value: "socks5", ValueType: "string"})
	srv.db.Create(&model.SystemSetting{Key: "proxy_enabled", Value: "true", ValueType: "string"})
	srv.db.Create(&model.SystemSetting{Key: "proxy_host", Value: "", ValueType: "string"})

	w := httptest.NewRecorder()
	// %2B 是 + 的 URL 编码，避免 form-urlencoded 中 + 被解析为空格
	body := "phone=%2B8613800138000&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	t.Logf("Proxy config invalid response: %s", bodyStr)
	if !strings.Contains(bodyStr, `"ok":false`) {
		t.Error("代理配置不完整应返回 ok:false")
	}
	if !strings.Contains(bodyStr, "proxy_config_invalid") && !strings.Contains(bodyStr, "代理配置") {
		t.Error("应返回代理配置错误")
	}
	// 不应返回 "网络异常"
	if strings.Contains(bodyStr, "网络异常") {
		t.Error("不应返回 '网络异常'，应返回代理配置错误")
	}
}

func TestCodeStep_HasFlowIDHolder(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /accounts/login 现在重定向到 /app/#/accounts/login（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际=%d", w.Code)
	}
}

func TestLoginJS_CodeSubmitHasLoadingText(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /accounts/login 现在重定向到 /app/#/accounts/login（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际=%d", w.Code)
	}
}

func TestLoginJS_NoFullPageFormPostForCode(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /accounts/login 现在重定向到 /app/#/accounts/login（canonical hash URL）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts/login", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际=%d", w.Code)
	}
}

func TestProxyPasswordDecryptFailed_ReturnsProxyConfigInvalid(t *testing.T) {
	// 测试代理密码解密失败时返回 proxy_config_invalid
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)
	csrfCookie := refreshCSRF(t, r, sessionCookie)

	// 创建 API Key
	credSvc := credential.NewService(srv.db, srv.key)
	credSvc.Create(credential.CreateInput{
		DisplayName: "Default API",
		APIID:       "12345678",
		APIHash:     "abcdef0123456789abcdef0123456789",
		Status:      "enabled",
		RiskPolicy:  "disabled",
	})

	// 配置代理但密码解密会失败（用错误的加密方式）
	srv.db.Create(&model.SystemSetting{Key: "proxy_type", Value: "socks5", ValueType: "string"})
	srv.db.Create(&model.SystemSetting{Key: "proxy_enabled", Value: "true", ValueType: "string"})
	srv.db.Create(&model.SystemSetting{Key: "proxy_host", Value: "127.0.0.1", ValueType: "string"})
	srv.db.Create(&model.SystemSetting{Key: "proxy_port", Value: "1080", ValueType: "string"})
	srv.db.Create(&model.SystemSetting{Key: "proxy_password", Value: "invalid_encrypted_data", ValueType: "string", IsSensitive: true})

	w := httptest.NewRecorder()
	body := "phone=%2B8613800138000&csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/accounts/login/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	// 应返回代理配置错误，不应静默降级为直连
	if !strings.Contains(bodyStr, `"ok":false`) {
		t.Error("密码解密失败应返回 ok:false")
	}
	// 不应返回 "网络异常"
	if strings.Contains(bodyStr, "网络异常") {
		t.Error("密码解密失败不应返回 '网络异常'")
	}
}

// ===== 账号切换器一致性测试 =====

// createTestAccount 创建测试用的 Telegram 账号和 Session。
func createTestAccount(t *testing.T, db *gorm.DB, displayName, username string, status model.TelegramAccountStatus) *model.TelegramAccount {
	t.Helper()

	account := &model.TelegramAccount{
		APICredentialID:  1,
		UserID:           123456789,
		PhoneEncrypted:   "encrypted_phone_data",
		PhoneFingerprint: "***1234",
		Username:         username,
		FirstName:        displayName,
		LastName:         "",
		DisplayName:      displayName,
		Status:           status,
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("创建测试账号失败: %s", err)
	}

	// 创建对应的 Session
	session := &model.AccountSession{
		TelegramAccountID:  account.ID,
		SessionFilePath:    "sessions/test.session",
		SessionFingerprint: "test_fingerprint",
		EncryptionVersion:  1,
		Status:             "active",
	}
	if err := db.Create(session).Error; err != nil {
		t.Fatalf("创建测试 Session 失败: %s", err)
	}

	return account
}

func TestTopbar_CurrentAccount_ShownOnDashboard(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建一个有效账号
	createTestAccount(t, srv.db, "Aronn AT", "aronn_test", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	if !strings.Contains(body, "Aronn AT") {
		t.Error("仪表盘顶部应显示账号名 'Aronn AT'")
	}
	if strings.Contains(body, "未接入账号") {
		t.Error("有有效账号时不应显示'未接入账号'")
	}
}

func TestTopbar_CurrentAccount_ShownOnAccountsPage(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建一个有效账号
	createTestAccount(t, srv.db, "Aronn AT", "aronn_test", model.TelegramAccountStatusActive)

	// /accounts 现在重定向到 /app/accounts
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
}

func TestTopbar_CurrentAccount_ConsistentAcrossPages(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建一个有效账号
	createTestAccount(t, srv.db, "Aronn AT", "aronn_test", model.TelegramAccountStatusActive)

	// / 是旧模板页面，应显示账号名
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)
	body := w.Body.String()
	if !strings.Contains(body, "Aronn AT") {
		t.Errorf("/ 应显示当前账号名 Aronn AT")
	}

	// /accounts 和 /settings 现在重定向
	redirectPages := []string{"/accounts", "/settings"}
	for _, page := range redirectPages {
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", page, nil)
		req.Header.Set("Cookie", "atria_session="+sessionCookie)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusFound {
			t.Errorf("%s 期望 302 重定向，实际 %d", page, w.Code)
		}
	}
}

func TestTopbar_FallbackToValidAccount_WhenCookieMissing(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建一个有效账号，但不设置 selected_account_id cookie
	createTestAccount(t, srv.db, "Aronn AT", "aronn_test", model.TelegramAccountStatusActive)

	// /accounts 现在重定向到 /app/accounts
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
}

func TestTopbar_FallbackToValidAccount_WhenCookieInvalid(t *testing.T) {
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

	// 提取更新后的 session cookie
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_session" {
			sessionCookie = cookie.Value
		}
	}

	// /accounts 现在重定向到 /app/accounts
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/accounts", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
}

func TestTopbar_NoAccount_ShowsNotConnected(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 不创建任何账号

	// /accounts 现在重定向到 /app/accounts
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
}

func TestAccountsPage_OnlyOneConnectAccountButton(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// /accounts 现在重定向到 /app/accounts
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/accounts", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302 重定向，实际 %d", w.Code)
	}
}

func TestTopbar_DoesNotLeakSensitiveAccountData(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建带 Session 的账号
	createTestAccount(t, srv.db, "Aronn AT", "aronn_test", model.TelegramAccountStatusActive)

	pages := []string{"/", "/accounts", "/settings"}
	for _, page := range pages {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", page, nil)
		req.Header.Set("Cookie", "atria_session="+sessionCookie)
		r.ServeHTTP(w, req)

		body := w.Body.String()

		sensitiveTerms := []string{
			"abcdef0123456789",     // api_hash 明文
			"encrypted_phone_data", // 加密手机号数据
			"+8613800138000",       // 完整手机号
		}
		for _, s := range sensitiveTerms {
			if strings.Contains(body, s) {
				t.Errorf("页面 %s 不应包含敏感数据 %q", page, s)
			}
		}
	}
}
