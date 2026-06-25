// Package media 提供媒体文件下载和缓存服务。
package media

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/telegramclient"
	"gorm.io/gorm"
)

const (
	// MaxFileSize 最大允许缓存的文件大小（100MB）。
	MaxFileSize = 100 * 1024 * 1024
	// DownloadTimeout 下载超时时间，超过此时间的 downloading 状态将被视为 stale。
	DownloadTimeout = 5 * time.Minute
)

// Service 媒体服务。
type Service struct {
	db      *gorm.DB
	adapter telegramclient.ClientAdapter
	dataDir string
	logger  *slog.Logger
	mu      sync.Mutex // 防止并发下载同一文件
}

// NewService 创建媒体服务。
func NewService(db *gorm.DB, adapter telegramclient.ClientAdapter, dataDir string, logger *slog.Logger) *Service {
	s := &Service{db: db, adapter: adapter, dataDir: dataDir, logger: logger}
	s.recoverStaleDownloads()
	return s
}

// recoverStaleDownloads 启动时将卡在 downloading 状态超过 DownloadTimeout 的记录重置为 failed。
func (s *Service) recoverStaleDownloads() {
	cutoff := time.Now().Add(-DownloadTimeout)
	result := s.db.Model(&model.MediaCache{}).
		Where("status = ? AND updated_at < ?", "downloading", cutoff).
		Updates(map[string]any{
			"status":        "failed",
			"error_message": "download timeout",
		})
	if result.RowsAffected > 0 {
		s.logger.Warn("恢复卡住的媒体下载", "count", result.RowsAffected)
	}
}

// sanitizeLocalPath 防止路径穿越。
func sanitizeLocalPath(path string) string {
	if path == "" {
		return ""
	}
	// 移除前导 /
	path = strings.TrimLeft(path, "/")
	// 清理路径组件
	cleaned := filepath.Clean(path)
	// 检查是否包含 ..
	if strings.Contains(cleaned, "..") {
		return ""
	}
	return cleaned
}

// sanitizeFileName 清理文件名用于落盘。
func sanitizeFileName(name string) string {
	if name == "" {
		return "unnamed"
	}
	// 只保留 basename
	name = filepath.Base(name)
	// 移除路径分隔符
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	// 移除空字节
	name = strings.ReplaceAll(name, "\x00", "")
	if name == "" || name == "." || name == ".." {
		return "unnamed"
	}
	return name
}

// GetMediaStatus 返回媒体缓存状态。
func (s *Service) GetMediaStatus(ctx context.Context, accountID uint, peerRef string, messageID int) (*MediaStatusResult, error) {
	var cache model.MediaCache
	err := s.db.Where("account_id = ? AND peer_ref = ? AND telegram_message_id = ?",
		accountID, peerRef, messageID).First(&cache).Error

	if err == gorm.ErrRecordNotFound {
		return &MediaStatusResult{
			Status:    "none",
			Available: false,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return &MediaStatusResult{
		Status:    cache.Status,
		FileName:  cache.FileName,
		MIMEType:  cache.MIMEType,
		FileSize:  cache.FileSize,
		Available: cache.Status == "cached",
	}, nil
}

// DownloadMedia 下载媒体文件并缓存。
func (s *Service) DownloadMedia(ctx context.Context, accountID uint, peerRef string, messageID int, peerID int64, peerType string, accessHash int64, apiID int, apiHash string, sessionPath string) (*DownloadResult, error) {
	// 防止并发下载同一文件
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查已有缓存
	var cache model.MediaCache
	err := s.db.Where("account_id = ? AND peer_ref = ? AND telegram_message_id = ?",
		accountID, peerRef, messageID).First(&cache).Error

	if err == nil && cache.Status == "cached" {
		// 验证文件仍然存在
		fullPath := filepath.Join(s.dataDir, cache.LocalPath)
		if _, statErr := os.Stat(fullPath); statErr == nil {
			return &DownloadResult{
				Status:   "cached",
				FileName: cache.FileName,
				MIMEType: cache.MIMEType,
				FileSize: cache.FileSize,
			}, nil
		}
		// 文件丢失，重新下载
	}

	// 更新状态为 downloading
	if err == gorm.ErrRecordNotFound {
		cache = model.MediaCache{
			AccountID:         accountID,
			PeerRef:           peerRef,
			TelegramMessageID: messageID,
			Status:            "downloading",
		}
		s.db.Create(&cache)
	} else if err == nil {
		s.db.Model(&cache).Update("status", "downloading")
	} else {
		return nil, err
	}

	// 通过 adapter 下载
	result, err := s.adapter.DownloadMedia(ctx, telegramclient.DownloadMediaRequest{
		AccountID:       accountID,
		PeerRef:         peerRef,
		MessageID:       messageID,
		APIID:           apiID,
		APIHash:         apiHash,
		SessionFilePath: sessionPath,
		PeerID:          peerID,
		PeerType:        telegramclient.PeerType(peerType),
		AccessHash:      accessHash,
	})

	if err != nil {
		errMsg := err.Error()
		s.db.Model(&cache).Updates(map[string]any{
			"status":        "failed",
			"error_message": errMsg,
		})
		return nil, err
	}

	// 检查文件大小限制
	if result.Size > MaxFileSize {
		s.db.Model(&cache).Updates(map[string]any{
			"status":        "failed",
			"error_message": "file too large",
		})
		return nil, fmt.Errorf("文件大小超过限制 (%d bytes)", MaxFileSize)
	}

	// 更新缓存（sanitize file name）
	safeName := sanitizeFileName(result.FileName)
	s.db.Model(&cache).Updates(map[string]any{
		"status":     "cached",
		"file_name":  safeName,
		"mime_type":  result.MIMEType,
		"file_size":  result.Size,
		"local_path": result.FilePath,
	})

	return &DownloadResult{
		Status:   "cached",
		FileName: safeName,
		MIMEType: result.MIMEType,
		FileSize: result.Size,
	}, nil
}

// GetMediaContent 返回已缓存媒体文件的路径和元信息。
func (s *Service) GetMediaContent(ctx context.Context, accountID uint, peerRef string, messageID int) (string, string, string, error) {
	var cache model.MediaCache
	err := s.db.Where("account_id = ? AND peer_ref = ? AND telegram_message_id = ?",
		accountID, peerRef, messageID).First(&cache).Error
	if err != nil {
		return "", "", "", err
	}
	if cache.Status != "cached" {
		return "", "", "", fmt.Errorf("media not cached")
	}

	// 路径安全检查
	cleanPath := sanitizeLocalPath(cache.LocalPath)
	if cleanPath == "" {
		return "", "", "", fmt.Errorf("invalid local path")
	}

	fullPath := filepath.Join(s.dataDir, cleanPath)

	// 确保解析后的路径仍在 dataDir 内
	absDataDir, _ := filepath.Abs(s.dataDir)
	absFullPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFullPath, absDataDir) {
		return "", "", "", fmt.Errorf("path traversal detected")
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		s.db.Model(&cache).Update("status", "none")
		return "", "", "", fmt.Errorf("cached file missing")
	}

	// 返回安全的 MIME 和文件名
	mimeType := cache.MIMEType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	fileName := sanitizeFileName(cache.FileName)

	return fullPath, mimeType, fileName, nil
}

// GetCacheStats 返回媒体缓存统计。
func (s *Service) GetCacheStats() (*CacheStats, error) {
	var stats CacheStats
	s.db.Model(&model.MediaCache{}).Count(&stats.RecordCount)
	s.db.Model(&model.MediaCache{}).Where("status = ?", "cached").Count(&stats.CachedCount)
	s.db.Model(&model.MediaCache{}).Where("status = ?", "failed").Count(&stats.FailedCount)
	s.db.Model(&model.MediaCache{}).Where("status = ?", "downloading").Count(&stats.DownloadingCount)

	// 计算总大小
	var totalSize *int64
	s.db.Model(&model.MediaCache{}).Where("status = ?", "cached").Select("COALESCE(SUM(file_size), 0)").Scan(&totalSize)
	if totalSize != nil {
		stats.TotalSize = *totalSize
	}

	// 计算实际文件数
	mediaDir := filepath.Join(s.dataDir, "media")
	var fileCount int64
	filepath.Walk(mediaDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileCount++
		}
		return nil
	})
	stats.FileCount = fileCount

	return &stats, nil
}

