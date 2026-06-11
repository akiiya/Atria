// Package chat 提供聊天服务抽象。
package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/model"
	"github.com/user/atria/internal/mtproto"
	"github.com/user/atria/internal/security"

	"gorm.io/gorm"
)

// ChatService 实现聊天服务。
type ChatService struct {
	db         *gorm.DB
	sessionDir string
	key        []byte
	flowStore  mtproto.FlowStore
	dialFunc   dcs.DialFunc
	logger     *slog.Logger
}

// NewChatService 创建聊天服务。
func NewChatService(db *gorm.DB, sessionDir string, key []byte, flowStore mtproto.FlowStore, logger *slog.Logger) *ChatService {
	return &ChatService{
		db:         db,
		sessionDir: sessionDir,
		key:        key,
		flowStore:  flowStore,
		logger:     logger,
	}
}

// SetProxyDialer 设置代理拨号函数。
func (s *ChatService) SetProxyDialer(fn dcs.DialFunc) {
	s.dialFunc = fn
}

// ListDialogs 获取最近会话列表（cache-first）。
func (s *ChatService) ListDialogs(accountID uint, limit int) (*DialogsResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// 先尝试从缓存读取
	cached := s.listDialogsFromCache(accountID, limit)

	account, cred, err := s.getAccountAndCredential(accountID)
	if err != nil {
		if cached != nil {
			return &DialogsResult{Dialogs: cached, Source: "cache", Stale: true}, nil
		}
		return nil, err
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		if cached != nil {
			return &DialogsResult{Dialogs: cached, Source: "cache", Stale: true}, nil
		}
		return nil, &ChatError{Code: "api_key_invalid", Message: "解密 API Hash 失败"}
	}

	client := mtproto.NewGotdClient(s.sessionDir, s.key, s.flowStore, s.logger)
	if s.dialFunc != nil {
		client.SetDialer(s.dialFunc)
	}

	s.logger.Info("ListDialogs 开始",
		"operation", "list_dialogs",
		"account_id", accountID,
		"session_configured", account.Session != nil,
		"dialer_configured", s.dialFunc != nil,
		"api_id_present", cred.APIID > 0,
	)

	var dialogs []Dialog
	err = client.RunWithSession(context.Background(), int(cred.APIID), apiHash, account.Session.SessionFilePath, func(ctx context.Context, api *tg.Client) error {
		result, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			Limit:      limit,
			OffsetPeer: &tg.InputPeerEmpty{},
		})
		if err != nil {
			return err
		}

		switch d := result.(type) {
		case *tg.MessagesDialogs:
			for _, dialog := range d.Dialogs {
				dlg := s.convertAndCacheDialog(accountID, dialog, d.Messages, d.Users, d.Chats)
				if dlg != nil {
					dialogs = append(dialogs, *dlg)
				}
			}
		case *tg.MessagesDialogsSlice:
			for _, dialog := range d.Dialogs {
				dlg := s.convertAndCacheDialog(accountID, dialog, d.Messages, d.Users, d.Chats)
				if dlg != nil {
					dialogs = append(dialogs, *dlg)
				}
			}
		}
		return nil
	})
	if err != nil {
		// Telegram 刷新失败，返回缓存（如有）
		if cached != nil {
			s.logger.Warn("Telegram 刷新失败，返回缓存", "error", err)
			return &DialogsResult{Dialogs: cached, Source: "cache", Stale: true}, nil
		}
		return nil, s.classifyError(err)
	}

	source := "telegram"
	if cached != nil {
		source = "mixed"
	}
	return &DialogsResult{Dialogs: dialogs, Source: source, Stale: false}, nil
}

