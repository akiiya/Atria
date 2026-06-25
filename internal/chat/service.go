// Package chat 提供聊天服务抽象。
package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/security"
	"github.com/user/atria/internal/telegramclient"

	"gorm.io/gorm"
)

// ChatService 实现聊天服务。
// 通过 telegramclient.ClientAdapter 与 Telegram 通信，不直接依赖 gotd 类型。
type ChatService struct {
	db      *gorm.DB
	key     []byte
	adapter telegramclient.ClientAdapter
	logger  *slog.Logger
}

// NewChatService 创建聊天服务。
func NewChatService(db *gorm.DB, key []byte, adapter telegramclient.ClientAdapter, logger *slog.Logger) *ChatService {
	return &ChatService{
		db:      db,
		key:     key,
		adapter: adapter,
		logger:  logger,
	}
}

// ListDialogs 获取最近会话列表（cache-first）。
// forceRefresh=true 时跳过缓存直接调 Telegram；否则有缓存立即返回。
func (s *ChatService) ListDialogs(ctx context.Context, accountID uint, limit int, forceRefresh bool) (*DialogsResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	start := time.Now()

	// 先尝试从缓存读取
	cached := s.listDialogsFromCache(accountID, limit)

	// cache-first：有缓存且非强制刷新时，立即返回缓存
	if !forceRefresh && len(cached) > 0 {
		s.logger.Info("ListDialogs 缓存命中",
			"operation", "list_dialogs",
			"account_id", accountID,
			"source", "cache",
			"count", len(cached),
			"duration_ms", msSince(start),
		)
		return &DialogsResult{Dialogs: cached, Source: "cache", Stale: true}, nil
	}

	account, cred, err := s.getAccountAndCredential(accountID)
	if err != nil {
		if len(cached) > 0 {
			return &DialogsResult{Dialogs: cached, Source: "cache", Stale: true}, nil
		}
		return nil, err
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		if len(cached) > 0 {
			return &DialogsResult{Dialogs: cached, Source: "cache", Stale: true}, nil
		}
		return nil, &ChatError{Code: "api_key_invalid", Message: "解密 API Hash 失败"}
	}

	s.logger.Info("ListDialogs 开始",
		"operation", "list_dialogs",
		"account_id", accountID,
		"session_configured", account.Session != nil,
		"api_id_present", cred.APIID > 0,
		"force_refresh", forceRefresh,
		"cache_count", len(cached),
	)

	// 通过 adapter 获取会话列表
	page, err := s.adapter.ListDialogs(ctx, telegramclient.ListDialogsRequest{
		AccountID:       accountID,
		Limit:           limit,
		APIID:           int(cred.APIID),
		APIHash:         apiHash,
		SessionFilePath: account.Session.SessionFilePath,
	})
	if err != nil {
		// Telegram 刷新失败，返回缓存（如有）
		if len(cached) > 0 {
			s.logger.Warn("Telegram 刷新失败，返回缓存", "error", err, "duration_ms", msSince(start))
			return &DialogsResult{Dialogs: cached, Source: "cache", Stale: true}, nil
		}
		return nil, s.classifyError(err)
	}

	// 缓存 peer 信息
	for _, dlg := range page.Dialogs {
		s.upsertPeerCacheFromDialog(accountID, &dlg)
	}

	// 转换为内部 Dialog 类型
	dialogs := make([]Dialog, 0, len(page.Dialogs))
	for _, d := range page.Dialogs {
		dialogs = append(dialogs, mapNeutralDialogToChatDialog(d))
	}

	source := "telegram"
	if len(cached) > 0 {
		source = "mixed"
	}

	s.logger.Info("ListDialogs 完成",
		"operation", "list_dialogs",
		"account_id", accountID,
		"source", source,
		"count", len(dialogs),
		"duration_ms", msSince(start),
	)

	return &DialogsResult{Dialogs: dialogs, Source: source, Stale: false}, nil
}

