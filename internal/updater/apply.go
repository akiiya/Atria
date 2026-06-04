package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DownloadAsset 下载产物到指定目录。
func (u *DefaultUpdater) DownloadAsset(ctx context.Context, asset AssetInfo, destDir string) (string, error) {
	u.state.Status = StatusDownloading
	u.state.Message = fmt.Sprintf("正在下载 %s...", asset.Name)

	if err := os.MkdirAll(destDir, 0700); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	destPath := filepath.Join(destDir, asset.Name)

	// 安全检查：确保路径在目标目录内
	absDestDir, _ := filepath.Abs(destDir)
	absDestPath, _ := filepath.Abs(destPath)
	if !strings.HasPrefix(absDestPath, absDestDir) {
		return "", fmt.Errorf("路径穿越检测")
	}

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", asset.URL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Atria-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载返回状态码 %d", resp.StatusCode)
	}

	// 写入临时文件
	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("写入文件失败: %w", err)
	}
	f.Close()

	// rename 为正式文件
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("重命名文件失败: %w", err)
	}

	now := time.Now()
	u.state.DownloadedAt = &now
	u.state.AssetName = asset.Name
	u.state.Status = StatusDownloaded
	u.state.Message = "下载完成"

	return destPath, nil
}

// ApplyUpdate 应用更新。
func (u *DefaultUpdater) ApplyUpdate(ctx context.Context, opts ApplyOptions) (*ApplyResult, error) {
	// Docker 环境检测：只禁止真实 Apply，不禁止 DryRun
	// DryRun 用于验证资产、checksum、解压、版本检测等流程，不替换二进制
	if u.IsDocker() && !opts.DryRun {
		return &ApplyResult{
			Success: false,
			Message: "Docker 环境不支持容器内自更新，请使用新镜像重建容器",
		}, nil
	}

	u.state.Status = StatusApplying
	u.state.Message = "正在应用更新..."

	// 检查当前二进制
	if _, err := os.Stat(opts.CurrentBinaryPath); err != nil {
		return &ApplyResult{
			Success: false,
			Message: "当前二进制文件不存在",
		}, nil
	}

	// 解压更新包
	tmpDir := filepath.Join(filepath.Dir(opts.AssetPath), "extract")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return &ApplyResult{Success: false, Message: "创建临时目录失败"}, err
	}
	defer os.RemoveAll(tmpDir)

	newBinaryPath, err := extractBinary(opts.AssetPath, tmpDir)
	if err != nil {
		return &ApplyResult{Success: false, Message: "解压更新包失败"}, err
	}

	// 验证新二进制可执行
	if err := verifyBinary(newBinaryPath); err != nil {
		return &ApplyResult{Success: false, Message: "新二进制验证失败"}, err
	}

	// DryRun 模式
	if opts.DryRun {
		u.state.Status = StatusDownloaded
		u.state.Message = "DryRun 完成，未实际替换"
		return &ApplyResult{
			Success:     true,
			NeedRestart: false,
			Message:     "DryRun 验证通过",
		}, nil
	}

	// 备份当前二进制
	if err := os.MkdirAll(opts.BackupDir, 0700); err != nil {
		return &ApplyResult{Success: false, Message: "创建备份目录失败"}, err
	}

	backupName := fmt.Sprintf("atria.bak.%d", time.Now().Unix())
	backupPath := filepath.Join(opts.BackupDir, backupName)

	if err := copyFile(opts.CurrentBinaryPath, backupPath); err != nil {
		return &ApplyResult{Success: false, Message: "备份当前二进制失败"}, err
	}

	u.state.BackupPath = backupPath

	// 替换二进制
	if err := copyFile(newBinaryPath, opts.CurrentBinaryPath); err != nil {
		// 回滚
		copyFile(backupPath, opts.CurrentBinaryPath)
		return &ApplyResult{
			Success:    false,
			BackupPath: backupPath,
			Message:    "替换二进制失败，已回滚",
		}, err
	}

	// 设置可执行权限
	if err := os.Chmod(opts.CurrentBinaryPath, 0755); err != nil {
		// 回滚
		copyFile(backupPath, opts.CurrentBinaryPath)
		return &ApplyResult{
			Success:    false,
			BackupPath: backupPath,
			Message:    "设置权限失败，已回滚",
		}, err
	}

	now := time.Now()
	u.state.Status = StatusRestartRequired
	u.state.AppliedAt = &now
	u.state.PendingRestart = true
	u.state.Message = "更新已应用，需要重启服务"

	return &ApplyResult{
		Success:     true,
		BackupPath:  backupPath,
		NeedRestart: true,
		Message:     "更新已应用，请重启服务以使用新版本",
	}, nil
}

// extractBinary 从压缩包中提取二进制文件。
func extractBinary(archivePath, destDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZipBinary(archivePath, destDir)
	}
	return extractTarGzBinary(archivePath, destDir)
}

// extractTarGzBinary 从 tar.gz 中提取二进制。
func extractTarGzBinary(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		name := header.Name
		if name == "atria" || name == "atria.exe" {
			destPath := filepath.Join(destDir, name)
			outFile, err := os.Create(destPath)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()
			os.Chmod(destPath, 0755)
			return destPath, nil
		}
	}

	return "", fmt.Errorf("压缩包中未找到 atria 二进制")
}

// extractZipBinary 从 zip 中提取二进制。
func extractZipBinary(archivePath, destDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "atria" || f.Name == "atria.exe" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			destPath := filepath.Join(destDir, f.Name)
			outFile, err := os.Create(destPath)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(outFile, rc); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()
			os.Chmod(destPath, 0755)
			return destPath, nil
		}
	}

	return "", fmt.Errorf("压缩包中未找到 atria 二进制")
}

// verifyBinary 验证二进制可执行。
func verifyBinary(path string) error {
	cmd := exec.Command(path, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("执行 version 命令失败: %w, 输出: %s", err, string(output))
	}
	return nil
}

// copyFile 复制文件。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Sync()
}
