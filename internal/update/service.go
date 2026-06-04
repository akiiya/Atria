// Package update 提供自更新业务服务。
package update

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/user/atria/internal/audit"
	"github.com/user/atria/internal/config"
	"github.com/user/atria/internal/updater"
	"github.com/user/atria/internal/version"

	"gorm.io/gorm"
)

// Service 是更新业务服务。
type Service struct {
	db      *gorm.DB
	cfg     *config.Config
	updater updater.Updater
	state   updater.UpdateState
}

// NewService 创建更新服务。
func NewService(db *gorm.DB, cfg *config.Config) *Service {
	u := updater.NewDefaultUpdater(
		version.Short(),
		cfg.UpdateRepo,
		cfg.UpdateCheckURL,
		cfg.UpdateDownloadBaseURL,
		cfg.UpdateRequireChecksum,
		slog.Default(),
	)

	// 加载状态
	statePath := filepath.Join(cfg.UpdateDir, "update_state.json")
	state, err := updater.LoadState(statePath)
	if err != nil {
		slog.Warn("加载更新状态失败", "error", err)
		state = &updater.UpdateState{
			Status:         updater.StatusIdle,
			CurrentVersion: version.Short(),
		}
	}

	return &Service{
		db:      db,
		cfg:     cfg,
		updater: u,
		state:   *state,
	}
}

// GetState 获取当前更新状态。
func (s *Service) GetState() updater.UpdateState {
	return s.state
}

// CheckUpdate 检查更新。
func (s *Service) CheckUpdate(ctx context.Context, actorID uint, ip, userAgent string) (*updater.ReleaseInfo, error) {
	if !s.cfg.UpdateEnabled {
		s.state.Status = updater.StatusDisabled
		s.state.Message = "自更新功能已禁用"
		return nil, fmt.Errorf("自更新功能已禁用")
	}

	release, err := s.updater.CheckLatest(ctx, updater.CheckOptions{
		Repo:            s.cfg.UpdateRepo,
		AllowPrerelease: s.cfg.UpdateAllowPrerelease,
		CustomCheckURL:  s.cfg.UpdateCheckURL,
	})

	// 保存状态
	s.state = s.updater.GetState()
	s.saveState()

	// 审计日志
	if err != nil {
		audit.Log(ctx, s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", actorID),
			Action:       "system.update_checked",
			ResourceType: "system",
			ResourceID:   "0",
			RiskLevel:    "low",
			IP:           ip,
			UserAgent:    userAgent,
			Message:      "检查更新失败",
			Metadata: map[string]any{
				"current_version": version.Short(),
				"error":           err.Error(),
			},
		})
		return nil, err
	}

	if s.state.Status == updater.StatusUpdateAvailable {
		audit.Log(ctx, s.db, audit.Event{
			ActorType:    "admin",
			ActorID:      fmt.Sprintf("%d", actorID),
			Action:       "system.update_available",
			ResourceType: "system",
			ResourceID:   "0",
			RiskLevel:    "low",
			IP:           ip,
			UserAgent:    userAgent,
			Message:      fmt.Sprintf("发现新版本 %s", release.TagName),
			Metadata: map[string]any{
				"current_version": version.Short(),
				"latest_version":  release.TagName,
			},
		})
	}

	return release, nil
}