// GetMessages 获取指定会话的最近消息（cache-first）。
func (s *ChatService) GetMessages(accountID uint, peerRef string, limit int) (*MessagesResult, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if peerRef == "" {
		return nil, &ChatError{Code: "peer_invalid", Message: "会话引用不能为空"}
	}

	// 先尝试从缓存读取
	cached := s.getMessagesFromCache(accountID, peerRef, limit)

	account, cred, err := s.getAccountAndCredential(accountID)
	if err != nil {
		if cached != nil {
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true}, nil
		}
		return nil, err
	}

	cache, err := s.getPeerCache(accountID, peerRef)
	if err != nil {
		if cached != nil {
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true}, nil
		}
		return nil, err
	}

	inputPeer, err := s.buildInputPeerFromCache(cache)
	if err != nil {
		if cached != nil {
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true}, nil
		}
		return nil, err
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		if cached != nil {
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true}, nil
		}
		return nil, &ChatError{Code: "api_key_invalid", Message: "解密 API Hash 失败"}
	}

	client := mtproto.NewGotdClient(s.sessionDir, s.key, s.flowStore, s.logger)
	if s.dialFunc != nil {
		client.SetDialer(s.dialFunc)
	}

	var messages []Message
	err = client.RunWithSession(context.Background(), int(cred.APIID), apiHash, account.Session.SessionFilePath, func(ctx context.Context, api *tg.Client) error {
		result, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:  inputPeer,
			Limit: limit,
		})
		if err != nil {
			return err
		}

		switch m := result.(type) {
		case *tg.MessagesChannelMessages:
			for _, msg := range m.Messages {
				if m2, ok := msg.(*tg.Message); ok {
					messages = append(messages, convertMessage(m2))
				}
			}
		case *tg.MessagesMessages:
			for _, msg := range m.Messages {
				if m2, ok := msg.(*tg.Message); ok {
					messages = append(messages, convertMessage(m2))
				}
			}
		case *tg.MessagesMessagesSlice:
			for _, msg := range m.Messages {
				if m2, ok := msg.(*tg.Message); ok {
					messages = append(messages, convertMessage(m2))
				}
			}
		}
		return nil
	})
	if err != nil {
		if cached != nil {
			s.logger.Warn("Telegram 刷新消息失败，返回缓存", "error", err, "peer_ref", peerRef)
			return &MessagesResult{Messages: cached, Source: "cache", Stale: true}, nil
		}
		return nil, s.classifyError(err)
	}

	// 缓存消息
	s.cacheMessages(accountID, peerRef, messages)

	source := "telegram"
	if cached != nil {
		source = "mixed"
	}
	return &MessagesResult{Messages: messages, Source: source, Stale: false}, nil
}

// SendText 发送文本消息。
func (s *ChatService) SendText(accountID uint, peerRef string, text string) (*SendResult, error) {
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

	// 从缓存获取 peer 信息
	cache, err := s.getPeerCache(accountID, peerRef)
	if err != nil {
		return nil, err
	}

	inputPeer, err := s.buildInputPeerFromCache(cache)
	if err != nil {
		return nil, err
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		return nil, &ChatError{Code: "api_key_invalid", Message: "解密 API Hash 失败"}
	}

	client := mtproto.NewGotdClient(s.sessionDir, s.key, s.flowStore, s.logger)
	if s.dialFunc != nil {
		client.SetDialer(s.dialFunc)
	}

	s.logger.Info("发送消息", "text_len", len(text), "peer_ref", peerRef)

	var result *SendResult
	err = client.RunWithSession(context.Background(), int(cred.APIID), apiHash, account.Session.SessionFilePath, func(ctx context.Context, api *tg.Client) error {
		randomID := crypto.SecureRandomInt64()
		apiResult, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
			Peer:     inputPeer,
			Message:  text,
			RandomID: randomID,
		})
		if err != nil {
			return err
		}

		msgID := 0
		switch r := apiResult.(type) {
		case *tg.Updates:
			for _, update := range r.Updates {
				if u, ok := update.(*tg.UpdateNewMessage); ok {
					if m, ok := u.Message.(*tg.Message); ok {
						msgID = m.ID
					}
				}
			}
		case *tg.UpdateShortSentMessage:
			msgID = r.ID
		}

		result = &SendResult{
			MessageID: msgID,
			SentAt:    time.Now(),
			Status:    "sent",
			Direction: "out",
			Text:      text,
		}
		return nil
	})
	if err != nil {
		return nil, s.classifyError(err)
	}

	return result, nil
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