// GetMessages 获取指定会话的最近消息（cache-first）。
// forceRefresh=true 时跳过缓存直接调 Telegram；否则有缓存立即返回。
func (s *ChatService) GetMessages(ctx context.Context, accountID uint, peerRef string, limit int, forceRefresh bool) (*MessagesResult, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if peerRef == "" {
		return nil, &ChatError{Code: "peer_invalid", Message: "会话引用不能为空"}
	}

	start := time.Now()

	// 先尝试从缓存读取
	cached := s.getMessagesFromCache(accountID, peerRef, limit)

	// cache-first：有缓存且非强制刷新时，立即返回缓存
	if !forceRefresh && len(cached) > 0 {
		s.logger.Info("GetMessages 缓存命中",
			"operation", "get_messages",
			"account_id", accountID,
			"peer_ref", peerRef,
			"source", "cache",
			"count", len(cached),
			"duration_ms", msSince(start),
		)
		return &MessagesResult{
			Messages: cached,
			Source:   "cache",
			Stale:    true,
			HasOlder: len(cached) >= limit,
		}, nil
	}

	account, cred, err := s.getAccountAndCredential(accountID)
	if err != nil {
		if len(cached) > 0 {
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true}, nil
		}
		return nil, err
	}

	cache, err := s.getPeerCache(accountID, peerRef)
	if err != nil {
		if len(cached) > 0 {
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true}, nil
		}
		return nil, err
	}

	// 解密 access_hash（user/channel 类型必须有）
	var accessHash int64
	if PeerType(cache.PeerType) == PeerTypeUser || PeerType(cache.PeerType) == PeerTypeChannel {
		if cache.AccessHashEncrypted == "" {
			if len(cached) > 0 {
				return &MessagesResult{Messages: cached, Source: "cache", Stale: true}, nil
			}
			return nil, &ChatError{Code: "peer_incomplete", Message: "会话信息不完整，请刷新会话列表"}
		}
		accessHash, err = s.decryptAccessHash(cache.AccessHashEncrypted)
		if err != nil {
			if len(cached) > 0 {
				return &MessagesResult{Messages: cached, Source: "cache", Stale: true}, nil
			}
			return nil, &ChatError{Code: "peer_incomplete", Message: "会话信息解密失败，请刷新会话列表"}
		}
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		if len(cached) > 0 {
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true}, nil
		}
		return nil, &ChatError{Code: "api_key_invalid", Message: "解密 API Hash 失败"}
	}

	s.logger.Info("GetMessages 开始",
		"operation", "get_messages",
		"account_id", accountID,
		"peer_ref", peerRef,
		"force_refresh", forceRefresh,
		"cache_count", len(cached),
	)

	// 通过 adapter 获取消息
	page, err := s.adapter.GetRecentMessages(ctx, telegramclient.GetRecentMessagesRequest{
		AccountID:       accountID,
		PeerRef:         peerRef,
		Limit:           limit,
		APIID:           int(cred.APIID),
		APIHash:         apiHash,
		SessionFilePath: account.Session.SessionFilePath,
		PeerID:          cache.PeerID,
		PeerType:        telegramclient.PeerType(cache.PeerType),
		AccessHash:      accessHash,
	})
	if err != nil {
		if len(cached) > 0 {
			s.logger.Warn("Telegram 刷新消息失败，返回缓存", "error", err, "peer_ref", peerRef, "duration_ms", msSince(start))
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true}, nil
		}
		return nil, s.classifyError(err)
	}

	// 转换为内部 Message 类型
	messages := make([]Message, 0, len(page.Messages))
	for _, m := range page.Messages {
		messages = append(messages, mapNeutralMessageToChatMessage(m))
	}

	// 按 sent_at 正序排列
	sortMessagesByTime(messages)

	// 缓存消息
	s.cacheMessages(accountID, peerRef, messages)

	source := "telegram"
	if len(cached) > 0 {
		source = "mixed"
	}
	result := &MessagesResult{
		Messages: messages,
		Source:   source,
		Stale:    false,
		HasOlder: page.HasOlder,
	}
	if len(messages) > 0 {
		result.OldestMessageID = messages[0].MessageID
		result.NewestMessageID = messages[len(messages)-1].MessageID
	}

	s.logger.Info("GetMessages 完成",
		"operation", "get_messages",
		"account_id", accountID,
		"peer_ref", peerRef,
		"source", source,
		"count", len(messages),
		"duration_ms", msSince(start),
	)

	return result, nil
}

