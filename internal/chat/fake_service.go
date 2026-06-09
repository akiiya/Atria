package chat

import "time"

// FakeService 是用于测试的假聊天服务实现。
type FakeService struct {
	Dialogs       []Dialog
	Messages      []Message
	SendErr       error
	ListErr       error
	GetErr        error
	SendCallCount int // 记录 SendText 调用次数
}

// ListDialogs 返回预设的会话列表。
func (f *FakeService) ListDialogs(accountID uint, limit int) ([]Dialog, error) {
	if f.ListErr != nil {
		return nil, f.ListErr
	}
	if limit > 0 && limit < len(f.Dialogs) {
		return f.Dialogs[:limit], nil
	}
	return f.Dialogs, nil
}

// GetMessages 返回预设的消息列表。
func (f *FakeService) GetMessages(accountID uint, peerRef string, limit int) ([]Message, error) {
	if f.GetErr != nil {
		return nil, f.GetErr
	}
	if limit > 0 && limit < len(f.Messages) {
		return f.Messages[:limit], nil
	}
	return f.Messages, nil
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
