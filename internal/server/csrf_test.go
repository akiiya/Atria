package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ===== CSRF 边界测试 =====

func TestCSRF_RejectsPostWithoutToken(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化并登录以获取有效 session
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// POST 到受保护端点，不带 CSRF token（无 header、无 form field、无 cookie）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/logout", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("POST without CSRF token should return 403, got %d", w.Code)
	}
}

func TestCSRF_AllowsGetWithoutToken(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化并登录以获取有效 session
	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// GET 请求到受保护端点，不带 CSRF cookie
	// CSRF 中间件对 GET/HEAD/OPTIONS 跳过校验
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Error("GET requests should not be blocked by CSRF validation")
	}
}

func TestCSRF_RejectsMismatchedToken(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 初始化并登录，获取合法的 CSRF cookie
	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// POST 请求中 form field 的 csrf_token 与 cookie 中的 atria_csrf 不匹配
	w := httptest.NewRecorder()
	body := "csrf_token=wrong-token-does-not-match-cookie"
	req, _ := http.NewRequest("POST", "/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("POST with mismatched CSRF token should return 403, got %d", w.Code)
	}
}