// LoadOlderMessages 加载指定会话更早的消息（cache-first + adapter fallback）。
// forceRefresh=true 时跳过缓存直接调 Telegram。
func (s *ChatService) LoadOlderMessages(ctx context.Context, accountID uint, peerRef string, beforeMessageID int, limit int, forceRefresh bool) (*MessagesResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if peerRef == "" {
		return nil, &ChatError{Code: "peer_invalid", Message: "会话引用不能为空"}
	}
	if beforeMessageID <= 0 {
		return nil, &ChatError{Code: "peer_invalid", Message: "before_message_id 无效"}
	}

	// 先从缓存读取 before_id 之前的消息
	cached := s.getMessagesBeforeFromCache(accountID, peerRef, beforeMessageID, limit)

	// cache-first：有缓存且非强制刷新时，立即返回缓存
	if !forceRefresh && len(cached) > 0 {
		return &MessagesResult{Messages: cached, Source: "cache", Stale: true, HasOlder: len(cached) >= limit}, nil
	}

	account, cred, err := s.getAccountAndCredential(accountID)
	if err != nil {
		if len(cached) > 0 {
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true, HasOlder: len(cached) >= limit}, nil
		}
		return nil, err
	}

	cache, err := s.getPeerCache(accountID, peerRef)
	if err != nil {
		if len(cached) > 0 {
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true, HasOlder: len(cached) >= limit}, nil
		}
		return nil, err
	}

	// 解密 access_hash
	var accessHash int64
	if PeerType(cache.PeerType) == PeerTypeUser || PeerType(cache.PeerType) == PeerTypeChannel {
		if cache.AccessHashEncrypted == "" {
			if len(cached) > 0 {
				return &MessagesResult{Messages: cached, Source: "cache", Stale: true, HasOlder: len(cached) >= limit}, nil
			}
			return nil, &ChatError{Code: "peer_incomplete", Message: "会话信息不完整，请刷新会话列表"}
		}
		accessHash, err = s.decryptAccessHash(cache.AccessHashEncrypted)
		if err != nil {
			if len(cached) > 0 {
				return &MessagesResult{Messages: cached, Source: "cache", Stale: true, HasOlder: len(cached) >= limit}, nil
			}
			return nil, &ChatError{Code: "peer_incomplete", Message: "会话信息解密失败，请刷新会话列表"}
		}
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		if len(cached) > 0 {
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true, HasOlder: len(cached) >= limit}, nil
		}
		return nil, &ChatError{Code: "api_key_invalid", Message: "解密 API Hash 失败"}
	}

	// 通过 adapter 加载更早消息
	page, err := s.adapter.LoadOlderMessages(ctx, telegramclient.LoadOlderMessagesRequest{
		AccountID:       accountID,
		PeerRef:         peerRef,
		BeforeMessageID: int64(beforeMessageID),
		Limit:           limit,
		APIID:           int(cred.APIID),
		APIHash:         apiHash,
		SessionFilePath: account.Session.SessionFilePath,
		PeerID:          cache.PeerID,
		PeerType:        telegramclient.PeerType(cache.PeerType),
		AccessHash:      accessHash,
	})
	if err != nil {
		if len(cached) > 0 {
			s.logger.Warn("Telegram 加载更早消息失败，返回缓存", "error", err, "peer_ref", peerRef)
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true, HasOlder: len(cached) >= limit}, nil
		}
		return nil, s.classifyError(err)
	}

	// 转换为内部 Message 类型
	messages := make([]Message, 0, len(page.Messages))
	for _, m := range page.Messages {
		messages = append(messages, mapNeutralMessageToChatMessage(m))
	}

	// 按 sent_at 正序排列
	sortMessagesByTime(messages)

	// 缓存消息
	s.cacheMessages(accountID, peerRef, messages)

	result := &MessagesResult{
		Messages: messages,
		Source:   string(telegramclient.DataSourceTelegram),
		Stale:    false,
		HasOlder: page.HasOlder,
	}
	if len(messages) > 0 {
		result.OldestMessageID = messages[0].MessageID
		result.NewestMessageID = messages[len(messages)-1].MessageID
	}
	return result, nil
}

// SendText 发送文本消息。
func (s *ChatService) SendText(ctx context.Context, accountID uint, peerRef string, text string) (*SendResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, &ChatError{Code: "text_empty", Message: "消息内容不能为空"}
	}
	if len(text) > 4096 {
		return nil, &ChatError{Code: "text_too_long", Message: "消息内容不能超过 4096 个字符"}
	}
	if peerRef == "" {
		return nil, &ChatError{Code: "peer_invalid", Message: "会话引用不能为空"}
	}

	account, cred, err := s.getAccountAndCredential(accountID)
	if err != nil {
		return nil, err
	}

	cache, err := s.getPeerCache(accountID, peerRef)
	if err != nil {
		return nil, err
	}

	// 解密 access_hash（user/channel 类型必须有）
	var accessHash int64
	if PeerType(cache.PeerType) == PeerTypeUser || PeerType(cache.PeerType) == PeerTypeChannel {
		if cache.AccessHashEncrypted == "" {
			return nil, &ChatError{Code: "peer_incomplete", Message: "会话信息不完整，请刷新会话列表"}
		}
		accessHash, err = s.decryptAccessHash(cache.AccessHashEncrypted)
		if err != nil {
			return nil, &ChatError{Code: "peer_incomplete", Message: "会话信息解密失败，请刷新会话列表"}
		}
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		return nil, &ChatError{Code: "api_key_invalid", Message: "解密 API Hash 失败"}
	}

	s.logger.Info("发送消息", "text_len", len(text), "peer_ref", peerRef)

	// 通过 adapter 发送消息
	result, err := s.adapter.SendText(ctx, telegramclient.SendTextRequest{
		AccountID:       accountID,
		PeerRef:         peerRef,
		Text:            text,
		ClientRandomID:  crypto.SecureRandomInt64(),
		APIID:           int(cred.APIID),
		APIHash:         apiHash,
		SessionFilePath: account.Session.SessionFilePath,
		PeerID:          cache.PeerID,
		PeerType:        telegramclient.PeerType(cache.PeerType),
		AccessHash:      accessHash,
	})
	if err != nil {
		return nil, s.classifyError(err)
	}

	return &SendResult{
		MessageID:         result.MessageID,
		TelegramMessageID: result.MessageID,
		SentAt:            result.SentAt,
		Status:            result.Status,
		Direction:         result.Direction,
		Text:              result.Text,
	}, nil
}