// CleanupCache 清理媒体缓存。
func (s *Service) CleanupCache(ctx context.Context, accountID uint, peerRef string, onlyFailed bool) (*CleanupResult, error) {
	query := s.db.Model(&model.MediaCache{})
	if accountID > 0 {
		query = query.Where("account_id = ?", accountID)
	}
	if peerRef != "" {
		query = query.Where("peer_ref = ?", peerRef)
	}
	if onlyFailed {
		query = query.Where("status IN ?", []string{"failed", "downloading"})
	}

	// 获取将删除的记录
	var records []model.MediaCache
	query.Find(&records)

	result := &CleanupResult{
		RecordCount: int64(len(records)),
	}

	// 删除文件和记录
	for _, rec := range records {
		if rec.LocalPath != "" {
			cleanPath := sanitizeLocalPath(rec.LocalPath)
			if cleanPath != "" {
				fullPath := filepath.Join(s.dataDir, cleanPath)
				absDataDir, _ := filepath.Abs(s.dataDir)
				absFullPath, _ := filepath.Abs(fullPath)
				if strings.HasPrefix(absFullPath, absDataDir) {
					if err := os.Remove(fullPath); err == nil {
						result.FileCount++
						result.TotalSize += rec.FileSize
					}
				}
			}
		}
		s.db.Delete(&rec)
	}

	return result, nil
}

// CacheStats 媒体缓存统计。
type CacheStats struct {
	RecordCount      int64 `json:"record_count"`
	CachedCount      int64 `json:"cached_count"`
	FailedCount      int64 `json:"failed_count"`
	DownloadingCount int64 `json:"downloading_count"`
	FileCount        int64 `json:"file_count"`
	TotalSize        int64 `json:"total_size"`
}

// CleanupResult 缓存清理结果。
type CleanupResult struct {
	RecordCount int64 `json:"record_count"`
	FileCount   int64 `json:"file_count"`
	TotalSize   int64 `json:"total_size"`
}

// MediaStatusResult 媒体状态结果。
type MediaStatusResult struct {
	Status    string `json:"status"`
	FileName  string `json:"file_name,omitempty"`
	MIMEType  string `json:"mime_type,omitempty"`
	FileSize  int64  `json:"file_size"`
	Available bool   `json:"available"`
}

// DownloadResult 下载结果。
type DownloadResult struct {
	Status   string `json:"status"`
	FileName string `json:"file_name"`
	MIMEType string `json:"mime_type"`
	FileSize int64  `json:"file_size"`
}
