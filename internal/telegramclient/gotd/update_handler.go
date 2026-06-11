package gotd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gotd/td/tg"
	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/telegramclient"

	"gorm.io/gorm"
)

// UpdateHandler 实现 telegram.UpdateHandler 接口。
// 接收 gotd updates.Manager 排序后的 updates，
// 映射为中立 UpdateEvent，写入缓存，发布到 EventBus。
type UpdateHandler struct {
	accountID uint
	db        *gorm.DB
	key       []byte
	bus       *telegramclient.EventBus
	logger    *slog.Logger
	onEvent   func() // 每次处理 update 时调用，用于更新 runtime lastEvent
}

// NewUpdateHandler 创建 UpdateHandler。
func NewUpdateHandler(accountID uint, db *gorm.DB, key []byte, bus *telegramclient.EventBus, logger *slog.Logger, onEvent func()) *UpdateHandler {
	return &UpdateHandler{
		accountID: accountID,
		db:        db,
		key:       key,
		bus:       bus,
		logger:    logger,
		onEvent:   onEvent,
	}
}

// Handle 处理排序后的 updates。
// 这是 telegram.UpdateHandler 接口的实现。
func (h *UpdateHandler) Handle(ctx context.Context, u tg.UpdatesClass) error {
	updates, users, chats := extractUpdates(u)
	if len(updates) == 0 {
		return nil
	}

	_ = chats // 当前不使用 chats 信息

	for _, update := range updates {
		if err := h.handleSingleUpdate(ctx, update, users); err != nil {
			h.logger.Warn("处理 update 失败",
				"account_id", h.accountID,
				"error", err,
			)
			// 不中断处理其他 updates
		}
	}

	// 更新 runtime 的 lastEvent 时间
	if h.onEvent != nil {
		h.onEvent()
	}

	return nil
}

// handleSingleUpdate 处理单个 update。
func (h *UpdateHandler) handleSingleUpdate(ctx context.Context, update tg.UpdateClass, users []tg.UserClass) error {
	switch u := update.(type) {
	case *tg.UpdateNewMessage:
		return h.handleNewMessage(ctx, u, users)
	case *tg.UpdateNewChannelMessage:
		return h.handleNewChannelMessage(ctx, u, users)
	case *tg.UpdateEditMessage:
		return h.handleEditMessage(ctx, u)
	case *tg.UpdateDeleteMessages:
		return h.handleDeleteMessages(ctx, u)
	case *tg.UpdateDeleteChannelMessages:
		return h.handleDeleteChannelMessages(ctx, u)
	default:
		// 安全忽略不支持的 update 类型
		h.logger.Debug("忽略不支持的 update 类型",
			"account_id", h.accountID,
			"type", fmt.Sprintf("%T", update),
		)
		return nil
	}
}

// handleNewMessage 处理新消息。
func (h *UpdateHandler) handleNewMessage(ctx context.Context, u *tg.UpdateNewMessage, users []tg.UserClass) error {
	msg, ok := u.Message.(*tg.Message)
	if !ok {
		return nil
	}

	neutralMsg, event := mapUpdateNewMessage(msg, users)
	event.AccountID = h.accountID

	// 写入缓存
	h.upsertMessageCache(neutralMsg)
	h.updateDialogPreview(neutralMsg)

	// 发布事件
	h.bus.Publish(h.accountID, event)

	h.logger.Info("新消息处理完成",
		"account_id", h.accountID,
		"peer_ref", event.PeerRef,
		"message_id", neutralMsg.TelegramMessageID,
		"text_len", len(neutralMsg.Text),
	)

	return nil
}

// handleNewChannelMessage 处理频道新消息。
func (h *UpdateHandler) handleNewChannelMessage(ctx context.Context, u *tg.UpdateNewChannelMessage, users []tg.UserClass) error {
	msg, ok := u.Message.(*tg.Message)
	if !ok {
		return nil
	}

	neutralMsg, event := mapUpdateNewChannelMessage(msg, users)
	event.AccountID = h.accountID

	// 写入缓存
	h.upsertMessageCache(neutralMsg)
	h.updateDialogPreview(neutralMsg)

	// 发布事件
	h.bus.Publish(h.accountID, event)

	h.logger.Info("频道新消息处理完成",
		"account_id", h.accountID,
		"peer_ref", event.PeerRef,
		"message_id", neutralMsg.TelegramMessageID,
		"text_len", len(neutralMsg.Text),
	)

	return nil
}

// handleEditMessage 处理编辑消息。
func (h *UpdateHandler) handleEditMessage(ctx context.Context, u *tg.UpdateEditMessage) error {
	msg, ok := u.Message.(*tg.Message)
	if !ok {
		return nil
	}

	neutralMsg, event := mapUpdateEditMessage(msg)
	event.AccountID = h.accountID

	// 更新缓存
	h.updateMessageCache(neutralMsg)

	// 发布事件
	h.bus.Publish(h.accountID, event)

	h.logger.Info("消息编辑处理完成",
		"account_id", h.accountID,
		"peer_ref", event.PeerRef,
		"message_id", neutralMsg.TelegramMessageID,
	)

	return nil
}