// getPeerCache 从缓存获取 peer 信息，验证 account_id 匹配。
func (s *ChatService) getPeerCache(accountID uint, peerRef string) (*model.ChatPeerCache, error) {
	var cache model.ChatPeerCache
	err := s.db.Where("peer_ref = ? AND account_id = ?", peerRef, accountID).First(&cache).Error
	if err == gorm.ErrRecordNotFound {
		return nil, &ChatError{Code: "peer_invalid", Message: "会话不存在或已过期，请刷新会话列表"}
	}
	if err != nil {
		return nil, fmt.Errorf("查询 peer 缓存失败: %w", err)
	}
	return &cache, nil
}

// encryptAccessHash 加密 access_hash。
func (s *ChatService) encryptAccessHash(accessHash int64) (string, error) {
	plain := fmt.Sprintf("%d", accessHash)
	return crypto.EncryptString(s.key, plain, []byte("atria:chat_peer:v1"))
}

// decryptAccessHash 解密 access_hash。
func (s *ChatService) decryptAccessHash(encrypted string) (int64, error) {
	plain, err := crypto.DecryptString(s.key, encrypted, []byte("atria:chat_peer:v1"))
	if err != nil {
		return 0, err
	}
	var hash int64
	fmt.Sscanf(plain, "%d", &hash)
	return hash, nil
}

// listDialogsFromCache 从 peer 缓存读取会话列表。
func (s *ChatService) listDialogsFromCache(accountID uint, limit int) []Dialog {
	var peers []model.ChatPeerCache
	if err := s.db.Where("account_id = ?", accountID).
		Order("is_pinned DESC, last_message_at DESC").
		Limit(limit).Find(&peers).Error; err != nil {
		return nil
	}
	if len(peers) == 0 {
		return nil
	}

	dialogs := make([]Dialog, 0, len(peers))
	for _, p := range peers {
		dlg := Dialog{
			PeerRef:            p.PeerRef,
			PeerType:           PeerType(p.PeerType),
			Title:              p.Title,
			Username:           p.Username,
			AvatarPlaceholder:  getInitial(p.Title),
			LastMessagePreview: p.LastMessagePreview,
			UnreadCount:        p.UnreadCount,
			IsPinned:           p.IsPinned,
			IsMuted:            p.IsMuted,
		}
		if p.LastMessageAt != nil {
			dlg.LastMessageAt = *p.LastMessageAt
		}
		dialogs = append(dialogs, dlg)
	}
	return dialogs
}

// getMessagesFromCache 从消息缓存读取最近消息。
func (s *ChatService) getMessagesFromCache(accountID uint, peerRef string, limit int) []Message {
	var cached []model.ChatMessageCache
	if err := s.db.Where("account_id = ? AND peer_ref = ?", accountID, peerRef).
		Order("telegram_message_id DESC").Limit(limit).Find(&cached).Error; err != nil {
		return nil
	}
	if len(cached) == 0 {
		return nil
	}

	return s.decryptCachedMessages(cached)
}

// getMessagesBeforeFromCache 从消息缓存读取 before_id 之前的消息。
func (s *ChatService) getMessagesBeforeFromCache(accountID uint, peerRef string, beforeMessageID int, limit int) []Message {
	var cached []model.ChatMessageCache
	if err := s.db.Where("account_id = ? AND peer_ref = ? AND telegram_message_id < ?", accountID, peerRef, beforeMessageID).
		Order("telegram_message_id DESC").Limit(limit).Find(&cached).Error; err != nil {
		return nil
	}
	if len(cached) == 0 {
		return nil
	}

	return s.decryptCachedMessages(cached)
}

// decryptCachedMessages 解密缓存消息并按时间正序返回。
func (s *ChatService) decryptCachedMessages(cached []model.ChatMessageCache) []Message {
	messages := make([]Message, 0, len(cached))
	for i := len(cached) - 1; i >= 0; i-- {
		c := cached[i]
		msg := Message{
			MessageID:         c.TelegramMessageID,
			TelegramMessageID: c.TelegramMessageID,
			PeerRef:           c.PeerRef,
			Direction:         MessageDirection(c.Direction),
			SenderName:        c.SenderName,
			SentAt:            c.SentAt,
			IsOutgoing:        c.Direction == "out",
			Status:            MessageStatusSent,
			MessageType:       c.Kind,
		}
		// 解密消息正文
		if c.TextEncrypted != "" {
			text, err := crypto.DecryptString(s.key, c.TextEncrypted, []byte("atria:msg:v1"))
			if err == nil {
				msg.Text = text
			}
		}
		messages = append(messages, msg)
	}
	return messages
}

