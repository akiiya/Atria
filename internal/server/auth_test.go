package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/user/atria/internal/auth"
)

// ===== 认证边界测试 =====

func TestAuth_RejectsUnauthenticatedAPI(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 不带 session cookie 访问 /api/me
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/me", nil)
	r.ServeHTTP(w, req)

	// auth 中间件对未认证请求返回 302 重定向到 /login（非 401 JSON）
	if w.Code != http.StatusFound {
		t.Errorf("GET /api/me without session should return 302 redirect, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "/login") {
		t.Errorf("should redirect to /login, got %s", location)
	}
}

func TestAuth_AllowsHealthz(t *testing.T) {
	r, _ := setupTestRouter(t)

	// /healthz 是公开端点，无需认证
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /healthz without session should return 200, got %d", w.Code)
	}

	// 响应应包含健康检查数据
	body := w.Body.String()
	if !strings.Contains(body, `"status"`) {
		t.Errorf("/healthz response should contain status field, got: %s", body)
	}
}

func TestAuth_RejectsExpiredSession(t *testing.T) {
	r, srv := setupTestRouter(t)

	// 初始化管理员（确保数据库就绪）
	initAdmin(t, r)

	// 创建一个已过期的 session token（TTL 为 -1 小时，即已过期）
	claims := auth.NewSessionClaims(1, "admin", -1*time.Hour)
	expiredToken, err := auth.EncodeSessionToken(srv.key, claims)
	if err != nil {
		t.Fatalf("failed to encode expired session token: %v", err)
	}

	// 用过期 session 访问受保护端点
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Cookie", "atria_session="+expiredToken)
	r.ServeHTTP(w, req)

	// auth 中间件应拒绝过期 session，重定向到 /login
	if w.Code != http.StatusFound {
		t.Errorf("expired session should redirect to login (302), got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "/login") {
		t.Errorf("should redirect to /login, got %s", location)
	}
}