// buildInputPeerFromCache 从缓存构造 InputPeerClass。
func (s *ChatService) buildInputPeerFromCache(cache *model.ChatPeerCache) (tg.InputPeerClass, error) {
	peerType := PeerType(cache.PeerType)

	switch peerType {
	case PeerTypeUser:
		if cache.AccessHashEncrypted == "" {
			return nil, &ChatError{Code: "peer_incomplete", Message: "会话信息不完整，请刷新会话列表"}
		}
		accessHash, err := s.decryptAccessHash(cache.AccessHashEncrypted)
		if err != nil {
			return nil, &ChatError{Code: "peer_incomplete", Message: "会话信息解密失败，请刷新会话列表"}
		}
		return &tg.InputPeerUser{UserID: cache.PeerID, AccessHash: accessHash}, nil

	case PeerTypeChat:
		// chat 类型不需要 access_hash
		return &tg.InputPeerChat{ChatID: cache.PeerID}, nil

	case PeerTypeChannel:
		if cache.AccessHashEncrypted == "" {
			return nil, &ChatError{Code: "peer_incomplete", Message: "会话信息不完整，请刷新会话列表"}
		}
		accessHash, err := s.decryptAccessHash(cache.AccessHashEncrypted)
		if err != nil {
			return nil, &ChatError{Code: "peer_incomplete", Message: "会话信息解密失败，请刷新会话列表"}
		}
		return &tg.InputPeerChannel{ChannelID: cache.PeerID, AccessHash: accessHash}, nil
	}

	return nil, &ChatError{Code: "peer_invalid", Message: "无效的会话类型"}
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

// getMessagesFromCache 从消息缓存读取消息。
func (s *ChatService) getMessagesFromCache(accountID uint, peerRef string, limit int) []Message {
	var cached []model.ChatMessageCache
	if err := s.db.Where("account_id = ? AND peer_ref = ?", accountID, peerRef).
		Order("sent_at DESC").Limit(limit).Find(&cached).Error; err != nil {
		return nil
	}
	if len(cached) == 0 {
		return nil
	}

	messages := make([]Message, 0, len(cached))
	for i := len(cached) - 1; i >= 0; i-- {
		c := cached[i]
		msg := Message{
			MessageID:   c.TelegramMessageID,
			PeerRef:     c.PeerRef,
			Direction:   MessageDirection(c.Direction),
			SenderName:  c.SenderName,
			SentAt:      c.SentAt,
			IsOutgoing:  c.Direction == "out",
			Status:      MessageStatusSent,
			MessageType: c.Kind,
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

	// 限制每个 peer 最多缓存 100 条
	const maxCachePerPeer = 100

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

// classifyError 分类错误。
// 使用 tgerr.As 从 error chain 中提取 Telegram RPC 错误，
// 不再依赖 mtproto.ClassifyError 的粗暴兜底。
func (s *ChatService) classifyError(err error) error {
	if err == nil {
		return nil
	}

	if _, ok := err.(*ChatError); ok {
		return err
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

	// 使用 tgerr.As 从 error chain 中提取 Telegram RPC 错误
	if rpcErr, ok := tgerr.As(err); ok {
		s.logger.Warn("chat RPC 错误",
			"rpc_code", rpcErr.Code,
			"rpc_type", rpcErr.Type,
		)
		return classifyRPCErrorForChat(rpcErr)
	}

	// 检查 net.Error（超时/连接失败）
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return &ChatError{Code: "telegram_timeout", Message: "连接 Telegram 超时，请稍后重试或检查代理"}
		}
		return &ChatError{Code: "network_error", Message: "网络异常，请检查网络连接或代理配置"}
	}

	// 检查是否是 mtproto.MTProtoError
	if mtprotoErr, ok := err.(*mtproto.MTProtoError); ok {
		return classifyMTProtoErrorForChat(mtprotoErr)
	}

	// 未知错误：归类为 telegram_error，不是 network_error
	s.logger.Warn("未分类的聊天错误",
		"error_type", fmt.Sprintf("%T", err),
		"error_summary", sanitizeErrorForLog(err.Error()),
	)
	return &ChatError{Code: "telegram_error", Message: "Telegram 返回异常，请稍后重试或检查日志"}
}

// classifyRPCErrorForChat 根据 Telegram RPC 错误类型分类（聊天场景）。
func classifyRPCErrorForChat(rpcErr *tgerr.Error) *ChatError {
	switch rpcErr.Type {
	case "AUTH_KEY_UNREGISTERED", "AUTH_KEY_INVALID":
		return &ChatError{Code: "session_invalid", Message: "当前账号 Session 已失效，请重新接入"}
	case "SESSION_REVOKED", "SESSION_EXPIRED":
		return &ChatError{Code: "session_invalid", Message: "当前账号 Session 已失效，请重新接入"}
	case "USER_DEACTIVATED", "USER_DEACTIVATED_BAN":
		return &ChatError{Code: "account_deactivated", Message: "该 Telegram 账号不可用或已被停用"}
	case "API_ID_INVALID":
		return &ChatError{Code: "api_key_invalid", Message: "Telegram API Key 不可用，请检查 API ID / API Hash"}
	case "API_HASH_INVALID":
		return &ChatError{Code: "api_key_invalid", Message: "Telegram API Hash 不可用"}
	case "FLOOD_WAIT":
		return &ChatError{Code: "flood_wait", Message: "Telegram 限制请求过快，请稍后再试"}
	case "AUTH_RESTART":
		return &ChatError{Code: "auth_restart", Message: "Telegram 要求重新开始认证，请重新接入账号"}
	case "TIMEOUT":
		return &ChatError{Code: "telegram_timeout", Message: "连接 Telegram 超时，请稍后重试或检查代理"}
	case "INTERNAL":
		return &ChatError{Code: "telegram_error", Message: "Telegram 内部错误，请稍后重试"}
	default:
		return &ChatError{Code: "telegram_error", Message: fmt.Sprintf("Telegram 返回错误 (%s)，请稍后重试", rpcErr.Type)}
	}
}

// classifyMTProtoErrorForChat 分类 mtproto.MTProtoError（聊天场景）。
func classifyMTProtoErrorForChat(mtprotoErr *mtproto.MTProtoError) *ChatError {
	switch mtprotoErr.Kind {
	case mtproto.ErrProxyConnectFailed:
		return &ChatError{Code: "proxy_connect_failed", Message: "无法连接代理，请检查 API 网络代理配置"}
	case mtproto.ErrProxyAuthFailed:
		return &ChatError{Code: "proxy_auth_failed", Message: "代理认证失败，请检查代理用户名和密码"}
	case mtproto.ErrTelegramTimeout:
		return &ChatError{Code: "telegram_timeout", Message: "连接 Telegram 超时，请稍后重试或检查代理"}
	case mtproto.ErrSessionInvalid, mtproto.ErrSessionContextLost:
		return &ChatError{Code: "session_invalid", Message: "当前账号 Session 已失效，请重新接入"}
	case mtproto.ErrUnauthorized:
		return &ChatError{Code: "session_invalid", Message: "当前账号 Session 已失效，请重新接入"}
	case mtproto.ErrCredentialDisabled:
		return &ChatError{Code: "api_key_invalid", Message: "Telegram API Key 不可用，请检查 API ID / API Hash"}
	case mtproto.ErrFloodWait:
		return &ChatError{Code: "flood_wait", Message: "Telegram 限制请求过快，请稍后再试"}
	case mtproto.ErrTelegramError:
		return &ChatError{Code: "telegram_error", Message: "Telegram 返回异常，请稍后重试或检查日志"}
	case mtproto.ErrNetworkError:
		return &ChatError{Code: "network_error", Message: "网络异常，请检查网络连接或代理配置"}
	default:
		return &ChatError{Code: "telegram_error", Message: "Telegram 返回异常，请稍后重试或检查日志"}
	}
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

// buildInputPeer 构建 InputPeerClass（用于缓存写入时的临时构造）。
func buildInputPeer(peerID int64, peerType PeerType, accessHash int64) tg.InputPeerClass {
	switch peerType {
	case PeerTypeUser:
		return &tg.InputPeerUser{UserID: peerID, AccessHash: accessHash}
	case PeerTypeChat:
		return &tg.InputPeerChat{ChatID: peerID}
	case PeerTypeChannel:
		return &tg.InputPeerChannel{ChannelID: peerID, AccessHash: accessHash}
	}
	return nil
}

// convertAndCacheDialog 转换 gotd Dialog 为内部 Dialog，同时缓存 peer 信息。
func (s *ChatService) convertAndCacheDialog(accountID uint, dialog tg.DialogClass, messages []tg.MessageClass, users []tg.UserClass, chats []tg.ChatClass) *Dialog {
	d, ok := dialog.(*tg.Dialog)
	if !ok {
		return nil
	}

	peerRef := encodePeerRef(d.Peer)
	if peerRef == "" {
		return nil
	}

	dlg := &Dialog{
		PeerRef:     peerRef,
		UnreadCount: d.UnreadCount,
		IsPinned:    d.Pinned,
	}

	var peerID int64
	var peerType PeerType
	var accessHash int64

	switch p := d.Peer.(type) {
	case *tg.PeerUser:
		peerType = PeerTypeUser
		peerID = p.UserID
		dlg.PeerType = PeerTypeUser
		for _, u := range users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				dlg.Title = buildDisplayName(user.FirstName, user.LastName)
				dlg.Username = user.Username
				dlg.AvatarPlaceholder = getInitial(dlg.Title)
				accessHash = user.AccessHash
				break
			}
		}
	case *tg.PeerChat:
		peerType = PeerTypeChat
		peerID = p.ChatID
		dlg.PeerType = PeerTypeChat
		for _, c := range chats {
			if chat, ok := c.(*tg.Chat); ok && chat.ID == p.ChatID {
				dlg.Title = chat.Title
				dlg.AvatarPlaceholder = getInitial(dlg.Title)
				break
			}
		}
	case *tg.PeerChannel:
		peerType = PeerTypeChannel
		peerID = p.ChannelID
		dlg.PeerType = PeerTypeChannel
		for _, c := range chats {
			if channel, ok := c.(*tg.Channel); ok && channel.ID == p.ChannelID {
				dlg.Title = channel.Title
				dlg.Username = channel.Username
				dlg.AvatarPlaceholder = getInitial(dlg.Title)
				accessHash = channel.AccessHash
				break
			}
		}
	}

	// 缓存 peer 信息（Muted 状态需要从 NotifySettings 获取，此处暂不处理）
	s.upsertPeerCache(accountID, peerRef, peerType, peerID, accessHash, dlg.Title, dlg.Username, d.Pinned, false)

	for _, msg := range messages {
		if m, ok := msg.(*tg.Message); ok && m.ID == d.TopMessage {
			dlg.LastMessagePreview = truncateText(m.Message, 50)
			dlg.LastMessageAt = time.Unix(int64(m.Date), 0)
			break
		}
	}

	if dlg.Title == "" {
		dlg.Title = "未知会话"
		dlg.AvatarPlaceholder = "?"
	}

	return dlg
}

// upsertPeerCache 创建或更新 peer 缓存。
func (s *ChatService) upsertPeerCache(accountID uint, peerRef string, peerType PeerType, peerID int64, accessHash int64, title, username string, isPinned bool, muted bool) {
	// chat 类型不需要 access_hash
	var encryptedHash string
	if peerType == PeerTypeUser || peerType == PeerTypeChannel {
		if accessHash == 0 {
			s.logger.Warn("peer 缺少 access_hash，跳过缓存", "peer_ref", peerRef, "peer_type", string(peerType))
			return
		}
		encrypted, err := s.encryptAccessHash(accessHash)
		if err != nil {
			s.logger.Error("加密 access_hash 失败", "error", err, "peer_ref", peerRef)
			return
		}
		encryptedHash = encrypted
	}

	cache := model.ChatPeerCache{
		AccountID:           accountID,
		PeerRef:             peerRef,
		PeerType:            string(peerType),
		PeerID:              peerID,
		AccessHashEncrypted: encryptedHash,
		Title:               title,
		Username:            username,
		IsPinned:            isPinned,
		IsMuted:             muted,
	}

	// Upsert: 先尝试更新，不存在则创建
	var existing model.ChatPeerCache
	err := s.db.Where("peer_ref = ? AND account_id = ?", peerRef, accountID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		s.db.Create(&cache)
	} else if err == nil {
		s.db.Model(&existing).Updates(map[string]any{
			"access_hash_encrypted": encryptedHash,
			"title":                 title,
			"username":              username,
			"is_pinned":             isPinned,
			"is_muted":              muted,
		})
	}
}

// convertMessage 转换 gotd Message 为内部 Message。
func convertMessage(m *tg.Message) Message {
	msg := Message{
		MessageID:  m.ID,
		SentAt:     time.Unix(int64(m.Date), 0),
		IsOutgoing: m.Out,
		Status:     MessageStatusSent,
	}

	if m.Out {
		msg.Direction = MessageDirectionOut
	} else {
		msg.Direction = MessageDirectionIn
	}

	if m.Message != "" {
		msg.Text = m.Message
		msg.MessageType = "text"
	} else {
		msg.Text = ""
		msg.MessageType = "unsupported"
	}

	return msg
}

// encodePeerRef 将 peer 编码为不透明的引用字符串。
func encodePeerRef(peer tg.PeerClass) string {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return fmt.Sprintf("u_%d", p.UserID)
	case *tg.PeerChat:
		return fmt.Sprintf("c_%d", p.ChatID)
	case *tg.PeerChannel:
		return fmt.Sprintf("ch_%d", p.ChannelID)
	}
	return ""
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

// getInitial 获取名称首字母。
func getInitial(name string) string {
	if name == "" {
		return "?"
	}
	r := []rune(name)
	return strings.ToUpper(string(r[0]))
}

// truncateText 截断文本。
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// Ensure ChatService implements Service.
var _ Service = (*ChatService)(nil)
