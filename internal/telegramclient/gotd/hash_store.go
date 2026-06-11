package gotd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gotd/td/telegram/updates"
	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/model"

	"gorm.io/gorm"
)

// HashStore 实现 gotd updates.ChannelAccessHasher 接口。
// 将 channel access_hash 加密存储到 ChatPeerCache。
type HashStore struct {
	db     *gorm.DB
	key    []byte
	logger *slog.Logger
}

// NewHashStore 创建 HashStore。
func NewHashStore(db *gorm.DB, key []byte, logger *slog.Logger) *HashStore {
	return &HashStore{db: db, key: key, logger: logger}
}

// GetChannelAccessHash 获取频道的 access_hash。
func (h *HashStore) GetChannelAccessHash(ctx context.Context, userID, channelID int64) (int64, bool, error) {
	var cache model.ChatPeerCache
	peerRef := fmt.Sprintf("ch_%d", channelID)
	err := h.db.Where("peer_ref = ? AND account_id = ? AND peer_type = ?", peerRef, uint(userID), "channel").
		First(&cache).Error
	if err == gorm.ErrRecordNotFound {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("查询 channel access_hash 失败: %w", err)
	}

	if cache.AccessHashEncrypted == "" {
		return 0, false, nil
	}

	// 解密 access_hash
	plain, err := crypto.DecryptString(h.key, cache.AccessHashEncrypted, []byte("atria:chat_peer:v1"))
	if err != nil {
		h.logger.Warn("解密 channel access_hash 失败", "peer_ref", peerRef, "error", err)
		return 0, false, nil
	}

	var accessHash int64
	fmt.Sscanf(plain, "%d", &accessHash)
	return accessHash, true, nil
}

// SetChannelAccessHash 设置频道的 access_hash（加密存储）。
func (h *HashStore) SetChannelAccessHash(ctx context.Context, userID, channelID, accessHash int64) error {
	peerRef := fmt.Sprintf("ch_%d", channelID)

	// 加密 access_hash
	plain := fmt.Sprintf("%d", accessHash)
	encrypted, err := crypto.EncryptString(h.key, plain, []byte("atria:chat_peer:v1"))
	if err != nil {
		return fmt.Errorf("加密 channel access_hash 失败: %w", err)
	}

	// 更新 ChatPeerCache
	var existing model.ChatPeerCache
	err = h.db.Where("peer_ref = ? AND account_id = ?", peerRef, uint(userID)).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		// 创建新记录
		cache := model.ChatPeerCache{
			AccountID:           uint(userID),
			PeerRef:             peerRef,
			PeerType:            "channel",
			PeerID:              channelID,
			AccessHashEncrypted: encrypted,
		}
		return h.db.Create(&cache).Error
	}
	if err != nil {
		return fmt.Errorf("查询 channel peer cache 失败: %w", err)
	}

	return h.db.Model(&existing).Update("access_hash_encrypted", encrypted).Error
}

// 确保 HashStore 实现 updates.ChannelAccessHasher。
var _ updates.ChannelAccessHasher = (*HashStore)(nil)
