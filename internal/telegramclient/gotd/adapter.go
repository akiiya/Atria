// Package gotd 实现基于 gotd/td 的 Telegram 客户端适配器。
// gotd 类型只在此包内部使用，不泄漏到上层业务。
package gotd

import (
	"context"
	"log/slog"
	"time"

	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/user/atria/internal/mtproto"
	"github.com/user/atria/internal/telegramclient"
)

// Adapter 是基于 gotd/td 的 ClientAdapter 实现。
type Adapter struct {
	sessionDir string
	key        []byte
	flowStore  mtproto.FlowStore
	logger     *slog.Logger
	dialFunc   dcs.DialFunc
	gate       *AccountGate // per-account 执行锁，可选
}

// NewAdapter 创建 gotd adapter。
func NewAdapter(sessionDir string, key []byte, flowStore mtproto.FlowStore, logger *slog.Logger) *Adapter {
	return &Adapter{
		sessionDir: sessionDir,
		key:        key,
		flowStore:  flowStore,
		logger:     logger,
	}
}

// SetDialer 设置代理拨号函数。
func (a *Adapter) SetDialer(fn dcs.DialFunc) {
	a.dialFunc = fn
}

// SetGate 设置 per-account 执行锁。
// 设置后，所有 API 调用会先获取对应 account 的锁，防止与 runtime 并发。
func (a *Adapter) SetGate(gate *AccountGate) {
	a.gate = gate
}

// acquireGate 获取指定 account 的执行锁。
// 如果 gate 未设置，返回空操作的 unlock 函数。
func (a *Adapter) acquireGate(accountID uint) func() {
	if a.gate == nil {
		return func() {}
	}
	a.gate.Lock(accountID, "rest")
	return func() { a.gate.Unlock(accountID) }
}

// ListDialogs 获取会话列表。
func (a *Adapter) ListDialogs(ctx context.Context, req telegramclient.ListDialogsRequest) (telegramclient.DialogsPage, error) {
	unlock := a.acquireGate(req.AccountID)
	defer unlock()

	client := mtproto.NewGotdClient(a.sessionDir, a.key, a.flowStore, a.logger)
	if a.dialFunc != nil {
		client.SetDialer(a.dialFunc)
	}

	var dialogs []telegramclient.Dialog
	err := client.RunWithSession(ctx, req.APIID, req.APIHash, req.SessionFilePath, func(ctx context.Context, api *tg.Client) error {
		result, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			Limit:      req.Limit,
			OffsetPeer: &tg.InputPeerEmpty{},
		})
		if err != nil {
			return err
		}

		switch d := result.(type) {
		case *tg.MessagesDialogs:
			for _, dialog := range d.Dialogs {
				dlg := mapDialog(dialog, d.Messages, d.Users, d.Chats)
				if dlg != nil {
					dialogs = append(dialogs, *dlg)
				}
			}
		case *tg.MessagesDialogsSlice:
			for _, dialog := range d.Dialogs {
				dlg := mapDialog(dialog, d.Messages, d.Users, d.Chats)
				if dlg != nil {
					dialogs = append(dialogs, *dlg)
				}
			}
		}
		return nil
	})
	if err != nil {
		return telegramclient.DialogsPage{}, classifyError(err)
	}

	return telegramclient.DialogsPage{
		Source:  telegramclient.DataSourceTelegram,
		Stale:   false,
		Dialogs: dialogs,
	}, nil
}

// GetRecentMessages 获取最近消息。
func (a *Adapter) GetRecentMessages(ctx context.Context, req telegramclient.GetRecentMessagesRequest) (telegramclient.MessagesPage, error) {
	unlock := a.acquireGate(req.AccountID)
	defer unlock()

	inputPeer := buildInputPeerFromInfo(req.PeerID, req.PeerType, req.AccessHash)
	if inputPeer == nil {
		return telegramclient.MessagesPage{}, telegramclient.NewError(telegramclient.ErrorCodePeerInvalid, "无效的会话类型")
	}

	client := mtproto.NewGotdClient(a.sessionDir, a.key, a.flowStore, a.logger)
	if a.dialFunc != nil {
		client.SetDialer(a.dialFunc)
	}

	var messages []telegramclient.Message
	err := client.RunWithSession(ctx, req.APIID, req.APIHash, req.SessionFilePath, func(ctx context.Context, api *tg.Client) error {
		result, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:  inputPeer,
			Limit: req.Limit,
		})
		if err != nil {
			return err
		}
		messages = mapMessages(result)
		return nil
	})
	if err != nil {
		return telegramclient.MessagesPage{}, classifyError(err)
	}

	page := telegramclient.MessagesPage{
		Source:   telegramclient.DataSourceTelegram,
		Stale:    false,
		Messages: messages,
	}
	if len(messages) > 0 {
		page.OldestMessageID = int64(messages[0].TelegramMessageID)
		page.NewestMessageID = int64(messages[len(messages)-1].TelegramMessageID)
		page.HasOlder = len(messages) >= req.Limit
	}
	return page, nil
}