// cacheMessages 缓存消息到数据库。
func (s *ChatService) cacheMessages(accountID uint, peerRef string, messages []Message) {
	if len(messages) == 0 {
		return
	}

	// 限制每个 peer 最多缓存 500 条
	const maxCachePerPeer = 500

	for _, msg := range messages {
		// 加密消息正文
		textEncrypted := ""
		if msg.Text != "" {
			encrypted, err := crypto.EncryptString(s.key, msg.Text, []byte("atria:msg:v1"))
			if err != nil {
				s.logger.Warn("加密消息正文失败", "error", err)
				continue
			}
			textEncrypted = encrypted
		}

		cache := model.ChatMessageCache{
			AccountID:         accountID,
			PeerRef:           peerRef,
			TelegramMessageID: msg.MessageID,
			Direction:         string(msg.Direction),
			SenderName:        msg.SenderName,
			Kind:              msg.MessageType,
			TextEncrypted:     textEncrypted,
			SentAt:            msg.SentAt,
		}

		// Upsert by (account_id, peer_ref, telegram_message_id)
		var existing model.ChatMessageCache
		err := s.db.Where("account_id = ? AND peer_ref = ? AND telegram_message_id = ?",
			accountID, peerRef, msg.MessageID).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			s.db.Create(&cache)
		} else if err == nil {
			s.db.Model(&existing).Updates(map[string]any{
				"text_encrypted": textEncrypted,
				"sender_name":    msg.SenderName,
				"kind":           msg.MessageType,
				"sent_at":        msg.SentAt,
			})
		}
	}

	// 清理旧缓存，只保留最近 maxCachePerPeer 条
	var count int64
	s.db.Model(&model.ChatMessageCache{}).
		Where("account_id = ? AND peer_ref = ?", accountID, peerRef).
		Count(&count)
	if count > maxCachePerPeer {
		// 删除最旧的记录
		s.db.Exec(`DELETE FROM chat_message_cache
			WHERE account_id = ? AND peer_ref = ? AND id NOT IN (
				SELECT id FROM chat_message_cache
				WHERE account_id = ? AND peer_ref = ?
				ORDER BY sent_at DESC LIMIT ?
			)`, accountID, peerRef, accountID, peerRef, maxCachePerPeer)
	}
}

// getAccountAndCredential 获取账号和关联的 API 凭据。
func (s *ChatService) getAccountAndCredential(accountID uint) (*model.TelegramAccount, *model.APICredential, error) {
	var account model.TelegramAccount
	err := s.db.Preload("Session").Where("id = ? AND status = ?", accountID, model.TelegramAccountStatusActive).
		First(&account).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, &ChatError{Code: "session_invalid", Message: "当前账号 Session 不可用，请重新接入"}
		}
		return nil, nil, fmt.Errorf("查询账号失败: %w", err)
	}

	if account.Session == nil {
		return nil, nil, &ChatError{Code: "session_invalid", Message: "当前账号 Session 不可用，请重新接入"}
	}

	var cred model.APICredential
	if err := s.db.First(&cred, account.APICredentialID).Error; err != nil {
		return nil, nil, &ChatError{Code: "api_key_invalid", Message: "Telegram API Key 不可用"}
	}

	return &account, &cred, nil
}

// upsertPeerCacheFromDialog 从 adapter 返回的 Dialog 更新 peer 缓存。
func (s *ChatService) upsertPeerCacheFromDialog(accountID uint, dlg *telegramclient.Dialog) {
	if dlg.PeerRef == "" {
		return
	}

	// chat 类型不需要 access_hash
	var encryptedHash string
	if dlg.PeerType == telegramclient.PeerTypeUser || dlg.PeerType == telegramclient.PeerTypeChannel {
		if dlg.AccessHash == 0 {
			s.logger.Warn("peer 缺少 access_hash，跳过缓存", "peer_ref", dlg.PeerRef, "peer_type", string(dlg.PeerType))
			return
		}
		encrypted, err := s.encryptAccessHash(dlg.AccessHash)
		if err != nil {
			s.logger.Error("加密 access_hash 失败", "error", err, "peer_ref", dlg.PeerRef)
			return
		}
		encryptedHash = encrypted
	}

	cache := model.ChatPeerCache{
		AccountID:           accountID,
		PeerRef:             dlg.PeerRef,
		PeerType:            string(dlg.PeerType),
		PeerID:              dlg.PeerID,
		AccessHashEncrypted: encryptedHash,
		Title:               dlg.Title,
		Username:            dlg.Username,
		IsPinned:            dlg.IsPinned,
		IsMuted:             dlg.IsMuted,
	}
	if !dlg.LastMessageAt.IsZero() {
		cache.LastMessageAt = &dlg.LastMessageAt
	}
	cache.LastMessagePreview = dlg.LastMessagePreview

	// Upsert
	var existing model.ChatPeerCache
	err := s.db.Where("peer_ref = ? AND account_id = ?", dlg.PeerRef, accountID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		s.db.Create(&cache)
	} else if err == nil {
		s.db.Model(&existing).Updates(map[string]any{
			"access_hash_encrypted": encryptedHash,
			"title":                 dlg.Title,
			"username":              dlg.Username,
			"is_pinned":             dlg.IsPinned,
			"is_muted":              dlg.IsMuted,
			"last_message_at":       cache.LastMessageAt,
			"last_message_preview":  dlg.LastMessagePreview,
		})
	}
}

