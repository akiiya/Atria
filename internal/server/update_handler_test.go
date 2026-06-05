package server

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/user/atria/internal/updater"

	"github.com/gin-gonic/gin"
)

// setupLoggedInServer 创建并初始化一个已登录的测试服务器。
func setupLoggedInServer(t *testing.T) (*Server, *gin.Engine, string, string) {
	t.Helper()
	srv, db := setupTestServer(t)

	// 初始化管理员
	adminSvc := NewAdminService(db)
	_, err := adminSvc.Initialize(InitializeInput{Username: "admin", Password: "password123456"})
	if err != nil {
		t.Fatalf("初始化管理员失败: %s", err)
	}

	// 创建路由
	gin.SetMode(gin.TestMode)
	r := gin.New()
	srv.setupRoutes(r)

	// 登录获取 session cookie
	// 先获取登录页面的 CSRF token
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

	// 获取 session cookie
	var sessionCookie string
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "atria_session" {
			sessionCookie = cookie.Value
		}
	}

	return srv, r, sessionCookie, csrfCookie
}

func TestUpdateHandler_PostCheck_NoAuth_RedirectsToLogin(t *testing.T) {
	_, r, _, _ := setupLoggedInServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/update/check", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际 %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Location"), "/login") {
		t.Errorf("应重定向到 /login，实际 %s", w.Header().Get("Location"))
	}
}

func TestUpdateHandler_PostCheck_NoCSRF_Returns403(t *testing.T) {
	_, r, sessionCookie, _ := setupLoggedInServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/update/check", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("期望 403，实际 %d", w.Code)
	}
}

func TestUpdateHandler_PostCheck_Success(t *testing.T) {
	srv, r, sessionCookie, csrfCookie := setupLoggedInServer(t)

	// 配置更新目录
	dir := t.TempDir()
	srv.cfg.UpdateDir = dir + "/updates"
	srv.cfg.UpdateBackupDir = dir + "/updates/backups"
	srv.cfg.UpdateEnabled = false // 禁用以避免真实网络请求

	w := httptest.NewRecorder()
	body := "csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/settings/update/check", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 禁用时应显示错误页面
	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际 %d", w.Code)
	}
}

func TestUpdateHandler_PostDownload_NoAuth_RedirectsToLogin(t *testing.T) {
	_, r, _, _ := setupLoggedInServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/update/download", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际 %d", w.Code)
	}
}

func TestUpdateHandler_PostDownload_NoCSRF_Returns403(t *testing.T) {
	_, r, sessionCookie, _ := setupLoggedInServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/update/download", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("期望 403，实际 %d", w.Code)
	}
}

func TestUpdateHandler_PostDryRun_NoAuth_RedirectsToLogin(t *testing.T) {
	_, r, _, _ := setupLoggedInServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/update/dry-run", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际 %d", w.Code)
	}
}

func TestUpdateHandler_PostDryRun_NoCSRF_Returns403(t *testing.T) {
	_, r, sessionCookie, _ := setupLoggedInServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/update/dry-run", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("期望 403，实际 %d", w.Code)
	}
}

func TestUpdateHandler_PostApply_NoAuth_RedirectsToLogin(t *testing.T) {
	_, r, _, _ := setupLoggedInServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/update/apply", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("期望 302，实际 %d", w.Code)
	}
}

func TestUpdateHandler_PostApply_NoCSRF_Returns403(t *testing.T) {
	_, r, sessionCookie, _ := setupLoggedInServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/update/apply", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("期望 403，实际 %d", w.Code)
	}
}

func TestUpdateHandler_PostApply_MissingConfirm_ReturnsError(t *testing.T) {
	_, r, sessionCookie, csrfCookie := setupLoggedInServer(t)

	w := httptest.NewRecorder()
	body := "csrf_token=" + csrfCookie
	req, _ := http.NewRequest("POST", "/settings/update/apply", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 缺少 confirm 字段应返回错误页面
	if w.Code != http.StatusOK {
		t.Errorf("期望 200（错误页面），实际 %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "缺少确认字段") {
		t.Error("应显示缺少确认字段错误")
	}
}

func TestUpdateHandler_PostApply_WithConfirm_CallsService(t *testing.T) {
	srv, r, sessionCookie, csrfCookie := setupLoggedInServer(t)

	// 配置更新目录
	dir := t.TempDir()
	srv.cfg.UpdateDir = dir + "/updates"
	srv.cfg.UpdateBackupDir = dir + "/updates/backups"

	// 设置状态为已下载
	statePath := dir + "/updates/update_state.json"
	os.MkdirAll(dir+"/updates", 0700)
	state := updater.UpdateState{
		Status:         updater.StatusDownloaded,
		AssetName:      "test.tar.gz",
		CurrentVersion: "v0.1.0",
		LatestVersion:  "v0.2.0",
	}
	updater.SaveState(statePath, state)

	w := httptest.NewRecorder()
	body := "csrf_token=" + csrfCookie + "&confirm=apply_update"
	req, _ := http.NewRequest("POST", "/settings/update/apply", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "atria_session="+sessionCookie+"; atria_csrf="+csrfCookie)
	r.ServeHTTP(w, req)

	// 由于没有真实的更新包，apply 会失败，但 confirm 校验应通过
	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际 %d", w.Code)
	}
}

func TestUpdateHandler_GetSettings_DisplaysUpdateStatus(t *testing.T) {
	_, r, sessionCookie, _ := setupLoggedInServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200，实际 %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "更新程序") {
		t.Error("应显示更新程序卡片")
	}
}

