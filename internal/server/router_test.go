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
