// Package chat 提供聊天服务抽象。
package chat

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/gotd/td/tg"
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
	dialFunc   func(ctx context.Context, network, addr string) (interface{}, error)
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

// ListDialogs 获取最近会话列表。
func (s *ChatService) ListDialogs(accountID uint, limit int) ([]Dialog, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	account, cred, err := s.getAccountAndCredential(accountID)
	if err != nil {
		return nil, err
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		return nil, fmt.Errorf("解密 API Hash 失败")
	}

	client := mtproto.NewGotdClient(s.sessionDir, s.key, s.flowStore, s.logger)

	var dialogs []Dialog
	err = client.RunWithSession(context.Background(), int(cred.APIID), apiHash, account.Session.SessionFilePath, func(ctx context.Context, api *tg.Client) error {
		result, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			Limit: limit,
		})
		if err != nil {
			return err
		}

		switch d := result.(type) {
		case *tg.MessagesDialogs:
			for _, dialog := range d.Dialogs {
				dlg := convertDialog(dialog, d.Messages, d.Users, d.Chats)
				if dlg != nil {
					dialogs = append(dialogs, *dlg)
				}
			}
		case *tg.MessagesDialogsSlice:
			for _, dialog := range d.Dialogs {
				dlg := convertDialog(dialog, d.Messages, d.Users, d.Chats)
				if dlg != nil {
					dialogs = append(dialogs, *dlg)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, s.classifyError(err)
	}

	return dialogs, nil
}

// GetMessages 获取指定会话的最近消息。
func (s *ChatService) GetMessages(accountID uint, peerRef string, limit int) ([]Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if peerRef == "" {
		return nil, &ChatError{Code: "peer_invalid", Message: "会话引用不能为空"}
	}

	account, cred, err := s.getAccountAndCredential(accountID)
	if err != nil {
		return nil, err
	}

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		return nil, fmt.Errorf("解密 API Hash 失败")
	}

	peerID, peerType := decodePeerRef(peerRef)
	if peerID == 0 {
		return nil, &ChatError{Code: "peer_invalid", Message: "无效的会话引用"}
	}

	client := mtproto.NewGotdClient(s.sessionDir, s.key, s.flowStore, s.logger)

	var messages []Message
	err = client.RunWithSession(context.Background(), int(cred.APIID), apiHash, account.Session.SessionFilePath, func(ctx context.Context, api *tg.Client) error {
		peer := buildInputPeer(peerID, peerType, 0)
		if peer == nil {
			return fmt.Errorf("无法构建 peer")
		}

		result, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:  peer,
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
		return nil, s.classifyError(err)
	}

	return messages, nil
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

	apiHash, err := security.DecryptAPIHash(s.key, cred.EncryptedAPIHash)
	if err != nil {
		return nil, fmt.Errorf("解密 API Hash 失败")
	}

	peerID, peerType := decodePeerRef(peerRef)
	if peerID == 0 {
		return nil, &ChatError{Code: "peer_invalid", Message: "无效的会话引用"}
	}

	client := mtproto.NewGotdClient(s.sessionDir, s.key, s.flowStore, s.logger)

	var result *SendResult
	err = client.RunWithSession(context.Background(), int(cred.APIID), apiHash, account.Session.SessionFilePath, func(ctx context.Context, api *tg.Client) error {
		peer := buildInputPeer(peerID, peerType, 0)
		if peer == nil {
			return fmt.Errorf("无法构建 peer")
		}

		randomID := rand.Int63()
		apiResult, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
			Peer:     peer,
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

// getAccountAndCredential 获取账号和关联的 API 凭据。
func (s *ChatService) getAccountAndCredential(accountID uint) (*model.TelegramAccount, *model.APICredential, error) {
	var account model.TelegramAccount
	err := s.db.Preload("Session").Where("id = ? AND status IN ?", accountID, []string{"active", "logged_out"}).
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
func (s *ChatService) classifyError(err error) error {
	if err == nil {
		return nil
	}

	errKind := mtproto.ClassifyError(err)

	switch errKind {
	case mtproto.ErrProxyConnectFailed:
		return &ChatError{Code: "proxy_connect_failed", Message: "无法连接代理，请检查 API 网络代理配置"}
	case mtproto.ErrProxyAuthFailed:
		return &ChatError{Code: "proxy_auth_failed", Message: "代理认证失败，请检查用户名和密码"}
	case mtproto.ErrTelegramTimeout:
		return &ChatError{Code: "telegram_timeout", Message: "连接 Telegram 超时，请稍后重试或检查代理"}
	case mtproto.ErrSessionInvalid:
		return &ChatError{Code: "session_invalid", Message: "账号登录状态已失效，请重新接入"}
	case mtproto.ErrUnauthorized:
		return &ChatError{Code: "session_invalid", Message: "账号登录状态已失效，请重新接入"}
	case mtproto.ErrCredentialDisabled:
		return &ChatError{Code: "api_key_invalid", Message: "Telegram API Key 不可用，请检查 API ID / API Hash"}
	default:
		s.logger.Warn("未分类的 Telegram 错误", "error_kind", string(errKind))
		return &ChatError{Code: "telegram_error", Message: "Telegram 返回异常，请稍后重试或检查日志"}
	}
}

// ChatError 聊天错误。
type ChatError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ChatError) Error() string {
	return e.Message
}

// buildInputPeer 构建 InputPeerClass。
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

// convertDialog 转换 gotd Dialog 为内部 Dialog。
func convertDialog(dialog tg.DialogClass, messages []tg.MessageClass, users []tg.UserClass, chats []tg.ChatClass) *Dialog {
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

	switch p := d.Peer.(type) {
	case *tg.PeerUser:
		dlg.PeerType = PeerTypeUser
		for _, u := range users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				dlg.Title = buildDisplayName(user.FirstName, user.LastName)
				dlg.Username = user.Username
				dlg.AvatarPlaceholder = getInitial(dlg.Title)
				break
			}
		}
	case *tg.PeerChat:
		dlg.PeerType = PeerTypeChat
		for _, c := range chats {
			if chat, ok := c.(*tg.Chat); ok && chat.ID == p.ChatID {
				dlg.Title = chat.Title
				dlg.AvatarPlaceholder = getInitial(dlg.Title)
				break
			}
		}
	case *tg.PeerChannel:
		dlg.PeerType = PeerTypeChannel
		for _, c := range chats {
			if channel, ok := c.(*tg.Channel); ok && channel.ID == p.ChannelID {
				dlg.Title = channel.Title
				dlg.Username = channel.Username
				dlg.AvatarPlaceholder = getInitial(dlg.Title)
				break
			}
		}
	}

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