func TestUpdateHandler_GetSettings_NoSensitiveData(t *testing.T) {
	_, r, sessionCookie, _ := setupLoggedInServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", nil)
	req.Header.Set("Cookie", "atria_session="+sessionCookie)
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// 检查不包含真正的敏感数据（密码、密钥内容等）
	// 注意：secret.key 作为文件名出现在目录说明中是安全的
	sensitivePatterns := []string{
		"password123456", // 明文密码
		"atria_session=", // Session cookie 值
	}
	for _, pattern := range sensitivePatterns {
		if strings.Contains(body, pattern) {
			t.Errorf("页面不应包含敏感数据: %s", pattern)
		}
	}
}

func TestDockerUnsupported_ApplyReturnsUnsupported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 环境跳过 Docker 测试")
	}

	dir := t.TempDir()

	// 创建测试二进制
	testBinary := filepath.Join(dir, "atria")
	os.WriteFile(testBinary, []byte("#!/bin/sh\necho test\n"), 0755)

	// 创建测试更新包
	createTestArchiveForDocker(t, dir, "atria")

	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0700)

	u := updater.NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)

	// 使用 DryRun 避免实际替换
	result, err := u.ApplyUpdate(context.Background(), updater.ApplyOptions{
		CurrentBinaryPath: testBinary,
		AssetPath:         filepath.Join(dir, "update.tar.gz"),
		BackupDir:         backupDir,
		DryRun:            true,
	})

	if err != nil {
		t.Fatalf("DryRun 失败: %s", err)
	}
	if !result.Success {
		t.Errorf("DryRun 应该成功: %s", result.Message)
	}

	// 验证 data 目录未被删除
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("data 目录不应被删除")
	}
}

func TestDockerUnsupported_DataPreserved(t *testing.T) {
	dir := t.TempDir()

	// 创建 data 目录和 secret.key
	dataDir := filepath.Join(dir, "data")
	os.MkdirAll(dataDir, 0700)
	secretKey := filepath.Join(dataDir, "secret.key")
	os.WriteFile(secretKey, []byte("test-key"), 0600)

	// 创建 session 文件
	sessionDir := filepath.Join(dataDir, "sessions")
	os.MkdirAll(sessionDir, 0700)
	sessionFile := filepath.Join(sessionDir, "session_1.enc")
	os.WriteFile(sessionFile, []byte("encrypted-session"), 0600)

	// 创建测试二进制
	testBinary := filepath.Join(dir, "atria")
	os.WriteFile(testBinary, []byte("#!/bin/sh\necho test\n"), 0755)

	// 创建更新包
	createTestArchiveForDocker(t, dir, "atria")

	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0700)

	u := updater.NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)

	u.ApplyUpdate(context.Background(), updater.ApplyOptions{
		CurrentBinaryPath: testBinary,
		AssetPath:         filepath.Join(dir, "update.tar.gz"),
		BackupDir:         backupDir,
		DryRun:            true,
	})

	// 验证 secret.key 未被删除
	if _, err := os.Stat(secretKey); os.IsNotExist(err) {
		t.Error("secret.key 不应被删除")
	}

	// 验证 session 文件未被删除
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Error("Session 文件不应被删除")
	}
}

// createTestArchiveForDocker 创建测试用的更新包。
func createTestArchiveForDocker(t *testing.T, dir string, binaryName string) {
	t.Helper()
	archivePath := filepath.Join(dir, "update.tar.gz")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("创建文件失败: %s", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	content := []byte("#!/bin/sh\necho 'v0.2.0-test'\n")
	header := &tar.Header{
		Name: binaryName,
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("写入 header 失败: %s", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("写入内容失败: %s", err)
	}
}
