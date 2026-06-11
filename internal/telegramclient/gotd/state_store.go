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
// 从 TelegramChannelUpdateState 表读取。
func (s *StateStore) GetChannelPts(ctx context.Context, userID, channelID int64) (int, bool, error) {
	var state model.TelegramChannelUpdateState
	err := s.db.Where("account_id = ? AND channel_id = ?", uint(userID), channelID).First(&state).Error
	if err == gorm.ErrRecordNotFound {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("查询 channel pts 失败: %w", err)
	}
	return state.Pts, true, nil
}

// SetChannelPts 设置频道的 pts。
// 持久化到 TelegramChannelUpdateState 表。
func (s *StateStore) SetChannelPts(ctx context.Context, userID, channelID int64, pts int) error {
	now := time.Now()

	var existing model.TelegramChannelUpdateState
	err := s.db.Where("account_id = ? AND channel_id = ?", uint(userID), channelID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		record := model.TelegramChannelUpdateState{
			AccountID:  uint(userID),
			ChannelID:  channelID,
			Pts:        pts,
			LastSyncAt: &now,
		}
		return s.db.Create(&record).Error
	}
	if err != nil {
		return fmt.Errorf("查询 channel state 失败: %w", err)
	}

	return s.db.Model(&existing).Updates(map[string]any{
		"pts":          pts,
		"last_sync_at": &now,
	}).Error
}

// ForEachChannels 遍历所有频道的 pts。
func (s *StateStore) ForEachChannels(ctx context.Context, userID int64, f func(ctx context.Context, channelID int64, pts int) error) error {
	var states []model.TelegramChannelUpdateState
	if err := s.db.Where("account_id = ?", uint(userID)).Find(&states).Error; err != nil {
		return fmt.Errorf("遍历 channel states 失败: %w", err)
	}

	for _, state := range states {
		if err := f(ctx, state.ChannelID, state.Pts); err != nil {
			return err
		}
	}
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
