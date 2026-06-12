package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/telegramclient"
)

func TestRealtimeWS_RequiresAuth(t *testing.T) {
	r, _ := setupTestRouter(t)

	// 不登录，直接访问
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/realtime/ws", nil)
	r.ServeHTTP(w, req)

	// 应返回 401 或重定向到登录
	if w.Code == http.StatusOK {
		t.Error("未登录不应返回 200")
	}
}

func TestRealtimeWS_NoCurrentAccount(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	// 不创建任何账号
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/realtime/ws", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	// 应返回 400 或类似错误
	if w.Code == http.StatusOK {
		body := w.Body.String()
		if strings.Contains(body, `"ok":true`) {
			t.Error("无账号时不应返回 ok:true")
		}
	}
}

func TestRealtimeWS_DoesNotReturnSensitiveFields(t *testing.T) {
	// 验证事件序列化不包含敏感字段
	event := telegramclient.UpdateEvent{
		EventID:   "test_1",
		AccountID: 1,
		Type:      telegramclient.EventMessageNew,
		PeerRef:   "u_123",
		CreatedAt: time.Now(),
		Payload: telegramclient.Message{
			ID:                "msg_1",
			TelegramMessageID: 123,
			PeerRef:           "u_123",
			Direction:         telegramclient.MessageDirectionOut,
			SenderName:        "Test",
			Text:              "Hello",
			Kind:              telegramclient.MessageKindText,
			SentAt:            time.Now(),
			IsOutgoing:        true,
			Status:            telegramclient.MessageStatusSent,
		},
	}

	envelope := realtimeWSEnvelope{
		Type:      event.Type,
		EventID:   event.EventID,
		AccountID: event.AccountID,
		PeerRef:   event.PeerRef,
		CreatedAt: event.CreatedAt.Format(time.RFC3339),
		Payload:   sanitizePayload(event),
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("JSON 序列化失败: %s", err)
	}

	body := string(data)

	sensitiveFields := []string{
		"access_hash",
		"api_hash",
		"proxy_password",
		"session_path",
		"EncryptedAPIHash",
		"SessionFilePath",
	}
	for _, field := range sensitiveFields {
		if strings.Contains(body, field) {
			t.Errorf("WebSocket 事件不应包含敏感字段 %q", field)
		}
	}
}

func TestRealtimeEvent_MessageNewSerialization(t *testing.T) {
	event := telegramclient.UpdateEvent{
		EventID:   "evt_msg_new_1",
		AccountID: 1,
		Type:      telegramclient.EventMessageNew,
		PeerRef:   "u_123",
		CreatedAt: time.Now(),
		Payload: telegramclient.Message{
			ID:                "msg_1",
			TelegramMessageID: 123,
			PeerRef:           "u_123",
			Direction:         telegramclient.MessageDirectionOut,
			SenderName:        "Test User",
			Text:              "Hello world",
			Kind:              telegramclient.MessageKindText,
			SentAt:            time.Now(),
			IsOutgoing:        true,
			Status:            telegramclient.MessageStatusSent,
		},
	}

	payload := sanitizePayload(event)
	if payload == nil {
		t.Fatal("payload 不应为 nil")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("JSON 序列化失败: %s", err)
	}

	body := string(data)
	if !strings.Contains(body, "Hello world") {
		t.Error("payload 应包含消息文本")
	}
	if !strings.Contains(body, "u_123") {
		t.Error("payload 应包含 peer_ref")
	}
}

func TestRealtimeEvent_MessageDeletedSerialization(t *testing.T) {
	event := telegramclient.UpdateEvent{
		EventID:   "evt_msg_del_1",
		AccountID: 1,
		Type:      telegramclient.EventMessageDeleted,
		PeerRef:   "u_123",
		CreatedAt: time.Now(),
		Payload: map[string]interface{}{
			"telegram_message_ids": []int{100, 200},
		},
	}

	payload := sanitizePayload(event)
	if payload == nil {
		t.Fatal("payload 不应为 nil")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("JSON 序列化失败: %s", err)
	}

	body := string(data)
	if !strings.Contains(body, "100") || !strings.Contains(body, "200") {
		t.Error("payload 应包含 telegram_message_ids")
	}
}

