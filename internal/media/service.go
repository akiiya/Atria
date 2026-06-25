// Package media 提供媒体文件下载和缓存服务。
package media

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/telegramclient"
	"gorm.io/gorm"
)

// Service 媒体服务。
type Service struct {
	db      *gorm.DB
	adapter telegramclient.ClientAdapter
	dataDir string
	logger  *slog.Logger
}

// NewService 创建媒体服务。
func NewService(db *gorm.DB, adapter telegramclient.ClientAdapter, dataDir string, logger *slog.Logger) *Service {
	return &Service{db: db, adapter: adapter, dataDir: dataDir, logger: logger}
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
	// 检查已有缓存
	var cache model.MediaCache
	err := s.db.Where("account_id = ? AND peer_ref = ? AND telegram_message_id = ?",
		accountID, peerRef, messageID).First(&cache).Error

	if err == nil && cache.Status == "cached" {
		return &DownloadResult{
			Status:   "cached",
			FileName: cache.FileName,
			MIMEType: cache.MIMEType,
			FileSize: cache.FileSize,
		}, nil
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
		s.db.Model(&cache).Updates(map[string]any{
			"status":        "failed",
			"error_message": err.Error(),
		})
		return nil, err
	}

	// 更新缓存
	s.db.Model(&cache).Updates(map[string]any{
		"status":     "cached",
		"file_name":  result.FileName,
		"mime_type":  result.MIMEType,
		"file_size":  result.Size,
		"local_path": result.FilePath,
	})

	return &DownloadResult{
		Status:   "cached",
		FileName: result.FileName,
		MIMEType: result.MIMEType,
		FileSize: result.Size,
	}, nil
}

// GetMediaContent 返回已缓存媒体文件的路径和元信息。
func (s *Service) GetMediaContent(ctx context.Context, accountID uint, peerRef string, messageID int) (string, *model.MediaCache, error) {
	var cache model.MediaCache
	err := s.db.Where("account_id = ? AND peer_ref = ? AND telegram_message_id = ?",
		accountID, peerRef, messageID).First(&cache).Error
	if err != nil {
		return "", nil, err
	}
	if cache.Status != "cached" {
		return "", nil, fmt.Errorf("media not cached")
	}

	// 验证文件存在
	fullPath := filepath.Join(s.dataDir, cache.LocalPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		s.db.Model(&cache).Update("status", "none")
		return "", nil, fmt.Errorf("cached file missing")
	}

	return fullPath, &cache, nil
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
