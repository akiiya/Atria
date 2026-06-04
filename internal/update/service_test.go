package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/user/atria/internal/config"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/updater"

	"gorm.io/gorm"
)

// mockUpdater 是用于测试的 mock updater。
type mockUpdater struct {
	checkLatestFunc func(ctx context.Context, opts updater.CheckOptions) (*updater.ReleaseInfo, error)
	selectAssetFunc func(release updater.ReleaseInfo, goos, goarch string) (*updater.AssetInfo, error)
	downloadFunc    func(ctx context.Context, asset updater.AssetInfo, destDir string) (string, error)
	applyFunc       func(ctx context.Context, opts updater.ApplyOptions) (*updater.ApplyResult, error)
	state           updater.UpdateState
}

func (m *mockUpdater) CheckLatest(ctx context.Context, opts updater.CheckOptions) (*updater.ReleaseInfo, error) {
	if m.checkLatestFunc != nil {
		return m.checkLatestFunc(ctx, opts)
	}
	return &updater.ReleaseInfo{TagName: "v0.2.0"}, nil
}

func (m *mockUpdater) SelectAsset(release updater.ReleaseInfo, goos, goarch string) (*updater.AssetInfo, error) {
	if m.selectAssetFunc != nil {
		return m.selectAssetFunc(release, goos, goarch)
	}
	return &updater.AssetInfo{Name: "atria_linux_amd64.tar.gz"}, nil
}

func (m *mockUpdater) DownloadAsset(ctx context.Context, asset updater.AssetInfo, destDir string) (string, error) {
	if m.downloadFunc != nil {
		return m.downloadFunc(ctx, asset, destDir)
	}
	return destDir + "/atria_linux_amd64.tar.gz", nil
}

func (m *mockUpdater) VerifyChecksum(filePath string, expected string) error {
	return nil
}

func (m *mockUpdater) ApplyUpdate(ctx context.Context, opts updater.ApplyOptions) (*updater.ApplyResult, error) {
	if m.applyFunc != nil {
		return m.applyFunc(ctx, opts)
	}
	return &updater.ApplyResult{Success: true, NeedRestart: true, Message: "更新成功"}, nil
}

func (m *mockUpdater) GetState() updater.UpdateState {
	return m.state
}

func (m *mockUpdater) SetState(state updater.UpdateState) {
	m.state = state
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("创建测试数据库失败: %s", err)
	}
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("数据库迁移失败: %s", err)
	}
	return db
}

func setupTestConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	return &config.Config{
		UpdateEnabled:         true,
		UpdateRepo:            "test/repo",
		UpdateDir:             dir + "/updates",
		UpdateBackupDir:       dir + "/updates/backups",
		UpdateTimeout:         60 * time.Second,
		UpdateAllowPrerelease: true,
		UpdateRequireChecksum: false,
	}
}

func TestService_CheckUpdate_Success(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)

	mock := &mockUpdater{
		state: updater.UpdateState{
			Status:         updater.StatusUpdateAvailable,
			CurrentVersion: "v0.1.0",
			LatestVersion:  "v0.2.0",
		},
	}

	svc := &Service{db: db, cfg: cfg, updater: mock}

	release, err := svc.CheckUpdate(context.Background(), 1, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("检查更新失败: %s", err)
	}

	if release.TagName != "v0.2.0" {
		t.Errorf("版本不匹配，期望 v0.2.0，实际 %s", release.TagName)
	}
}

func TestService_CheckUpdate_NoUpdate(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)

	mock := &mockUpdater{
		state: updater.UpdateState{
			Status:         updater.StatusUpToDate,
			CurrentVersion: "v0.2.0",
		},
	}

	svc := &Service{db: db, cfg: cfg, updater: mock}
	svc.state = mock.state

	_, err := svc.CheckUpdate(context.Background(), 1, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("检查更新失败: %s", err)
	}

	if svc.GetState().Status != updater.StatusUpToDate {
		t.Errorf("状态应为 up_to_date，实际 %s", svc.GetState().Status)
	}
}

func TestService_CheckUpdate_Disabled(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)
	cfg.UpdateEnabled = false

	mock := &mockUpdater{}
	svc := &Service{db: db, cfg: cfg, updater: mock}

	_, err := svc.CheckUpdate(context.Background(), 1, "127.0.0.1", "test")
	if err == nil {
		t.Error("禁用时应该返回错误")
	}
}