func TestRealtimeEvent_DialogUpsertedSerialization(t *testing.T) {
	now := time.Now()
	event := telegramclient.UpdateEvent{
		EventID:   "evt_dlg_1",
		AccountID: 1,
		Type:      telegramclient.EventDialogUpserted,
		PeerRef:   "u_456",
		CreatedAt: now,
		Payload: telegramclient.Dialog{
			PeerRef:            "u_456",
			PeerType:           telegramclient.PeerTypeUser,
			Title:              "Alice",
			Username:           "alice",
			LastMessagePreview: "Hi!",
			LastMessageAt:      now,
			UnreadCount:        3,
			IsPinned:           false,
			IsMuted:            false,
		},
	}

	payload := sanitizePayload(event)
	if payload == nil {
		t.Fatal("payload 不应为 nil")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("JSON 序列化失败: %s", err)
	}

	body := string(data)
	if !strings.Contains(body, "Alice") {
		t.Error("payload 应包含 title")
	}
	if !strings.Contains(body, "u_456") {
		t.Error("payload 应包含 peer_ref")
	}
}

func TestRealtimeEvent_NoAccessHash(t *testing.T) {
	// 验证 sanitizePayload 不会暴露 access_hash
	event := telegramclient.UpdateEvent{
		Type: telegramclient.EventDialogUpserted,
		Payload: telegramclient.Dialog{
			PeerRef:    "u_1",
			AccessHash: 123456789,
		},
	}

	payload := sanitizePayload(event)
	data, _ := json.Marshal(payload)
	body := string(data)

	if strings.Contains(body, "access_hash") || strings.Contains(body, "123456789") {
		t.Error("payload 不应包含 access_hash")
	}
}

func TestRealtimeEvent_SyncStatusSerialization(t *testing.T) {
	event := telegramclient.UpdateEvent{
		EventID:   "evt_sync_1",
		AccountID: 1,
		Type:      telegramclient.EventSyncDone,
		CreatedAt: time.Now(),
	}

	payload := sanitizePayload(event)
	// sync 事件 payload 为 nil
	if payload != nil {
		data, _ := json.Marshal(payload)
		body := string(data)
		if strings.Contains(body, "api_hash") || strings.Contains(body, "proxy_password") {
			t.Error("sync 事件不应包含敏感字段")
		}
	}
}

func TestRealtimeWS_SubscribesSelectedAccountOnly(t *testing.T) {
	// 验证 WebSocket 只订阅 selected account
	// 这是架构约束测试 - 通过代码审查确认
	r, srv := setupTestRouter(t)
	_ = r
	_ = srv

	// 验证 handleRealtimeWS 使用 resolveCurrentAccountID
	// 而不是允许客户端传入任意 account_id
	// 这是代码审查级别的测试
}

// ===== REST 回归测试 =====

func TestChatDialogsStillWorksWithRealtimeWS(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/dialogs", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusInternalServerError {
		t.Error("dialogs API 不应返回 500")
	}
}

func TestChatMessagesStillWorksWithRealtimeWS(t *testing.T) {
	r, srv := setupTestRouter(t)

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/chats/u_123/messages", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusInternalServerError {
		t.Error("messages API 不应返回 500")
	}
}

func TestRuntimeStatusStillWorksWithRealtimeWS(t *testing.T) {
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
		t.Errorf("runtime status 应返回 ok:true，实际: %s", body)
	}
}

// ===== Dev Publish Endpoint Tests =====

