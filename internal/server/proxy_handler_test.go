package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/atria/internal/model"
)

func TestProxyHandler_GetSettings_NoAuth_RedirectsToLogin(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/proxy", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际=%d", w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "/login") {
		t.Errorf("应重定向到 /login，实际=%s", location)
	}
}

func TestProxyHandler_GetSettings_Authenticated_Returns200(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/proxy", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际=%d", w.Code)
	}
}

func TestProxyHandler_PostSettings_SaveHTTPS_Success(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 获取 proxy 页面的 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/proxy", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// 保存 HTTPS 代理
	w = httptest.NewRecorder()
	body := "proxy_type=https&proxy_host=127.0.0.1&proxy_port=8080&proxy_username=&proxy_password=&proxy_remark=&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/settings/proxy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际=%d", w.Code)
	}

	// 验证设置已保存
	var setting model.SystemSetting
	srv.db.Where("key = ?", "proxy_type").First(&setting)
	if setting.Value != "https" {
		t.Errorf("proxy_type 应为 https，实际=%s", setting.Value)
	}
}

func TestProxyHandler_PostSettings_SaveSOCKS5_Success(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 获取 proxy 页面的 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/proxy", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// 保存 SOCKS5 代理
	w = httptest.NewRecorder()
	body := "proxy_type=socks5&proxy_host=10.0.0.1&proxy_port=1080&proxy_username=user&proxy_password=pass123&proxy_remark=测试代理&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/settings/proxy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际=%d", w.Code)
	}

	// 验证设置已保存
	var setting model.SystemSetting
	srv.db.Where("key = ?", "proxy_type").First(&setting)
	if setting.Value != "socks5" {
		t.Errorf("proxy_type 应为 socks5，实际=%s", setting.Value)
	}
}

func TestProxyHandler_PostSettings_DisableProxy_Success(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 先保存一个代理
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/proxy", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	w = httptest.NewRecorder()
	body := "proxy_type=none&proxy_host=&proxy_port=&proxy_username=&proxy_password=&proxy_remark=&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/settings/proxy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际=%d", w.Code)
	}

	// 验证设置已保存
	var setting model.SystemSetting
	srv.db.Where("key = ?", "proxy_type").First(&setting)
	if setting.Value != "none" {
		t.Errorf("proxy_type 应为 none，实际=%s", setting.Value)
	}
}

func TestProxyHandler_PostSettings_InvalidPort_Error(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 获取 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/proxy", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// 保存无效端口
	w = httptest.NewRecorder()
	body := "proxy_type=https&proxy_host=127.0.0.1&proxy_port=99999&proxy_username=&proxy_password=&proxy_remark=&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/settings/proxy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 应返回错误
	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "无效的代理端口") {
		t.Error("应返回无效端口错误")
	}
}

func TestProxyHandler_PostSettings_InvalidType_Error(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 获取 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/proxy", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// 保存无效类型
	w = httptest.NewRecorder()
	body := "proxy_type=invalid&proxy_host=127.0.0.1&proxy_port=8080&proxy_username=&proxy_password=&proxy_remark=&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/settings/proxy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 应返回错误
	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "无效的代理类型") {
		t.Error("应返回无效代理类型错误")
	}
}

func TestProxyHandler_PostSettings_PasswordEncrypted(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 获取 CSRF token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/proxy", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	// 保存带密码的代理
	w = httptest.NewRecorder()
	body := "proxy_type=socks5&proxy_host=10.0.0.1&proxy_port=1080&proxy_username=user&proxy_password=secret123&proxy_remark=&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/settings/proxy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 验证密码已加密保存
	var setting model.SystemSetting
	srv.db.Where("key = ? AND is_sensitive = ?", "proxy_password", true).First(&setting)

	if setting.Value == "secret123" {
		t.Error("proxy_password 不应明文保存")
	}
	if setting.Value == "" {
		t.Error("proxy_password 不应为空")
	}
	if !setting.IsSensitive {
		t.Error("proxy_password 应标记为敏感")
	}
}

func TestProxyHandler_GetSettings_PasswordNotExposed(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 先保存一个带密码的代理
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/proxy", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_csrf" {
			csrfCookie = cookie.Value
		}
	}

	w = httptest.NewRecorder()
	body := "proxy_type=socks5&proxy_host=10.0.0.1&proxy_port=1080&proxy_username=user&proxy_password=secret123&proxy_remark=&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/settings/proxy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 获取页面检查密码不暴露
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/settings/proxy", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	pageBody := w.Body.String()
	if strings.Contains(pageBody, "secret123") {
		t.Error("页面不应暴露 proxy password 明文")
	}

	// 检查数据库中密码不为明文
	var setting model.SystemSetting
	srv.db.Where("key = ?", "proxy_password").First(&setting)
	if setting.Value == "secret123" {
		t.Error("数据库不应存储明文 proxy password")
	}
}

func TestProxyHandler_PostSettings_NoCSRF_Returns403(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	body := "proxy_type=none&proxy_host=&proxy_port=&proxy_username=&proxy_password=&proxy_remark="
	req, _ := http.NewRequest("POST", "/settings/proxy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("期望 403，实际=%d", w.Code)
	}
}

func TestProxyHandler_PageContainsProxyForm(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/proxy", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	mustContain := []string{
		"API 网络代理",
		"proxy_type",
		"proxy_host",
		"proxy_port",
		"HTTPS",
		"SOCKS5",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("代理页面缺少 %q", s)
		}
	}
}