// classifyError 分类错误为 ChatError。
func (s *ChatService) classifyError(err error) error {
	if err == nil {
		return nil
	}

	if _, ok := err.(*ChatError); ok {
		return err
	}

	// 检查中立错误
	var tcErr *telegramclient.Error
	if errors.As(err, &tcErr) {
		return &ChatError{
			Code:    string(tcErr.Code),
			Message: tcErr.Message,
		}
	}

	// 检查 context 错误
	if errors.Is(err, context.DeadlineExceeded) {
		return &ChatError{Code: "telegram_timeout", Message: "连接 Telegram 超时，请稍后重试或检查代理"}
	}
	if errors.Is(err, context.Canceled) {
		return &ChatError{Code: "telegram_timeout", Message: "连接已取消"}
	}

	// 检查代理相关错误
	if isProxyError(err) {
		return classifyProxyError(err)
	}

	// 检查 net.Error（超时/连接失败）
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return &ChatError{Code: "telegram_timeout", Message: "连接 Telegram 超时，请稍后重试或检查代理"}
		}
		return &ChatError{Code: "network_error", Message: "网络异常，请检查网络连接或代理配置"}
	}

	// 未知错误
	s.logger.Warn("未分类的聊天错误",
		"error_type", fmt.Sprintf("%T", err),
		"error_summary", sanitizeErrorForLog(err.Error()),
	)
	return &ChatError{Code: "telegram_error", Message: "Telegram 返回异常，请稍后重试或检查日志"}
}

// isProxyError 检查错误是否与代理相关。
func isProxyError(err error) bool {
	if err == nil {
		return false
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}
	errStr := err.Error()
	return strings.Contains(errStr, "proxy") ||
		strings.Contains(errStr, "SOCKS") ||
		strings.Contains(errStr, "CONNECT") ||
		strings.Contains(errStr, "407")
}

// classifyProxyError 分类代理错误。
func classifyProxyError(err error) *ChatError {
	errStr := err.Error()
	if strings.Contains(errStr, "auth") || strings.Contains(errStr, "407") {
		return &ChatError{Code: "proxy_auth_failed", Message: "代理认证失败，请检查代理用户名和密码"}
	}
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline") {
		return &ChatError{Code: "telegram_timeout", Message: "连接 Telegram 超时，请稍后重试或检查代理"}
	}
	return &ChatError{Code: "proxy_connect_failed", Message: "无法连接代理，请检查 API 网络代理配置"}
}

// sanitizeErrorForLog 安全脱敏错误消息用于日志。
func sanitizeErrorForLog(msg string) string {
	if len(msg) > 200 {
		return msg[:200] + "..."
	}
	return msg
}

// ChatError 聊天错误。
type ChatError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ChatError) Error() string {
	return e.Message
}