func TestService_GetState(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)

	mock := &mockUpdater{
		state: updater.UpdateState{
			Status:         updater.StatusIdle,
			CurrentVersion: "v0.1.0",
		},
	}

	svc := &Service{db: db, cfg: cfg, updater: mock, state: mock.state}

	state := svc.GetState()
	if state.Status != updater.StatusIdle {
		t.Errorf("状态应为 idle，实际 %s", state.Status)
	}
}

func TestService_CheckUpdate_AuditLogged(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v0.2.0","assets":[]}`))
	}))
	defer server.Close()

	cfg.UpdateCheckURL = server.URL

	mock := &mockUpdater{
		state: updater.UpdateState{
			Status:         updater.StatusUpdateAvailable,
			CurrentVersion: "v0.1.0",
			LatestVersion:  "v0.2.0",
		},
	}

	svc := &Service{db: db, cfg: cfg, updater: mock}

	svc.CheckUpdate(context.Background(), 1, "127.0.0.1", "test")

	var count int64
	db.Model(&model.AuditLog{}).Where("action = ?", "system.update_available").Count(&count)
	if count == 0 {
		t.Error("应写入 system.update_available 审计日志")
	}
}

func TestService_ApplyUpdate_DryRun(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)

	mock := &mockUpdater{
		state: updater.UpdateState{
			Status:        updater.StatusDownloaded,
			AssetName:     "atria_linux_amd64.tar.gz",
			LatestVersion: "v0.2.0",
		},
		applyFunc: func(ctx context.Context, opts updater.ApplyOptions) (*updater.ApplyResult, error) {
			if !opts.DryRun {
				t.Error("应该是 DryRun 模式")
			}
			return &updater.ApplyResult{Success: true, Message: "DryRun 通过"}, nil
		},
	}

	svc := &Service{db: db, cfg: cfg, updater: mock}
	svc.state = mock.state

	result, err := svc.ApplyUpdate(context.Background(), 1, "127.0.0.1", "test", true)
	if err != nil {
		t.Fatalf("DryRun 失败: %s", err)
	}
	if !result.Success {
		t.Error("DryRun 应该成功")
	}
}

func TestService_ApplyUpdate_AuditLogged(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)

	mock := &mockUpdater{
		state: updater.UpdateState{
			Status:        updater.StatusDownloaded,
			AssetName:     "atria_linux_amd64.tar.gz",
			LatestVersion: "v0.2.0",
		},
	}

	svc := &Service{db: db, cfg: cfg, updater: mock}
	svc.state = mock.state

	svc.ApplyUpdate(context.Background(), 1, "127.0.0.1", "test", true)

	var count int64
	db.Model(&model.AuditLog{}).Where("action = ?", "system.update_dry_run").Count(&count)
	if count == 0 {
		t.Error("应写入 system.update_dry_run 审计日志")
	}
}

func TestService_ApplyUpdate_FailedAudit(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)

	mock := &mockUpdater{
		state: updater.UpdateState{
			Status:        updater.StatusDownloaded,
			AssetName:     "atria_linux_amd64.tar.gz",
			LatestVersion: "v0.2.0",
		},
		applyFunc: func(ctx context.Context, opts updater.ApplyOptions) (*updater.ApplyResult, error) {
			return &updater.ApplyResult{Success: false, Message: "替换失败"}, nil
		},
	}

	svc := &Service{db: db, cfg: cfg, updater: mock}
	svc.state = mock.state

	svc.ApplyUpdate(context.Background(), 1, "127.0.0.1", "test", false)

	var count int64
	db.Model(&model.AuditLog{}).Where("action = ?", "system.update_apply_failed").Count(&count)
	if count == 0 {
		t.Error("应写入 system.update_apply_failed 审计日志")
	}
}

func TestService_AuditMetadata_NoSensitiveData(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)

	mock := &mockUpdater{
		state: updater.UpdateState{
			Status:         updater.StatusUpdateAvailable,
			CurrentVersion: "v0.1.0",
			LatestVersion:  "v0.2.0",
		},
	}

	svc := &Service{db: db, cfg: cfg, updater: mock}

	svc.CheckUpdate(context.Background(), 1, "127.0.0.1", "test")

	var logs []model.AuditLog
	db.Find(&logs)

	sensitiveKeys := []string{"token", "secret", "session", "api_hash", "phone", "password"}
	for _, log := range logs {
		for _, key := range sensitiveKeys {
			if contains(log.MetadataJSON, key) {
				t.Errorf("审计 metadata 不应包含敏感字段 %s", key)
			}
		}
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