// LoadOlderMessages 加载更早的消息。
func (a *Adapter) LoadOlderMessages(ctx context.Context, req telegramclient.LoadOlderMessagesRequest) (telegramclient.MessagesPage, error) {
	unlock := a.acquireGate(req.AccountID)
	defer unlock()

	inputPeer := buildInputPeerFromInfo(req.PeerID, req.PeerType, req.AccessHash)
	if inputPeer == nil {
		return telegramclient.MessagesPage{}, telegramclient.NewError(telegramclient.ErrorCodePeerInvalid, "无效的会话类型")
	}

	client := mtproto.NewGotdClient(a.sessionDir, a.key, a.flowStore, a.logger)
	if a.dialFunc != nil {
		client.SetDialer(a.dialFunc)
	}

	var messages []telegramclient.Message
	err := client.RunWithSession(ctx, req.APIID, req.APIHash, req.SessionFilePath, func(ctx context.Context, api *tg.Client) error {
		result, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     inputPeer,
			OffsetID: int(req.BeforeMessageID),
			Limit:    req.Limit,
		})
		if err != nil {
			return err
		}
		messages = mapMessages(result)
		return nil
	})
	if err != nil {
		return telegramclient.MessagesPage{}, classifyError(err)
	}

	page := telegramclient.MessagesPage{
		Source:   telegramclient.DataSourceTelegram,
		Stale:    false,
		Messages: messages,
	}
	if len(messages) > 0 {
		page.OldestMessageID = int64(messages[0].TelegramMessageID)
		page.NewestMessageID = int64(messages[len(messages)-1].TelegramMessageID)
		page.HasOlder = len(messages) >= req.Limit
	}
	return page, nil
}

// SendText 发送文本消息。
func (a *Adapter) SendText(ctx context.Context, req telegramclient.SendTextRequest) (telegramclient.SendResult, error) {
	unlock := a.acquireGate(req.AccountID)
	defer unlock()

	inputPeer := buildInputPeerFromInfo(req.PeerID, req.PeerType, req.AccessHash)
	if inputPeer == nil {
		return telegramclient.SendResult{}, telegramclient.NewError(telegramclient.ErrorCodePeerInvalid, "无效的会话类型")
	}

	client := mtproto.NewGotdClient(a.sessionDir, a.key, a.flowStore, a.logger)
	if a.dialFunc != nil {
		client.SetDialer(a.dialFunc)
	}

	var result telegramclient.SendResult
	err := client.RunWithSession(ctx, req.APIID, req.APIHash, req.SessionFilePath, func(ctx context.Context, api *tg.Client) error {
		apiResult, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
			Peer:     inputPeer,
			Message:  req.Text,
			RandomID: req.ClientRandomID,
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

		result = telegramclient.SendResult{
			MessageID: msgID,
			SentAt:    time.Now(),
			Status:    "sent",
			Direction: "out",
			Text:      req.Text,
		}
		return nil
	})
	if err != nil {
		return telegramclient.SendResult{}, classifyError(err)
	}

	return result, nil
}

// buildInputPeerFromInfo 从 peer 信息构造 gotd InputPeerClass。
func buildInputPeerFromInfo(peerID int64, peerType telegramclient.PeerType, accessHash int64) tg.InputPeerClass {
	switch peerType {
	case telegramclient.PeerTypeUser:
		return &tg.InputPeerUser{UserID: peerID, AccessHash: accessHash}
	case telegramclient.PeerTypeChat:
		return &tg.InputPeerChat{ChatID: peerID}
	case telegramclient.PeerTypeChannel:
		return &tg.InputPeerChannel{ChannelID: peerID, AccessHash: accessHash}
	}
	return nil
}

// classifyError 将 gotd 错误转换为中立错误。
func classifyError(err error) error {
	if err == nil {
		return nil
	}

	// 检查是否已经是中立错误
	if _, ok := err.(*telegramclient.Error); ok {
		return err
	}

	// 使用 tgerr.As 提取 Telegram RPC 错误
	if rpcErr, ok := tgerr.As(err); ok {
		return mapRPCError(rpcErr)
	}

	// 检查 mtproto 错误
	if mtprotoErr, ok := err.(*mtproto.MTProtoError); ok {
		return mapMTProtoError(mtprotoErr)
	}

	// 未知错误
	return telegramclient.WrapError(telegramclient.ErrorCodeTelegramError, "Telegram 返回异常", err)
}

