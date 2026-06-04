// Package updater 提供 Atria 自更新功能。
// 包括检查更新、下载、校验、应用更新等能力。
package updater

import (
	"time"
)

// UpdateStatus 表示更新状态。
type UpdateStatus string

const (
	StatusIdle            UpdateStatus = "idle"
	StatusChecking        UpdateStatus = "checking"
	StatusUpdateAvailable UpdateStatus = "update_available"
	StatusUpToDate        UpdateStatus = "up_to_date"
	StatusDownloading     UpdateStatus = "downloading"
	StatusDownloaded      UpdateStatus = "downloaded"
	StatusApplying        UpdateStatus = "applying"
	StatusApplied         UpdateStatus = "applied"
	StatusRestartRequired UpdateStatus = "restart_required"
	StatusFailed          UpdateStatus = "failed"
	StatusDisabled        UpdateStatus = "disabled"
	StatusUnsupported     UpdateStatus = "unsupported"
)

// ReleaseInfo 表示 GitHub Release 信息。
type ReleaseInfo struct {
	Version       string      `json:"version"`
	TagName       string      `json:"tag_name"`
	Name          string      `json:"name"`
	Body          string      `json:"body"`
	PublishedAt   time.Time   `json:"published_at"`
	Prerelease    bool        `json:"prerelease"`
	Assets        []AssetInfo `json:"assets"`
	ChecksumAsset *AssetInfo  `json:"checksum_asset,omitempty"`
}

// AssetInfo 表示 Release 产物信息。
type AssetInfo struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Checksum string `json:"checksum,omitempty"`
}

// CheckOptions 是检查更新的选项。
type CheckOptions struct {
	// Repo GitHub 仓库，格式 owner/repo
	Repo string

	// AllowPrerelease 是否允许预发布版本
	AllowPrerelease bool

	// CustomCheckURL 自定义检查 URL（测试用）
	CustomCheckURL string
}

// UpdateState 表示更新状态（可持久化）。
type UpdateState struct {
	Status         UpdateStatus `json:"status"`
	CurrentVersion string       `json:"current_version"`
	LatestVersion  string       `json:"latest_version,omitempty"`
	Message        string       `json:"message,omitempty"`
	CheckedAt      *time.Time   `json:"checked_at,omitempty"`
	DownloadedAt   *time.Time   `json:"downloaded_at,omitempty"`
	AppliedAt      *time.Time   `json:"applied_at,omitempty"`
	Error          string       `json:"error,omitempty"`
	AssetName      string       `json:"asset_name,omitempty"`
	BackupPath     string       `json:"backup_path,omitempty"`
	PendingRestart bool         `json:"pending_restart"`
}

// ApplyOptions 是应用更新的选项。
type ApplyOptions struct {
	// CurrentBinaryPath 当前二进制路径
	CurrentBinaryPath string

	// AssetPath 已下载的更新包路径
	AssetPath string

	// BackupDir 备份目录
	BackupDir string

	// DryRun 是否只验证不实际替换
	DryRun bool
}

// ApplyResult 是应用更新的结果。
type ApplyResult struct {
	Success     bool   `json:"success"`
	BackupPath  string `json:"backup_path,omitempty"`
	NewVersion  string `json:"new_version,omitempty"`
	NeedRestart bool   `json:"need_restart"`
	Message     string `json:"message"`
}
