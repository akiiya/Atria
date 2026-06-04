package updater

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
)

// Updater 定义自更新操作的接口。
type Updater interface {
	// CheckLatest 检查最新版本。
	CheckLatest(ctx context.Context, opts CheckOptions) (*ReleaseInfo, error)

	// SelectAsset 从 Release 中选择匹配当前平台的产物。
	SelectAsset(release ReleaseInfo, goos string, goarch string) (*AssetInfo, error)

	// DownloadAsset 下载产物到指定目录。
	DownloadAsset(ctx context.Context, asset AssetInfo, destDir string) (string, error)

	// VerifyChecksum 校验文件 checksum。
	VerifyChecksum(filePath string, expectedChecksum string) error

	// ApplyUpdate 应用更新。
	ApplyUpdate(ctx context.Context, opts ApplyOptions) (*ApplyResult, error)

	// GetState 获取当前更新状态。
	GetState() UpdateState

	// SetState 设置更新状态。
	SetState(state UpdateState)
}

// DockerDetector 是 Docker 环境检测函数类型。
type DockerDetector func() bool

// DefaultUpdater 是默认的 Updater 实现。
type DefaultUpdater struct {
	currentVersion  string
	repo            string
	checkURL        string
	downloadURL     string
	requireChecksum bool
	logger          *slog.Logger
	state           UpdateState
	dockerDetector  DockerDetector
}

// NewDefaultUpdater 创建默认的 Updater 实例。
func NewDefaultUpdater(currentVersion, repo, checkURL, downloadURL string, requireChecksum bool, logger *slog.Logger) *DefaultUpdater {
	return &DefaultUpdater{
		currentVersion:  currentVersion,
		repo:            repo,
		checkURL:        checkURL,
		downloadURL:     downloadURL,
		requireChecksum: requireChecksum,
		logger:          logger,
		dockerDetector:  IsDockerEnvironment,
		state: UpdateState{
			Status:         StatusIdle,
			CurrentVersion: currentVersion,
		},
	}
}

// SetDockerDetector 设置 Docker 环境检测函数（用于测试）。
func (u *DefaultUpdater) SetDockerDetector(detector DockerDetector) {
	u.dockerDetector = detector
}

// IsDocker 检测当前是否在 Docker 环境中。
func (u *DefaultUpdater) IsDocker() bool {
	if u.dockerDetector != nil {
		return u.dockerDetector()
	}
	return false
}

// VerifyChecksum 校验文件 checksum。
func (u *DefaultUpdater) VerifyChecksum(filePath string, expectedChecksum string) error {
	return VerifyChecksum(filePath, expectedChecksum)
}

// GetState 获取当前更新状态。
func (u *DefaultUpdater) GetState() UpdateState {
	return u.state
}

// SetState 设置更新状态。
func (u *DefaultUpdater) SetState(state UpdateState) {
	u.state = state
}

// SelectAsset 从 Release 中选择匹配当前平台的产物。
func (u *DefaultUpdater) SelectAsset(release ReleaseInfo, goos string, goarch string) (*AssetInfo, error) {
	// 构建期望的文件名模式
	expectedName := fmt.Sprintf("atria_%s_%s", goos, goarch)

	for _, asset := range release.Assets {
		if asset.Name == expectedName+".tar.gz" || asset.Name == expectedName+".zip" {
			asset.OS = goos
			asset.Arch = goarch
			return &asset, nil
		}
	}

	return nil, fmt.Errorf("未找到匹配的产物: %s/%s", goos, goarch)
}

// GetPlatformAssetName 返回当前平台的产物文件名。
func GetPlatformAssetName(goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("atria_%s_%s%s", goos, goarch, ext)
}

// IsDockerEnvironment 检测是否在 Docker 容器中运行。
func IsDockerEnvironment() bool {
	// 检查 /.dockerenv 文件
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// 检查 /proc/1/cgroup
	data, err := os.ReadFile("/proc/1/cgroup")
	if err == nil {
		content := string(data)
		if len(content) > 0 {
			// Docker 容器的 cgroup 通常包含 "docker" 或 "containerd"
			if len(content) > 1024 {
				content = content[:1024]
			}
			if len(content) > 0 {
				return true
			}
		}
	}

	return false
}

// GetCurrentOS 返回当前操作系统。
func GetCurrentOS() string {
	return runtime.GOOS
}

// GetCurrentArch 返回当前架构。
func GetCurrentArch() string {
	return runtime.GOARCH
}