// handleDeleteMessages 处理删除消息。
func (h *UpdateHandler) handleDeleteMessages(ctx context.Context, u *tg.UpdateDeleteMessages) error {
	// 从 IDs 中提取消息 ID
	msgIDs := make([]int, 0, len(u.Messages))
	for _, id := range u.Messages {
		msgIDs = append(msgIDs, id)
	}

	// 删除缓存
	h.deleteMessageCache(msgIDs)

	// 发布事件（peerRef 为空，因为 UpdateDeleteMessages 不包含 peer 信息）
	event := mapUpdateDeleteMessages("", msgIDs)
	event.AccountID = h.accountID
	h.bus.Publish(h.accountID, event)

	h.logger.Info("消息删除处理完成",
		"account_id", h.accountID,
		"count", len(msgIDs),
	)

	return nil
}

// handleDeleteChannelMessages 处理频道消息删除。
func (h *UpdateHandler) handleDeleteChannelMessages(ctx context.Context, u *tg.UpdateDeleteChannelMessages) error {
	msgIDs := make([]int, 0, len(u.Messages))
	for _, id := range u.Messages {
		msgIDs = append(msgIDs, id)
	}

	peerRef := fmt.Sprintf("ch_%d", u.ChannelID)

	// 删除缓存
	h.deleteMessageCache(msgIDs)

	// 发布事件
	event := mapUpdateDeleteMessages(peerRef, msgIDs)
	event.AccountID = h.accountID
	h.bus.Publish(h.accountID, event)

	h.logger.Info("频道消息删除处理完成",
		"account_id", h.accountID,
		"peer_ref", peerRef,
		"count", len(msgIDs),
	)

	return nil
}

// upsertMessageCache 将消息写入 ChatMessageCache。
// 正文使用 AES-256-GCM 加密。
func (h *UpdateHandler) upsertMessageCache(msg telegramclient.Message) {
	textEncrypted := ""
	if msg.Text != "" {
		encrypted, err := crypto.EncryptString(h.key, msg.Text, []byte("atria:msg:v1"))
		if err != nil {
			h.logger.Warn("加密消息正文失败", "error", err)
			return
		}
		textEncrypted = encrypted
	}

	cache := model.ChatMessageCache{
		AccountID:         h.accountID,
		PeerRef:           msg.PeerRef,
		TelegramMessageID: msg.TelegramMessageID,
		Direction:         string(msg.Direction),
		SenderName:        msg.SenderName,
		Kind:              string(msg.Kind),
		TextEncrypted:     textEncrypted,
		SentAt:            msg.SentAt,
	}

	// Upsert
	var existing model.ChatMessageCache
	err := h.db.Where("account_id = ? AND peer_ref = ? AND telegram_message_id = ?",
		h.accountID, msg.PeerRef, msg.TelegramMessageID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		h.db.Create(&cache)
	} else if err == nil {
		h.db.Model(&existing).Updates(map[string]any{
			"text_encrypted": textEncrypted,
			"sender_name":    msg.SenderName,
			"kind":           string(msg.Kind),
			"sent_at":        msg.SentAt,
		})
	}
}

// updateMessageCache 更新已存在的消息缓存。
func (h *UpdateHandler) updateMessageCache(msg telegramclient.Message) {
	textEncrypted := ""
	if msg.Text != "" {
		encrypted, err := crypto.EncryptString(h.key, msg.Text, []byte("atria:msg:v1"))
		if err != nil {
			h.logger.Warn("加密消息正文失败", "error", err)
			return
		}
		textEncrypted = encrypted
	}

	h.db.Model(&model.ChatMessageCache{}).
		Where("account_id = ? AND peer_ref = ? AND telegram_message_id = ?",
			h.accountID, msg.PeerRef, msg.TelegramMessageID).
		Updates(map[string]any{
			"text_encrypted": textEncrypted,
			"sender_name":    msg.SenderName,
			"kind":           string(msg.Kind),
		})
}

// deleteMessageCache 从缓存中删除消息。
func (h *UpdateHandler) deleteMessageCache(msgIDs []int) {
	if len(msgIDs) == 0 {
		return
	}
	h.db.Where("account_id = ? AND telegram_message_id IN ?", h.accountID, msgIDs).
		Delete(&model.ChatMessageCache{})
}

// updateDialogPreview 更新 ChatPeerCache 的最后消息预览。
func (h *UpdateHandler) updateDialogPreview(msg telegramclient.Message) {
	preview := truncateText(msg.Text, 50)

	h.db.Model(&model.ChatPeerCache{}).
		Where("account_id = ? AND peer_ref = ?", h.accountID, msg.PeerRef).
		Updates(map[string]any{
			"last_message_preview": preview,
			"last_message_at":      &msg.SentAt,
		})
}

// 确保 UpdateHandler 实现 telegram.UpdateHandler。
// 注意：这里使用函数签名检查，因为 telegram.UpdateHandler 在 gotd 包中。
