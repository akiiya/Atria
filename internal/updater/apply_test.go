package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// createTestBinary 创建一个测试用的假二进制文件。
func createTestBinary(t *testing.T, dir string, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	// 写入一个简单的脚本，让它响应 version 命令
	content := []byte("#!/bin/sh\necho 'v0.1.0-test'\n")
	if err := os.WriteFile(path, content, 0755); err != nil {
		t.Fatalf("创建测试二进制失败: %s", err)
	}
	return path
}

// createTestArchive 创建一个包含假二进制的 tar.gz 文件。
func createTestArchive(t *testing.T, dir string, binaryName string) string {
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

	// 添加二进制文件
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

	return archivePath
}

func TestApplyUpdate_DryRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 环境跳过 apply 测试")
	}

	dir := t.TempDir()

	// 创建当前二进制
	currentBinary := createTestBinary(t, dir, "atria")

	// 创建更新包
	archivePath := createTestArchive(t, dir, "atria")

	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0700)

	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)

	result, err := u.ApplyUpdate(context.Background(), ApplyOptions{
		CurrentBinaryPath: currentBinary,
		AssetPath:         archivePath,
		BackupDir:         backupDir,
		DryRun:            true,
	})
	if err != nil {
		t.Fatalf("DryRun 失败: %s", err)
	}
	if !result.Success {
		t.Errorf("DryRun 应该成功: %s", result.Message)
	}
	if result.NeedRestart {
		t.Error("DryRun 不应该需要重启")
	}
}

func TestApplyUpdate_DryRun_NoReplace(t *testing.T) {
	dir := t.TempDir()

	currentBinary := createTestBinary(t, dir, "atria")
	originalContent, _ := os.ReadFile(currentBinary)

	archivePath := createTestArchive(t, dir, "atria")
	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0700)

	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)

	u.ApplyUpdate(context.Background(), ApplyOptions{
		CurrentBinaryPath: currentBinary,
		AssetPath:         archivePath,
		BackupDir:         backupDir,
		DryRun:            true,
	})

	// 验证原文件未被修改
	currentContent, _ := os.ReadFile(currentBinary)
	if string(currentContent) != string(originalContent) {
		t.Error("DryRun 不应该修改原二进制")
	}
}

func TestApplyUpdate_BackupCreated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 环境跳过 apply 测试")
	}

	dir := t.TempDir()

	currentBinary := createTestBinary(t, dir, "atria")
	archivePath := createTestArchive(t, dir, "atria")
	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0700)

	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)

	result, err := u.ApplyUpdate(context.Background(), ApplyOptions{
		CurrentBinaryPath: currentBinary,
		AssetPath:         archivePath,
		BackupDir:         backupDir,
		DryRun:            false,
	})
	if err != nil {
		t.Fatalf("ApplyUpdate 失败: %s", err)
	}

	if result.BackupPath == "" {
		t.Error("应该创建备份")
	}

	// 验证备份文件存在
	if _, err := os.Stat(result.BackupPath); os.IsNotExist(err) {
		t.Error("备份文件应该存在")
	}
}

func TestApplyUpdate_DataDirPreserved(t *testing.T) {
	dir := t.TempDir()

	// 创建 data 目录和 secret.key
	dataDir := filepath.Join(dir, "data")
	os.MkdirAll(dataDir, 0700)
	secretKey := filepath.Join(dataDir, "secret.key")
	os.WriteFile(secretKey, []byte("test-key"), 0600)

	currentBinary := createTestBinary(t, dir, "atria")
	archivePath := createTestArchive(t, dir, "atria")
	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0700)

	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)

	u.ApplyUpdate(context.Background(), ApplyOptions{
		CurrentBinaryPath: currentBinary,
		AssetPath:         archivePath,
		BackupDir:         backupDir,
		DryRun:            false,
	})

	// 验证 data 目录和 secret.key 仍然存在
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Error("data 目录不应被删除")
	}
	if _, err := os.Stat(secretKey); os.IsNotExist(err) {
		t.Error("secret.key 不应被删除")
	}
}

func TestApplyUpdate_SecretKeyPreserved(t *testing.T) {
	dir := t.TempDir()

	dataDir := filepath.Join(dir, "data")
	os.MkdirAll(dataDir, 0700)
	secretKey := filepath.Join(dataDir, "secret.key")
	os.WriteFile(secretKey, []byte("original-key-content"), 0600)

	currentBinary := createTestBinary(t, dir, "atria")
	archivePath := createTestArchive(t, dir, "atria")
	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0700)

	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)

	u.ApplyUpdate(context.Background(), ApplyOptions{
		CurrentBinaryPath: currentBinary,
		AssetPath:         archivePath,
		BackupDir:         backupDir,
		DryRun:            false,
	})

	// 验证 secret.key 内容未变
	content, _ := os.ReadFile(secretKey)
	if string(content) != "original-key-content" {
		t.Error("secret.key 内容不应被修改")
	}
}