// GetContacts 获取联系人列表（cache-first）。
// 联系人缓存在 chat_peer_cache 表中（peer_type=user），
// 同时确保 peer_cache 包含 access_hash，以便无 dialog 联系人也能正常进入聊天。
func (s *ChatService) GetContacts(ctx context.Context, accountID uint, forceRefresh bool) (*ContactsResult, error) {
	start := time.Now()

	// cache-first：从 chat_peer_cache 读取已缓存的联系人
	cachedContacts := s.getContactsFromCache(accountID)
	if !forceRefresh && len(cachedContacts) > 0 {
		s.logger.Info("GetContacts 缓存命中",
			"operation", "get_contacts",
			"account_id", accountID,
			"source", "cache",
			"count", len(cachedContacts),
			"duration_ms", msSince(start),
		)
		return &ContactsResult{Contacts: cachedContacts, Source: "cache", Stale: true}, nil
	}

	account, cred, err := s.getAccountAndCredential(accountID)
	if err != nil {
		if len(cachedContacts) > 0 {
			return &ContactsResult{Contacts: cachedContacts, Source: "cache", Stale: true}, nil
		}
		return nil, err
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		if len(cachedContacts) > 0 {
			return &ContactsResult{Contacts: cachedContacts, Source: "cache", Stale: true}, nil
		}
		return nil, &ChatError{Code: "api_key_invalid", Message: "解密 API Hash 失败"}
	}

	result, err := s.adapter.GetContacts(ctx, telegramclient.GetContactsRequest{
		AccountID:       accountID,
		APIID:           int(cred.APIID),
		APIHash:         apiHash,
		SessionFilePath: account.Session.SessionFilePath,
	})
	if err != nil {
		if len(cachedContacts) > 0 {
			s.logger.Warn("Telegram 获取联系人失败，返回缓存", "error", err, "duration_ms", msSince(start))
			return &ContactsResult{Contacts: cachedContacts, Source: "cache", Stale: true}, nil
		}
		return nil, s.classifyError(err)
	}

	// 将联系人写入 peer_cache，确保 access_hash 可用于后续聊天
	for _, c := range result.Contacts {
		s.upsertPeerCacheFromContact(accountID, &c)
	}

	// 构建联系人列表，判断 has_dialog
	contacts := s.buildContactsList(accountID, result.Contacts)

	source := "telegram"
	if len(cachedContacts) > 0 {
		source = "mixed"
	}

	s.logger.Info("GetContacts 完成",
		"operation", "get_contacts",
		"account_id", accountID,
		"source", source,
		"count", len(contacts),
		"duration_ms", msSince(start),
	)

	return &ContactsResult{Contacts: contacts, Source: source, Stale: false}, nil
}

// getContactsFromCache 从 chat_peer_cache 读取联系人（peer_type=user）。
func (s *ChatService) getContactsFromCache(accountID uint) []Contact {
	var peers []model.ChatPeerCache
	if err := s.db.Where("account_id = ? AND peer_type = ?", accountID, "user").
		Order("title ASC").Find(&peers).Error; err != nil {
		return nil
	}
	if len(peers) == 0 {
		return nil
	}

	// 构建 dialog 集合（有 last_message_at 的 peer 视为已有 dialog）
	dialogRefs := make(map[string]bool)
	var allPeers []model.ChatPeerCache
	if err := s.db.Where("account_id = ?", accountID).Find(&allPeers).Error; err == nil {
		for _, p := range allPeers {
			if p.LastMessageAt != nil {
				dialogRefs[p.PeerRef] = true
			}
		}
	}

	contacts := make([]Contact, 0, len(peers))
	for _, p := range peers {
		contacts = append(contacts, Contact{
			PeerRef:       p.PeerRef,
			DisplayName:   p.Title,
			Username:      p.Username,
			AvatarInitial: getInitial(p.Title),
			HasDialog:     dialogRefs[p.PeerRef],
		})
	}
	return contacts
}

// upsertPeerCacheFromContact 从联系人数据写入 peer_cache。
// 确保 access_hash 被加密保存，以便后续聊天 API 使用。
func (s *ChatService) upsertPeerCacheFromContact(accountID uint, c *telegramclient.Contact) {
	if c.PeerRef == "" || c.AccessHash == 0 {
		return
	}

	encrypted, err := s.encryptAccessHash(c.AccessHash)
	if err != nil {
		s.logger.Error("加密联系人 access_hash 失败", "error", err, "peer_ref", c.PeerRef)
		return
	}

	cache := model.ChatPeerCache{
		AccountID:           accountID,
		PeerRef:             c.PeerRef,
		PeerType:            string(c.PeerType),
		PeerID:              c.PeerID,
		AccessHashEncrypted: encrypted,
		Title:               c.DisplayName,
		Username:            c.Username,
	}

	var existing model.ChatPeerCache
	err = s.db.Where("peer_ref = ? AND account_id = ?", c.PeerRef, accountID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		s.db.Create(&cache)
	} else if err == nil {
		s.db.Model(&existing).Updates(map[string]any{
			"access_hash_encrypted": encrypted,
			"title":                 c.DisplayName,
			"username":              c.Username,
		})
	}
}

// buildContactsList 从 Telegram 返回的联系人构建最终列表。
func (s *ChatService) buildContactsList(accountID uint, tcContacts []telegramclient.Contact) []Contact {
	// 构建 dialog 集合
	dialogRefs := make(map[string]bool)
	var peers []model.ChatPeerCache
	if err := s.db.Where("account_id = ?", accountID).Find(&peers).Error; err == nil {
		for _, p := range peers {
			if p.LastMessageAt != nil {
				dialogRefs[p.PeerRef] = true
			}
		}
	}

	contacts := make([]Contact, 0, len(tcContacts))
	for _, c := range tcContacts {
		contacts = append(contacts, Contact{
			PeerRef:       c.PeerRef,
			DisplayName:   c.DisplayName,
			Username:      c.Username,
			Phone:         c.Phone,
			AvatarInitial: c.AvatarText,
			HasDialog:     dialogRefs[c.PeerRef],
		})
	}

	sort.Slice(contacts, func(i, j int) bool {
		return contacts[i].DisplayName < contacts[j].DisplayName
	})

	return contacts
}

