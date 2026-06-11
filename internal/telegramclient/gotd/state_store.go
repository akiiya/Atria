package gotd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gotd/td/telegram/updates"
	"github.com/user/atria/internal/model"

	"gorm.io/gorm"
)

// StateStore 实现 gotd updates.StateStorage 接口。
// 将 Telegram update state（pts/qts/date/seq）持久化到 SQLite。
// 按 account_id 隔离，不存储敏感字段。
type StateStore struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewStateStore 创建 StateStore。
func NewStateStore(db *gorm.DB, logger *slog.Logger) *StateStore {
	return &StateStore{db: db, logger: logger}
}

// GetState 获取指定账号的 update state。
func (s *StateStore) GetState(ctx context.Context, userID int64) (updates.State, bool, error) {
	var state model.TelegramUpdateState
	err := s.db.Where("account_id = ?", uint(userID)).First(&state).Error
	if err == gorm.ErrRecordNotFound {
		return updates.State{}, false, nil
	}
	if err != nil {
		return updates.State{}, false, fmt.Errorf("查询 update state 失败: %w", err)
	}
	return updates.State{
		Pts:  state.Pts,
		Qts:  state.Qts,
		Date: state.Date,
		Seq:  state.Seq,
	}, true, nil
}

// SetState 设置指定账号的 update state。
func (s *StateStore) SetState(ctx context.Context, userID int64, state updates.State) error {
	now := time.Now()
	record := model.TelegramUpdateState{
		AccountID:  uint(userID),
		Pts:        state.Pts,
		Qts:        state.Qts,
		Date:       state.Date,
		Seq:        state.Seq,
		LastSyncAt: &now,
	}

	var existing model.TelegramUpdateState
	err := s.db.Where("account_id = ?", uint(userID)).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return s.db.Create(&record).Error
	}
	if err != nil {
		return fmt.Errorf("查询 update state 失败: %w", err)
	}

	return s.db.Model(&existing).Updates(map[string]any{
		"pts":          state.Pts,
		"qts":          state.Qts,
		"date":         state.Date,
		"seq":          state.Seq,
		"last_sync_at": &now,
	}).Error
}

// SetPts 更新 pts。
func (s *StateStore) SetPts(ctx context.Context, userID int64, pts int) error {
	return s.updateField(uint(userID), "pts", pts)
}

// SetQts 更新 qts。
func (s *StateStore) SetQts(ctx context.Context, userID int64, qts int) error {
	return s.updateField(uint(userID), "qts", qts)
}

// SetDate 更新 date。
func (s *StateStore) SetDate(ctx context.Context, userID int64, date int) error {
	return s.updateField(uint(userID), "date", date)
}

// SetSeq 更新 seq。
func (s *StateStore) SetSeq(ctx context.Context, userID int64, seq int) error {
	return s.updateField(uint(userID), "seq", seq)
}

// SetDateSeq 同时更新 date 和 seq。
func (s *StateStore) SetDateSeq(ctx context.Context, userID int64, date, seq int) error {
	now := time.Now()
	return s.db.Model(&model.TelegramUpdateState{}).
		Where("account_id = ?", uint(userID)).
		Updates(map[string]any{
			"date":         date,
			"seq":          seq,
			"last_sync_at": &now,
		}).Error
}

// GetChannelPts 获取频道的 pts。
// 从 ChatPeerCache 中读取（如果存在 channel_pts 字段）。
// 当前实现：返回 not found，让 updates.Manager 使用 getDifference 恢复。
func (s *StateStore) GetChannelPts(ctx context.Context, userID, channelID int64) (int, bool, error) {
	// 当前 ChatPeerCache 不存储 channel_pts。
	// 返回 not found，updates.Manager 会通过 getDifference 恢复。
	return 0, false, nil
}

// SetChannelPts 设置频道的 pts。
// 当前实现：记录到 ChatPeerCache 的扩展字段（如有）。
// 暂时为空操作，后续可在 ChatPeerCache 中添加 channel_pts 字段。
func (s *StateStore) SetChannelPts(ctx context.Context, userID, channelID int64, pts int) error {
	// TODO: 当需要完整 channel state 持久化时，扩展 ChatPeerCache 或新建 channel_state 表
	return nil
}

// ForEachChannels 遍历所有频道的 pts。
// 当前实现：空遍历（无频道 pts 数据）。
func (s *StateStore) ForEachChannels(ctx context.Context, userID int64, f func(ctx context.Context, channelID int64, pts int) error) error {
	// TODO: 当 SetChannelPts 实现后，从数据库遍历频道状态
	return nil
}

// updateField 更新单个字段。
func (s *StateStore) updateField(accountID uint, field string, value int) error {
	now := time.Now()
	return s.db.Model(&model.TelegramUpdateState{}).
		Where("account_id = ?", accountID).
		Updates(map[string]any{
			field:          value,
			"last_sync_at": &now,
		}).Error
}

// 确保 StateStore 实现 updates.StateStorage。
var _ updates.StateStorage = (*StateStore)(nil)
