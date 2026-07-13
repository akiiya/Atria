package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/atria/internal/model"
)

// ===== 参数校验边界测试 =====

func TestChatSend_EmptyPeerRef(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 创建账号并选为当前账号
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

	// POST 到 /api/chats//messages（peer_ref 为空路径段）
	// gin 会将 // 清理为 /，导致路由不匹配返回 404
	w = httptest.NewRecorder()
	reqBody := "text=hello&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/api/chats//messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 不应返回成功响应
	bodyStr := w.Body.String()
	if w.Code == http.StatusOK && strings.Contains(bodyStr, `"ok":true`) {
		t.Errorf("empty peer_ref should return an error, got success (status=%d body=%s)", w.Code, bodyStr)
	}
}

func TestChatSend_EmptyText(t *testing.T) {
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

	// 发送空文本消息
	w = httptest.NewRecorder()
	reqBody := "text=&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/api/chats/u_123/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"text_empty"`) && !strings.Contains(bodyStr, "不能为空") {
		t.Errorf("empty text should return text_empty error, got: %s", bodyStr)
	}
}

func TestChatSend_TextTooLong(t *testing.T) {
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

	// 发送超过 4096 字符的文本
	longText := strings.Repeat("a", 4097)
	w = httptest.NewRecorder()
	reqBody := "text=" + longText + "&csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/api/chats/u_123/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, `"text_too_long"`) && !strings.Contains(bodyStr, "4096") {
		t.Errorf("text > 4096 chars should return text_too_long, got: %s", bodyStr)
	}
}

func TestChatRead_EmptyPeerRef(t *testing.T) {
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

	// POST 到 /api/chats//read（peer_ref 为空路径段）
	w = httptest.NewRecorder()
	reqBody := "csrf_token=" + csrfCookie
	req, _ = http.NewRequest("POST", "/api/chats//read", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 不应返回成功响应
	bodyStr := w.Body.String()
	if w.Code == http.StatusOK && strings.Contains(bodyStr, `"ok":true`) {
		t.Errorf("empty peer_ref should return an error, got success (status=%d body=%s)", w.Code, bodyStr)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建账号以避免 no_current_account 错误
	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// GET /api/search/messages 不带 q 参数
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/search/messages", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"ok":true`) {
		t.Errorf("empty query should return ok:true, got: %s", body)
	}
	if !strings.Contains(body, `"total":0`) {
		t.Errorf("empty query should return total:0, got: %s", body)
	}
}