// DownloadUpdate 下载更新。
func (s *Service) DownloadUpdate(ctx context.Context, actorID uint, ip, userAgent string) (string, error) {
	if !s.cfg.UpdateEnabled {
		return "", fmt.Errorf("自更新功能已禁用")
	}

	// 如果没有检查过，先检查
	if s.state.Status == updater.StatusIdle {
		if _, err := s.CheckUpdate(ctx, actorID, ip, userAgent); err != nil {
			return "", err
		}
	}

	if s.state.Status != updater.StatusUpdateAvailable {
		return "", fmt.Errorf("没有可用的更新")
	}

	// 获取 Release 信息
	release, err := s.updater.CheckLatest(ctx, updater.CheckOptions{
		Repo:            s.cfg.UpdateRepo,
		AllowPrerelease: s.cfg.UpdateAllowPrerelease,
		CustomCheckURL:  s.cfg.UpdateCheckURL,
	})
	if err != nil {
		return "", err
	}

	// 选择资产
	asset, err := s.updater.SelectAsset(*release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	// 下载
	assetPath, err := s.updater.DownloadAsset(ctx, *asset, s.cfg.UpdateDir)
	if err != nil {
		s.state.Status = updater.StatusFailed
		s.state.Error = err.Error()
		s.saveState()
		return "", err
	}

	// 校验 checksum
	if s.cfg.UpdateRequireChecksum && release.ChecksumAsset != nil {
		// 下载 checksum
		checksumPath, err := s.updater.DownloadAsset(ctx, *release.ChecksumAsset, s.cfg.UpdateDir)
		if err != nil {
			s.state.Status = updater.StatusFailed
			s.state.Error = "下载 checksum 失败"
			s.saveState()
			return "", fmt.Errorf("下载 checksum 失败: %w", err)
		}

		// 解析 checksum
		checksumData, err := os.ReadFile(checksumPath)
		if err != nil {
			return "", fmt.Errorf("读取 checksum 失败: %w", err)
		}

		checksums, err := updater.ParseChecksums(checksumData)
		if err != nil {
			return "", fmt.Errorf("解析 checksum 失败: %w", err)
		}

		expected, ok := checksums[asset.Name]
		if !ok {
			return "", fmt.Errorf("checksum 中未找到 %s", asset.Name)
		}

		if err := updater.VerifyChecksum(assetPath, expected); err != nil {
			s.state.Status = updater.StatusFailed
			s.state.Error = "checksum 校验失败"
			s.saveState()
			return "", err
		}
	}

	s.state.Status = updater.StatusDownloaded
	s.state.AssetName = asset.Name
	now := time.Now()
	s.state.DownloadedAt = &now
	s.state.Message = "下载完成"
	s.saveState()

	// 审计日志
	audit.Log(ctx, s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", actorID),
		Action:       "system.update_downloaded",
		ResourceType: "system",
		ResourceID:   "0",
		RiskLevel:    "low",
		IP:           ip,
		UserAgent:    userAgent,
		Message:      fmt.Sprintf("下载更新 %s", asset.Name),
		Metadata: map[string]any{
			"asset_name":     asset.Name,
			"latest_version": release.TagName,
		},
	})

	return assetPath, nil
}

// ApplyUpdate 应用更新。
func (s *Service) ApplyUpdate(ctx context.Context, actorID uint, ip, userAgent string, dryRun bool) (*updater.ApplyResult, error) {
	if !s.cfg.UpdateEnabled {
		return nil, fmt.Errorf("自更新功能已禁用")
	}

	if s.state.Status != updater.StatusDownloaded && s.state.Status != updater.StatusFailed {
		return nil, fmt.Errorf("没有已下载的更新")
	}

	// 获取当前二进制路径
	currentBinary, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("获取当前二进制路径失败: %w", err)
	}

	// 查找下载的资产
	assetPath := filepath.Join(s.cfg.UpdateDir, s.state.AssetName)

	result, err := s.updater.ApplyUpdate(ctx, updater.ApplyOptions{
		CurrentBinaryPath: currentBinary,
		AssetPath:         assetPath,
		BackupDir:         s.cfg.UpdateBackupDir,
		DryRun:            dryRun,
	})

	// 更新状态
	s.state = s.updater.GetState()
	s.saveState()

	// 审计日志
	action := "system.update_applied"
	message := "应用更新成功"
	riskLevel := "high"
	if dryRun {
		action = "system.update_dry_run"
		message = "DryRun 验证完成"
		riskLevel = "low"
	}
	if err != nil || (result != nil && !result.Success) {
		action = "system.update_apply_failed"
		message = "应用更新失败"
		if result != nil {
			message = result.Message
		}
	}

	audit.Log(ctx, s.db, audit.Event{
		ActorType:    "admin",
		ActorID:      fmt.Sprintf("%d", actorID),
		Action:       action,
		ResourceType: "system",
		ResourceID:   "0",
		RiskLevel:    riskLevel,
		IP:           ip,
		UserAgent:    userAgent,
		Message:      message,
		Metadata: map[string]any{
			"current_version": version.Short(),
			"latest_version":  s.state.LatestVersion,
			"dry_run":         dryRun,
		},
	})

	return result, err
}

// saveState 保存更新状态。
func (s *Service) saveState() {
	statePath := filepath.Join(s.cfg.UpdateDir, "update_state.json")
	if err := updater.SaveState(statePath, s.state); err != nil {
		slog.Error("保存更新状态失败", "error", err)
	}
}
