package updater

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "update_state.json")

	now := time.Now()
	state := UpdateState{
		Status:         StatusUpdateAvailable,
		CurrentVersion: "v0.1.0",
		LatestVersion:  "v0.2.0",
		Message:        "有新版本可用",
		CheckedAt:      &now,
	}

	if err := SaveState(statePath, state); err != nil {
		t.Fatalf("保存状态失败: %s", err)
	}

	loaded, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("加载状态失败: %s", err)
	}

	if loaded.Status != StatusUpdateAvailable {
		t.Errorf("状态不匹配，期望 %s，实际 %s", StatusUpdateAvailable, loaded.Status)
	}
	if loaded.CurrentVersion != "v0.1.0" {
		t.Errorf("当前版本不匹配，期望 v0.1.0，实际 %s", loaded.CurrentVersion)
	}
	if loaded.LatestVersion != "v0.2.0" {
		t.Errorf("最新版本不匹配，期望 v0.2.0，实际 %s", loaded.LatestVersion)
	}
}

func TestLoadState_FileNotExist(t *testing.T) {
	state, err := LoadState("/nonexistent/path/state.json")
	if err != nil {
		t.Fatalf("加载不存在的文件应该返回默认状态: %s", err)
	}
	if state.Status != StatusIdle {
		t.Errorf("默认状态应为 idle，实际 %s", state.Status)
	}
}

func TestSaveState_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "subdir", "update_state.json")

	state := UpdateState{
		Status:         StatusIdle,
		CurrentVersion: "v0.1.0",
	}

	if err := SaveState(statePath, state); err != nil {
		t.Fatalf("保存状态应该创建目录: %s", err)
	}

	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Error("状态文件应该存在")
	}
}

func TestSaveState_UsesTmpRename(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "update_state.json")

	state := UpdateState{
		Status:         StatusIdle,
		CurrentVersion: "v0.1.0",
	}

	if err := SaveState(statePath, state); err != nil {
		t.Fatalf("保存失败: %s", err)
	}

	// 临时文件不应该存在
	tmpPath := statePath + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("临时文件不应该存在")
	}
}

func TestSaveState_NoSensitiveData(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "update_state.json")

	state := UpdateState{
		Status:         StatusUpdateAvailable,
		CurrentVersion: "v0.1.0",
		LatestVersion:  "v0.2.0",
	}

	if err := SaveState(statePath, state); err != nil {
		t.Fatalf("保存失败: %s", err)
	}

	content, _ := os.ReadFile(statePath)
	contentStr := string(content)

	// 不应包含敏感信息
	sensitiveKeys := []string{"token", "secret", "session", "api_hash", "password", "phone"}
	for _, key := range sensitiveKeys {
		if contains(contentStr, key) {
			t.Errorf("状态文件不应包含敏感字段: %s", key)
		}
	}
}

func TestLoadState_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "update_state.json")

	// 写入损坏的 JSON
	os.WriteFile(statePath, []byte("{invalid json"), 0600)

	_, err := LoadState(statePath)
	if err == nil {
		t.Error("损坏的 JSON 应该返回错误")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
