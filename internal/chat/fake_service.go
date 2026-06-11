package chat

import "time"

// FakeService 是用于测试的假聊天服务实现。
type FakeService struct {
	Dialogs       []Dialog
	Messages      []Message
	SendErr       error
	ListErr       error
	GetErr        error
	LoadErr       error
	HasOlder      bool
	SendCallCount int // 记录 SendText 调用次数
}

// ListDialogs 返回预设的会话列表。
func (f *FakeService) ListDialogs(accountID uint, limit int) (*DialogsResult, error) {
	if f.ListErr != nil {
		return nil, f.ListErr
	}
	dialogs := f.Dialogs
	if limit > 0 && limit < len(dialogs) {
		dialogs = dialogs[:limit]
	}
	return &DialogsResult{Dialogs: dialogs, Source: "cache", Stale: false}, nil
}

// GetMessages 返回预设的消息列表。
func (f *FakeService) GetMessages(accountID uint, peerRef string, limit int) (*MessagesResult, error) {
	if f.GetErr != nil {
		return nil, f.GetErr
	}
	messages := f.Messages
	if limit > 0 && limit < len(messages) {
		messages = messages[:limit]
	}
	result := &MessagesResult{Messages: messages, Source: "cache", Stale: false, HasOlder: f.HasOlder}
	if len(messages) > 0 {
		result.OldestMessageID = messages[0].MessageID
		result.NewestMessageID = messages[len(messages)-1].MessageID
	}
	return result, nil
}

// LoadOlderMessages 返回预设的更早消息列表。
func (f *FakeService) LoadOlderMessages(accountID uint, peerRef string, beforeMessageID int, limit int) (*MessagesResult, error) {
	if f.LoadErr != nil {
		return nil, f.LoadErr
	}
	messages := f.Messages
	if limit > 0 && limit < len(messages) {
		messages = messages[:limit]
	}
	result := &MessagesResult{Messages: messages, Source: "cache", Stale: false, HasOlder: f.HasOlder}
	if len(messages) > 0 {
		result.OldestMessageID = messages[0].MessageID
		result.NewestMessageID = messages[len(messages)-1].MessageID
	}
	return result, nil
}

// SendText 返回预设的发送结果，并记录调用次数。
func (f *FakeService) SendText(accountID uint, peerRef string, text string) (*SendResult, error) {
	f.SendCallCount++
	if f.SendErr != nil {
		return nil, f.SendErr
	}
	return &SendResult{
		MessageID: 999,
		SentAt:    time.Now(),
		Status:    "sent",
		Direction: "out",
		Text:      text,
	}, nil
}

// Ensure FakeService implements Service.
var _ Service = (*FakeService)(nil)