// mapNeutralDialogToChatDialog 将中立 Dialog DTO 转换为 chat 包内部 Dialog。
func mapNeutralDialogToChatDialog(d telegramclient.Dialog) Dialog {
	avatar := d.AvatarText
	if avatar == "" {
		avatar = getInitial(d.Title)
	}
	return Dialog{
		PeerRef:            d.PeerRef,
		PeerType:           PeerType(d.PeerType),
		Title:              d.Title,
		Username:           d.Username,
		AvatarPlaceholder:  avatar,
		LastMessagePreview: d.LastMessagePreview,
		UnreadCount:        d.UnreadCount,
		IsPinned:           d.IsPinned,
		IsMuted:            d.IsMuted,
		LastMessageAt:      d.LastMessageAt,
	}
}

// mapNeutralMessageToChatMessage 将中立 Message DTO 转换为 chat 包内部 Message。
func mapNeutralMessageToChatMessage(m telegramclient.Message) Message {
	return Message{
		MessageID:         m.TelegramMessageID,
		TelegramMessageID: m.TelegramMessageID,
		PeerRef:           m.PeerRef,
		Direction:         MessageDirection(m.Direction),
		SenderName:        m.SenderName,
		Text:              m.Text,
		SentAt:            m.SentAt,
		IsOutgoing:        m.IsOutgoing,
		Status:            MessageStatus(m.Status),
		MessageType:       string(m.Kind),
	}
}

// decodePeerRef 解析 peer 引用。
func decodePeerRef(ref string) (int64, PeerType) {
	if strings.HasPrefix(ref, "u_") {
		var id int64
		fmt.Sscanf(ref, "u_%d", &id)
		return id, PeerTypeUser
	}
	if strings.HasPrefix(ref, "c_") {
		var id int64
		fmt.Sscanf(ref, "c_%d", &id)
		return id, PeerTypeChat
	}
	if strings.HasPrefix(ref, "ch_") {
		var id int64
		fmt.Sscanf(ref, "ch_%d", &id)
		return id, PeerTypeChannel
	}
	return 0, ""
}

// buildDisplayName 构建显示名。
func buildDisplayName(firstName, lastName string) string {
	name := strings.TrimSpace(firstName + " " + lastName)
	if name == "" {
		return "未知用户"
	}
	return name
}

// getInitial 获取名称首字符（grapheme 安全）。
//
// 正确处理：
//   - 国旗 emoji（regional indicator pair，如 🇺🇸）
//   - 基本 emoji + variation selector（如 ❤️）
//   - ZWJ 序列（如 👨‍👩‍👧‍👦）
//   - 普通字母、数字、CJK
func getInitial(name string) string {
	if name == "" {
		return "?"
	}
	r := []rune(name)
	if len(r) == 0 {
		return "?"
	}

	// Regional Indicator Pair（国旗 emoji）：U+1F1E6..U+1F1FF
	// 两个连续 regional indicator 组成一个国旗
	if isRegionalIndicator(r[0]) && len(r) > 1 && isRegionalIndicator(r[1]) {
		return string(r[:2])
	}

	// emoji + variation selector (U+FE0F) 或 ZWJ 序列
	end := 1
	for end < len(r) {
		// Variation Selector (U+FE0E, U+FE0F)
		if isVariationSelector(r[end]) {
			end++
			continue
		}
		// ZWJ (U+200D) + 后续字符
		if r[end] == 0x200D && end+1 < len(r) {
			end += 2 // 跳过 ZWJ 和下一个字符
			continue
		}
		// Skin Tone Modifier (U+1F3FB..U+1F3FF)
		if isSkinToneModifier(r[end]) {
			end++
			continue
		}
		break
	}
	return string(r[:end])
}

func isRegionalIndicator(r rune) bool {
	return r >= 0x1F1E6 && r <= 0x1F1FF
}

func isVariationSelector(r rune) bool {
	return r == 0xFE0E || r == 0xFE0F
}

func isSkinToneModifier(r rune) bool {
	return r >= 0x1F3FB && r <= 0x1F3FF
}

// sortMessagesByTime 按 sent_at 正序排列消息。
func sortMessagesByTime(msgs []Message) {
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].SentAt.Before(msgs[j].SentAt)
	})
}

// truncateText 截断文本（rune 安全，不会截断多字节字符或 emoji）。
func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}

// Ensure ChatService implements Service.
var _ Service = (*ChatService)(nil)

// msSince 返回从 start 到现在的毫秒数，用于安全计时日志。
func msSince(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
