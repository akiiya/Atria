package chat

import (
	"context"

	"github.com/user/atria/internal/telegramclient"
)

// FakeAdapter 是用于测试的假 ClientAdapter 实现。
type FakeAdapter struct {
	Dialogs       []telegramclient.Dialog
	Messages      []telegramclient.Message
	SendResult    telegramclient.SendResult
	ListErr       error
	GetErr        error
	SendErr       error
	LoadErr       error
	SendCallCount int
}

// ListDialogs 返回预设的会话列表。
func (f *FakeAdapter) ListDialogs(ctx context.Context, req telegramclient.ListDialogsRequest) (telegramclient.DialogsPage, error) {
	if f.ListErr != nil {
		return telegramclient.DialogsPage{}, f.ListErr
	}
	return telegramclient.DialogsPage{
		Source:  telegramclient.DataSourceCache,
		Stale:   false,
		Dialogs: f.Dialogs,
	}, nil
}

// GetRecentMessages 返回预设的消息列表。
func (f *FakeAdapter) GetRecentMessages(ctx context.Context, req telegramclient.GetRecentMessagesRequest) (telegramclient.MessagesPage, error) {
	if f.GetErr != nil {
		return telegramclient.MessagesPage{}, f.GetErr
	}
	return telegramclient.MessagesPage{
		Source:   telegramclient.DataSourceCache,
		Stale:    false,
		Messages: f.Messages,
	}, nil
}

// LoadOlderMessages 返回预设的消息列表。
func (f *FakeAdapter) LoadOlderMessages(ctx context.Context, req telegramclient.LoadOlderMessagesRequest) (telegramclient.MessagesPage, error) {
	if f.LoadErr != nil {
		return telegramclient.MessagesPage{}, f.LoadErr
	}
	return telegramclient.MessagesPage{
		Source:   telegramclient.DataSourceCache,
		Stale:    false,
		Messages: f.Messages,
	}, nil
}

// SendText 返回预设的发送结果。
func (f *FakeAdapter) SendText(ctx context.Context, req telegramclient.SendTextRequest) (telegramclient.SendResult, error) {
	f.SendCallCount++
	if f.SendErr != nil {
		return telegramclient.SendResult{}, f.SendErr
	}
	return f.SendResult, nil
}

// 确保 FakeAdapter 实现 ClientAdapter。
var _ telegramclient.ClientAdapter = (*FakeAdapter)(nil)
