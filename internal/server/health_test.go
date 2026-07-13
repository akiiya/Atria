package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/user/atria/internal/model"
)

func TestHealthz_BackwardCompatible(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际=%d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析 JSON 失败: %s", err)
	}

	if body["status"] != "ok" {
		t.Errorf("status 应为 ok，实际=%v", body["status"])
	}
	if body["service"] != "atria" {
		t.Errorf("service 应为 atria，实际=%v", body["service"])
	}
	if body["version"] == nil {
		t.Error("version 字段不应为空")
	}
}

func TestHealthz_NoAuthRequired(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("healthz 应无需认证返回 200，实际=%d", w.Code)
	}
}

func TestHealthz_ReturnsUptime(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)

	if body["uptime"] == nil {
		t.Error("uptime 字段不应为空")
	}
	if body["uptime_seconds"] == nil {
		t.Error("uptime_seconds 字段不应为空")
	}
}

func TestHealthz_ReturnsMigrationVersion(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)

	if body["migration_version"] == nil {
		t.Error("migration_version 字段不应为空")
	}
}

func TestHealthz_AccountsSummary_Empty(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)

	accounts, ok := body["accounts"].(map[string]any)
	if !ok {
		t.Fatal("accounts 字段应为对象")
	}

	if accounts["total"].(float64) != 0 {
		t.Errorf("无账号时 total 应为 0，实际=%v", accounts["total"])
	}
}

func TestHealthz_AccountsSummary_WithAccounts(t *testing.T) {
	r, srv := setupTestRouter(t)

	createTestAccount(t, srv.db, "User 1", "user1", model.TelegramAccountStatusActive)
	createTestAccount(t, srv.db, "User 2", "user2", model.TelegramAccountStatusActive)
	createTestAccount(t, srv.db, "Banned User", "banned", model.TelegramAccountStatusBanned)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)

	accounts, ok := body["accounts"].(map[string]any)
	if !ok {
		t.Fatal("accounts 字段应为对象")
	}

	if accounts["total"].(float64) != 3 {
		t.Errorf("total 应为 3，实际=%v", accounts["total"])
	}
	if accounts["stopped"].(float64) != 3 {
		t.Errorf("stopped 应为 3，实际=%v", accounts["stopped"])
	}
}

func TestHealthz_DatabaseField_OK(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)

	dbField, ok := body["database"].(map[string]any)
	if !ok {
		t.Fatal("database 字段应为对象")
	}

	if dbField["status"] != "ok" {
		t.Errorf("正常数据库时 database.status 应为 ok，实际=%v", dbField["status"])
	}
}

func TestHealthz_DoesNotLeakSensitiveData(t *testing.T) {
	r, srv := setupTestRouter(t)

	createTestAccount(t, srv.db, "Test User", "test_user", model.TelegramAccountStatusActive)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	body := strings.ToLower(w.Body.String())

	sensitiveTerms := []string{
		"api_hash", "session_data", "proxy_password",
		"secret", "encrypted", "session_file_path",
	}
	for _, s := range sensitiveTerms {
		if strings.Contains(body, s) {
			t.Errorf("healthz 响应不应包含敏感数据 %q", s)
		}
	}
}

func TestHealthz_AllFieldsPresent(t *testing.T) {
	r, _ := setupTestRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析 JSON 失败: %s", err)
	}

	requiredFields := []string{
		"status", "service", "version",
		"uptime", "uptime_seconds",
		"database", "initialized",
		"migration_version", "accounts",
	}
	for _, field := range requiredFields {
		if body[field] == nil {
			t.Errorf("缺少必需字段 %q", field)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{"zero", 0, "0s"},
		{"seconds only", 45 * time.Second, "45s"},
		{"minutes and seconds", 2*time.Minute + 30*time.Second, "2m30s"},
		{"hours minutes seconds", 1*time.Hour + 2*time.Minute + 3*time.Second, "1h2m3s"},
		{"large", 25*time.Hour + 59*time.Minute + 59*time.Second, "25h59m59s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