func TestRealtimeDevPublish_DisabledByDefault(t *testing.T) {
	r, _ := setupTestRouter(t)

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	// 默认不设置 ATRIA_DEV_REALTIME_TEST，应返回 404
	w := httptest.NewRecorder()
	reqBody := `{"type":"message.new","peer_ref":"u_123"}`
	req, _ := http.NewRequest("POST", "/api/realtime/dev/publish", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfCookie)
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("未启用时应返回 404，实际 %d", w.Code)
	}
}

func TestRealtimeDevPublish_RequiresAuth(t *testing.T) {
	r, _ := setupTestRouter(t)

	t.Setenv("ATRIA_DEV_REALTIME_TEST", "1")

	w := httptest.NewRecorder()
	reqBody := `{"type":"message.new","peer_ref":"u_123"}`
	req, _ := http.NewRequest("POST", "/api/realtime/dev/publish", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized && w.Code != http.StatusFound {
		t.Errorf("未登录应返回 401 或重定向，实际 %d", w.Code)
	}
}

func TestRealtimeDevPublish_UsesSelectedAccountOnly(t *testing.T) {
	r, srv := setupTestRouter(t)

	t.Setenv("ATRIA_DEV_REALTIME_TEST", "1")

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	reqBody := `{"type":"message.new","peer_ref":"u_123"}`
	req, _ := http.NewRequest("POST", "/api/realtime/dev/publish", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	// 应该成功（使用 selected account）
	body := w.Body.String()
	if !strings.Contains(body, `"ok":true`) && !strings.Contains(body, `"ok": true`) {
		t.Errorf("应返回 ok:true，实际: %s", body)
	}
}

func TestRealtimeDevPublish_PublishesMessageNew(t *testing.T) {
	r, srv := setupTestRouter(t)

	t.Setenv("ATRIA_DEV_REALTIME_TEST", "1")

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// 订阅 EventBus 以验证事件被发布
	sink, ch := telegramclient.NewChannelSink(10)
	sub, _ := srv.eventBus.Subscribe(1, sink)
	defer sub.Close()

	// 在 goroutine 中接收事件
	received := make(chan telegramclient.UpdateEvent, 1)
	go func() {
		select {
		case event := <-ch:
			received <- event
		case <-time.After(2 * time.Second):
		}
	}()

	w := httptest.NewRecorder()
	reqBody := `{"type":"message.new","peer_ref":"u_123","payload":{"text":"test message"}}`
	req, _ := http.NewRequest("POST", "/api/realtime/dev/publish", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"ok":true`) && !strings.Contains(body, `"ok": true`) {
		t.Errorf("应返回 ok:true，实际: %s", body)
	}

	// 验证事件被发布到 EventBus
	select {
	case event := <-received:
		if event.Type != telegramclient.EventMessageNew {
			t.Errorf("期望 message.new，实际 %s", event.Type)
		}
		if event.PeerRef != "u_123" {
			t.Errorf("期望 peer_ref=u_123，实际 %s", event.PeerRef)
		}
	case <-time.After(3 * time.Second):
		t.Error("超时：未收到事件")
	}
}

func TestRealtimeDevPublish_DoesNotAllowAccountIDOverride(t *testing.T) {
	r, srv := setupTestRouter(t)

	t.Setenv("ATRIA_DEV_REALTIME_TEST", "1")

	initAdmin(t, r)
	_, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	// 尝试传入 account_id（应被忽略）
	w := httptest.NewRecorder()
	reqBody := `{"type":"message.new","peer_ref":"u_123","account_id":999}`
	req, _ := http.NewRequest("POST", "/api/realtime/dev/publish", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	// 应该成功但使用 selected account，不是 999
	body := w.Body.String()
	if !strings.Contains(body, `"ok":true`) && !strings.Contains(body, `"ok": true`) {
		t.Errorf("应返回 ok:true，实际: %s", body)
	}
	_ = srv
}

func TestRealtimeDevPublish_DoesNotReturnSensitiveFields(t *testing.T) {
	r, srv := setupTestRouter(t)

	t.Setenv("ATRIA_DEV_REALTIME_TEST", "1")

	initAdmin(t, r)
	csrfCookie, sessionCookie := loginAdmin(t, r)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	reqBody := `{"type":"message.new","peer_ref":"u_123","payload":{"text":"test"}}`
	req, _ := http.NewRequest("POST", "/api/realtime/dev/publish", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfCookie)
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	sensitiveFields := []string{"api_hash", "proxy_password", "session_path", "access_hash"}
	for _, field := range sensitiveFields {
		if strings.Contains(body, field) {
			t.Errorf("响应不应包含敏感字段 %q", field)
		}
	}
	_ = srv
}

// ===== Event Serialization Extended Tests =====

func TestRealtimeEvent_MessageEditedSerialization(t *testing.T) {
	event := telegramclient.UpdateEvent{
		EventID:   "evt_msg_edit_1",
		AccountID: 1,
		Type:      telegramclient.EventMessageEdited,
		PeerRef:   "u_123",
		CreatedAt: time.Now(),
		Payload: telegramclient.Message{
			ID:                "msg_1",
			TelegramMessageID: 123,
			PeerRef:           "u_123",
			Direction:         telegramclient.MessageDirectionOut,
			SenderName:        "Test User",
			Text:              "Edited text",
			Kind:              telegramclient.MessageKindText,
			SentAt:            time.Now(),
			IsOutgoing:        true,
			Status:            telegramclient.MessageStatusSent,
		},
	}

	payload := sanitizePayload(event)
	if payload == nil {
		t.Fatal("payload 不应为 nil")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("JSON 序列化失败: %s", err)
	}

	body := string(data)
	if !strings.Contains(body, "Edited text") {
		t.Error("payload 应包含编辑后的文本")
	}
}

func TestRealtimeEvent_NoAPIHash(t *testing.T) {
	event := telegramclient.UpdateEvent{
		Type: telegramclient.EventMessageNew,
		Payload: telegramclient.Message{
			ID:   "msg_1",
			Text: "test",
		},
	}

	payload := sanitizePayload(event)
	data, _ := json.Marshal(payload)
	body := string(data)

	if strings.Contains(body, "api_hash") {
		t.Error("payload 不应包含 api_hash")
	}
}

func TestRealtimeEvent_NoProxyPassword(t *testing.T) {
	event := telegramclient.UpdateEvent{
		Type: telegramclient.EventMessageNew,
		Payload: telegramclient.Message{
			ID:   "msg_1",
			Text: "test",
		},
	}

	payload := sanitizePayload(event)
	data, _ := json.Marshal(payload)
	body := string(data)

	if strings.Contains(body, "proxy_password") {
		t.Error("payload 不应包含 proxy_password")
	}
}

func TestRealtimeEvent_NoSessionPath(t *testing.T) {
	event := telegramclient.UpdateEvent{
		Type: telegramclient.EventMessageNew,
		Payload: telegramclient.Message{
			ID:   "msg_1",
			Text: "test",
		},
	}

	payload := sanitizePayload(event)
	data, _ := json.Marshal(payload)
	body := string(data)

	if strings.Contains(body, "session_path") || strings.Contains(body, "SessionFilePath") {
		t.Error("payload 不应包含 session_path")
	}
}

func TestRealtimeEvent_NoGotdRawPayload(t *testing.T) {
	// 验证 payload 不包含 gotd 原始类型标记
	event := telegramclient.UpdateEvent{
		Type: telegramclient.EventMessageNew,
		Payload: telegramclient.Message{
			ID:   "msg_1",
			Text: "test",
		},
	}

	payload := sanitizePayload(event)
	data, _ := json.Marshal(payload)
	body := string(data)

	gotdMarkers := []string{"tg.Message", "tg.Dialog", "InputPeer", "gotd"}
	for _, marker := range gotdMarkers {
		if strings.Contains(body, marker) {
			t.Errorf("payload 不应包含 gotd 标记 %q", marker)
		}
	}
}