func TestApplyUpdate_SessionFilesPreserved(t *testing.T) {
	dir := t.TempDir()

	sessionDir := filepath.Join(dir, "data", "sessions")
	os.MkdirAll(sessionDir, 0700)
	sessionFile := filepath.Join(sessionDir, "session_1.enc")
	os.WriteFile(sessionFile, []byte("encrypted-session"), 0600)

	currentBinary := createTestBinary(t, dir, "atria")
	archivePath := createTestArchive(t, dir, "atria")
	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0700)

	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)

	u.ApplyUpdate(context.Background(), ApplyOptions{
		CurrentBinaryPath: currentBinary,
		AssetPath:         archivePath,
		BackupDir:         backupDir,
		DryRun:            false,
	})

	// 验证 Session 文件仍然存在
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Error("Session 文件不应被删除")
	}
}

func TestApplyUpdate_CurrentBinaryNotFound(t *testing.T) {
	dir := t.TempDir()

	archivePath := createTestArchive(t, dir, "atria")
	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0700)

	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)

	result, err := u.ApplyUpdate(context.Background(), ApplyOptions{
		CurrentBinaryPath: "/nonexistent/atria",
		AssetPath:         archivePath,
		BackupDir:         backupDir,
		DryRun:            false,
	})
	if err == nil && result.Success {
		t.Error("不存在的二进制应该失败")
	}
}

func TestApplyUpdate_ExtractBinary(t *testing.T) {
	dir := t.TempDir()

	// 创建更新包
	archivePath := createTestArchive(t, dir, "atria")

	// 测试解压
	extractDir := filepath.Join(dir, "extract")
	os.MkdirAll(extractDir, 0700)

	binaryPath, err := extractBinary(archivePath, extractDir)
	if err != nil {
		t.Fatalf("解压失败: %s", err)
	}

	// 验证文件存在
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Error("解压后的二进制应该存在")
	}
}

func TestApplyUpdate_ExtractBinary_WindowsExe(t *testing.T) {
	dir := t.TempDir()

	// 创建包含 .exe 的更新包
	archivePath := createTestArchive(t, dir, "atria.exe")

	extractDir := filepath.Join(dir, "extract")
	os.MkdirAll(extractDir, 0700)

	binaryPath, err := extractBinary(archivePath, extractDir)
	if err != nil {
		t.Fatalf("解压失败: %s", err)
	}

	if filepath.Ext(binaryPath) != ".exe" {
		t.Errorf("Windows 二进制应该有 .exe 扩展名，实际 %s", binaryPath)
	}
}

// TestApplyUpdate_DockerUnsupported 测试 Docker 环境下 ApplyUpdate 返回 unsupported。
// 通过注入 dockerDetector 模拟 Docker 环境，不依赖真实 Docker。
func TestApplyUpdate_DockerUnsupported(t *testing.T) {
	dir := t.TempDir()

	// 创建当前二进制
	currentBinary := createTestBinary(t, dir, "atria")
	originalContent, _ := os.ReadFile(currentBinary)

	// 创建 data 目录和 secret.key
	dataDir := filepath.Join(dir, "data")
	os.MkdirAll(dataDir, 0700)
	secretKey := filepath.Join(dataDir, "secret.key")
	os.WriteFile(secretKey, []byte("test-secret-key-content"), 0600)

	// 创建 session 文件
	sessionDir := filepath.Join(dataDir, "sessions")
	os.MkdirAll(sessionDir, 0700)
	sessionFile := filepath.Join(sessionDir, "session_1.enc")
	os.WriteFile(sessionFile, []byte("encrypted-session-data"), 0600)

	// 创建更新包
	archivePath := createTestArchive(t, dir, "atria")

	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0700)

	// 创建 updater 并注入 Docker 检测器返回 true
	u := NewDefaultUpdater("v0.1.0", "test/repo", "", "", true, nil)
	u.SetDockerDetector(func() bool { return true })

	result, err := u.ApplyUpdate(context.Background(), ApplyOptions{
		CurrentBinaryPath: currentBinary,
		AssetPath:         archivePath,
		BackupDir:         backupDir,
		DryRun:            false,
	})

	// 不应返回错误
	if err != nil {
		t.Fatalf("不应返回错误: %s", err)
	}

	// 应返回 unsupported
	if result.Success {
		t.Error("Docker 环境下 ApplyUpdate 不应成功")
	}
	if !strings.Contains(result.Message, "Docker") {
		t.Errorf("错误消息应包含 Docker，实际: %s", result.Message)
	}

	// 验证当前二进制未被替换
	currentContent, _ := os.ReadFile(currentBinary)
	if !bytes.Equal(currentContent, originalContent) {
		t.Error("Docker 环境下不应替换二进制")
	}

	// 验证 data/ 目录仍然存在
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Error("data/ 目录不应被删除")
	}

	// 验证 secret.key 仍然存在且内容未变
	secretContent, err := os.ReadFile(secretKey)
	if err != nil {
		t.Error("secret.key 不应被删除")
	}
	if string(secretContent) != "test-secret-key-content" {
		t.Error("secret.key 内容不应被修改")
	}

	// 验证 session 文件仍然存在且内容未变
	sessionContent, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Error("Session 文件不应被删除")
	}
	if string(sessionContent) != "encrypted-session-data" {
		t.Error("Session 文件内容不应被修改")
	}
}