// mapRPCError 将 gotd RPC 错误映射为中立错误。
func mapRPCError(rpcErr *tgerr.Error) *telegramclient.Error {
	switch rpcErr.Type {
	case "AUTH_KEY_UNREGISTERED", "AUTH_KEY_INVALID":
		return telegramclient.NewError(telegramclient.ErrorCodeSessionInvalid, "当前账号 Session 已失效，请重新接入")
	case "SESSION_REVOKED", "SESSION_EXPIRED":
		return telegramclient.NewError(telegramclient.ErrorCodeSessionInvalid, "当前账号 Session 已失效，请重新接入")
	case "USER_DEACTIVATED", "USER_DEACTIVATED_BAN":
		return telegramclient.NewError(telegramclient.ErrorCodeAccountDeactivated, "该 Telegram 账号不可用或已被停用")
	case "API_ID_INVALID":
		return telegramclient.NewError(telegramclient.ErrorCodeAPIKeyInvalid, "Telegram API Key 不可用，请检查 API ID / API Hash")
	case "API_HASH_INVALID":
		return telegramclient.NewError(telegramclient.ErrorCodeAPIKeyInvalid, "Telegram API Hash 不可用")
	case "FLOOD_WAIT":
		return telegramclient.NewError(telegramclient.ErrorCodeFloodWait, "Telegram 限制请求过快，请稍后再试")
	case "AUTH_RESTART":
		return telegramclient.NewError(telegramclient.ErrorCodeAuthRestart, "Telegram 要求重新开始认证，请重新接入账号")
	case "TIMEOUT":
		return telegramclient.NewError(telegramclient.ErrorCodeTelegramTimeout, "连接 Telegram 超时，请稍后重试或检查代理")
	case "INTERNAL":
		return telegramclient.NewError(telegramclient.ErrorCodeTelegramError, "Telegram 内部错误，请稍后重试")
	default:
		return telegramclient.NewErrorf(telegramclient.ErrorCodeTelegramError, "Telegram 返回错误 (%s)，请稍后重试", rpcErr.Type)
	}
}

// mapMTProtoError 将 mtproto 错误映射为中立错误。
func mapMTProtoError(mtprotoErr *mtproto.MTProtoError) *telegramclient.Error {
	switch mtprotoErr.Kind {
	case mtproto.ErrProxyConnectFailed:
		return telegramclient.NewError(telegramclient.ErrorCodeProxyConnectFailed, "无法连接代理，请检查 API 网络代理配置")
	case mtproto.ErrProxyAuthFailed:
		return telegramclient.NewError(telegramclient.ErrorCodeProxyAuthFailed, "代理认证失败，请检查代理用户名和密码")
	case mtproto.ErrTelegramTimeout:
		return telegramclient.NewError(telegramclient.ErrorCodeTelegramTimeout, "连接 Telegram 超时，请稍后重试或检查代理")
	case mtproto.ErrSessionInvalid, mtproto.ErrSessionContextLost:
		return telegramclient.NewError(telegramclient.ErrorCodeSessionInvalid, "当前账号 Session 已失效，请重新接入")
	case mtproto.ErrUnauthorized:
		return telegramclient.NewError(telegramclient.ErrorCodeSessionInvalid, "当前账号 Session 已失效，请重新接入")
	case mtproto.ErrCredentialDisabled:
		return telegramclient.NewError(telegramclient.ErrorCodeAPIKeyInvalid, "Telegram API Key 不可用，请检查 API ID / API Hash")
	case mtproto.ErrFloodWait:
		return telegramclient.NewError(telegramclient.ErrorCodeFloodWait, "Telegram 限制请求过快，请稍后再试")
	case mtproto.ErrTelegramError:
		return telegramclient.NewError(telegramclient.ErrorCodeTelegramError, "Telegram 返回异常，请稍后重试或检查日志")
	case mtproto.ErrNetworkError:
		return telegramclient.NewError(telegramclient.ErrorCodeNetworkError, "网络异常，请检查网络连接或代理配置")
	default:
		return telegramclient.NewError(telegramclient.ErrorCodeTelegramError, "Telegram 返回异常，请稍后重试或检查日志")
	}
}

// 确保 Adapter 实现 ClientAdapter。
var _ telegramclient.ClientAdapter = (*Adapter)(nil)
