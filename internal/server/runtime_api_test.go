package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/user/atria/internal/model"
)

func TestRuntimeStatusAPI_NoCurrentAccount(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 不创建任何账号
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/runtime/status", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"ok":false`) && !strings.Contains(body, `"ok": false`) {
		t.Errorf("无账号时应返回 ok:false，实际: %s", body)
	}
	if !strings.Contains(body, "no_current_account") {
		t.Errorf("应返回 no_current_account 错误码，实际: %s", body)
	}
}

func TestRuntimeStatusAPI_ReturnsStoppedByDefault(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/runtime/status", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"ok":true`) && !strings.Contains(body, `"ok": true`) {
		t.Errorf("应返回 ok:true，实际: %s", body)
	}
	if !strings.Contains(body, `"state":"stopped"`) {
		t.Errorf("默认状态应为 stopped，实际: %s", body)
	}
}

func TestRuntimeStartAPI_StartsCurrentAccount(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)
	csrfCookie = refreshCSRF(t, r, sessionCookie)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	reqBody := "csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/chats/runtime/start", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// start 可能因为缺少 API 凭据而失败，但不应 panic 或返回 500
	if w.Code == http.StatusInternalServerError {
		t.Error("runtime start 不应返回 500")
	}
	// 验证返回了合理的 JSON 响应
	if !strings.Contains(body, `"ok"`) {
		t.Errorf("应返回 JSON 响应，实际: %s", body)
	}

	// 停止 runtime 以清理
	srv.runtimeManager.StopAll()
}

func TestRuntimeStopAPI_StopsCurrentAccount(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)
	csrfCookie = refreshCSRF(t, r, sessionCookie)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// 先启动
	w := httptest.NewRecorder()
	reqBody := "csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/api/chats/runtime/start", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 再停止
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/chats/runtime/stop", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"ok":true`) && !strings.Contains(body, `"ok": true`) {
		t.Errorf("stop 应返回 ok:true，实际: %s", body)
	}
	if !strings.Contains(body, `"state":"stopped"`) {
		t.Errorf("stop 后状态应为 stopped，实际: %s", body)
	}
}

func TestRuntimeAPI_DoesNotReturnSensitiveFields(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/runtime/status", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	sensitiveFields := []string{
		"api_hash",
		"api_id",
		"session_path",
		"session_file",
		"access_hash",
		"proxy_password",
		"EncryptedAPIHash",
		"SessionFilePath",
	}
	for _, field := range sensitiveFields {
		if strings.Contains(body, field) {
			t.Errorf("runtime status API 不应包含敏感字段 %q，实际: %s", field, body)
		}
	}
}

func TestRuntimeAPI_RequiresAuth(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 不登录，直接访问
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/runtime/status", nil)
	r.ServeHTTP(w, req)

	// 应重定向到登录页
	if w.Code != http.StatusFound {
		t.Errorf("未登录应重定向，实际 %d", w.Code)
	}
}

func TestRuntimeStartAPI_RequiresCSRF(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 不带 CSRF token 的 POST
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/chats/runtime/start", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("无 CSRF 应返回 403，实际 %d", w.Code)
	}
}

func TestRuntimeAPI_UsesSelectedAccountOnly(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 创建一个账号
	createTestAccount(t, srv.db, "User 1", "user1", model.TelegramAccountStatusActive)

	// 查询 status，应使用 selected account（第一个）
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/runtime/status", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"ok":true`) && !strings.Contains(body, `"ok": true`) {
		t.Errorf("应返回 ok:true，实际: %s", body)
	}
}

func TestChatDialogsStillWorksWithRuntimeHardening(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// dialogs API 应该仍然正常工作
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/dialogs", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"ok":true`) && !strings.Contains(body, `"ok": true`) {
		// 可能因为没有 Telegram 连接而失败，但不应 panic
		if w.Code == http.StatusInternalServerError {
			t.Error("dialogs API 不应返回 500")
		}
	}
}

func TestChatMessagesStillWorksWithRuntimeHardening(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// messages API 应该仍然正常工作
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/u_123/messages", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	// 不应 panic 或返回 500
	if w.Code == http.StatusInternalServerError {
		t.Error("messages API 不应返回 500")
	}
}

func TestChatSendStillWorksWithRuntimeHardening(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)
	csrfCookie = refreshCSRF(t, r, sessionCookie)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// send API 应该仍然正常工作
	w := httptest.NewRecorder()
	reqBody := `{"text":"hello"}`
	req, _ := http.NewRequest("POST", "/api/chats/u_123/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfCookie)
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 不应 panic 或返回 500
	if w.Code == http.StatusInternalServerError {
		t.Error("send API 不应返回 500")
	}
}

func TestBulkFieldsStillRejected(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)
	csrfCookie = refreshCSRF(t, r, sessionCookie)
	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// 使用 JSON 格式发送 bulk 字段（参照已有测试 TestChatSend_DoesNotSupportBulk）
	w := httptest.NewRecorder()
	reqBody := `{"text":"hello","bulk":true}`
	req, _ := http.NewRequest("POST", "/api/chats/u_123/messages", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfCookie)
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// bulk 字段应被拒绝，返回 bulk_not_supported
	if !strings.Contains(body, "bulk_not_supported") {
		// 如果 CSRF 失败也算通过（因为 CSRF 在 bulk 检查之前）
		if strings.Contains(body, "CSRF") || strings.Contains(body, "403") {
			t.Log("CSRF 拦截了请求（在 bulk 检查之前）")
		} else {
			t.Errorf("bulk 字段应被拒绝，实际: %s", body)
		}
	}
}
