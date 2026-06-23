package gotd

import (
	"fmt"
	"testing"

	"github.com/gotd/td/tg"
	"github.com/user/atria/internal/telegramclient"
)

func TestGotdUpdateMapper_NewMessageToNeutralEvent(t *testing.T) {
	msg := &tg.Message{
		ID:      123,
		Message: "Hello world",
		Date:    1700000000,
		Out:     false,
		PeerID:  &tg.PeerUser{UserID: 456},
		FromID:  &tg.PeerUser{UserID: 456},
	}

	users := []tg.UserClass{
		&tg.User{ID: 456, FirstName: "Alice", LastName: "Smith"},
	}

	neutralMsg, event := mapUpdateNewMessage(msg, users)

	if neutralMsg.TelegramMessageID != 123 {
		t.Errorf("期望 TelegramMessageID=123，实际 %d", neutralMsg.TelegramMessageID)
	}
	if neutralMsg.Text != "Hello world" {
		t.Errorf("期望 Text='Hello world'，实际 '%s'", neutralMsg.Text)
	}
	if neutralMsg.SenderName != "Alice Smith" {
		t.Errorf("期望 SenderName='Alice Smith'，实际 '%s'", neutralMsg.SenderName)
	}
	if event.Type != telegramclient.EventMessageNew {
		t.Errorf("期望 EventMessageNew，实际 %s", event.Type)
	}
	if event.PeerRef != "u_456" {
		t.Errorf("期望 PeerRef=u_456，实际 %s", event.PeerRef)
	}
	// 验证 message payload 的 PeerRef 与 event 一致（修复前此字段为空）
	if neutralMsg.PeerRef != event.PeerRef {
		t.Errorf("消息 PeerRef 应与 event 一致，期望 %s，实际 %s", event.PeerRef, neutralMsg.PeerRef)
	}
}

func TestGotdUpdateMapper_NewChannelMessageToNeutralEvent(t *testing.T) {
	msg := &tg.Message{
		ID:      789,
		Message: "Channel post",
		Date:    1700000000,
		Out:     false,
		PeerID:  &tg.PeerChannel{ChannelID: 100},
	}

	neutralMsg, event := mapUpdateNewChannelMessage(msg, nil)

	if neutralMsg.TelegramMessageID != 789 {
		t.Errorf("期望 TelegramMessageID=789，实际 %d", neutralMsg.TelegramMessageID)
	}
	if event.Type != telegramclient.EventMessageNew {
		t.Errorf("期望 EventMessageNew，实际 %s", event.Type)
	}
	if event.PeerRef != "ch_100" {
		t.Errorf("期望 PeerRef=ch_100，实际 %s", event.PeerRef)
	}
	if neutralMsg.PeerRef != event.PeerRef {
		t.Errorf("消息 PeerRef 应与 event 一致，期望 %s，实际 %s", event.PeerRef, neutralMsg.PeerRef)
	}
}

func TestGotdUpdateMapper_EditMessageToNeutralEvent(t *testing.T) {
	msg := &tg.Message{
		ID:      456,
		Message: "Edited text",
		Date:    1700000000,
		PeerID:  &tg.PeerUser{UserID: 123},
	}

	neutralMsg, event := mapUpdateEditMessage(msg)

	if neutralMsg.TelegramMessageID != 456 {
		t.Errorf("期望 TelegramMessageID=456，实际 %d", neutralMsg.TelegramMessageID)
	}
	if neutralMsg.EditedAt == nil {
		t.Error("期望 EditedAt 非 nil")
	}
	if event.Type != telegramclient.EventMessageEdited {
		t.Errorf("期望 EventMessageEdited，实际 %s", event.Type)
	}
	if event.PeerRef != "u_123" {
		t.Errorf("期望 PeerRef=u_123，实际 %s", event.PeerRef)
	}
	if neutralMsg.PeerRef != event.PeerRef {
		t.Errorf("消息 PeerRef 应与 event 一致，期望 %s，实际 %s", event.PeerRef, neutralMsg.PeerRef)
	}
}

func TestGotdUpdateMapper_DeleteMessageToNeutralEvent(t *testing.T) {
	event := mapUpdateDeleteMessages("u_123", []int{1, 2, 3})

	if event.Type != telegramclient.EventMessageDeleted {
		t.Errorf("期望 EventMessageDeleted，实际 %s", event.Type)
	}
	if event.PeerRef != "u_123" {
		t.Errorf("期望 PeerRef=u_123，实际 %s", event.PeerRef)
	}

	payload, ok := event.Payload.(map[string]any)
	if !ok {
		t.Fatal("期望 Payload 是 map")
	}
	ids, ok := payload["telegram_message_ids"].([]int)
	if !ok {
		t.Fatal("期望 telegram_message_ids 是 []int")
	}
	if len(ids) != 3 {
		t.Errorf("期望 3 个 ID，实际 %d", len(ids))
	}
}

func TestGotdUpdateMapper_UnsupportedUpdateIgnoredSafely(t *testing.T) {
	// extractUpdates 对未知类型返回 nil
	updates, users, chats := extractUpdates(&tg.UpdatesTooLong{})
	if updates != nil {
		t.Error("未知类型应返回 nil updates")
	}
	if users != nil {
		t.Error("未知类型应返回 nil users")
	}
	if chats != nil {
		t.Error("未知类型应返回 nil chats")
	}
}

func TestGotdUpdateMapper_DoesNotLeakAccessHash(t *testing.T) {
	msg := &tg.Message{
		ID:      100,
		Message: "test",
		Date:    1700000000,
		PeerID:  &tg.PeerUser{UserID: 200},
	}

	_, event := mapUpdateNewMessage(msg, nil)

	// 检查 event 中不包含 access_hash
	eventStr := fmt.Sprintf("%+v", event)
	if contains(eventStr, "access_hash") {
		t.Error("UpdateEvent 不应包含 access_hash")
	}
}

func TestGotdUpdateMapper_ExtractUpdatesFromFull(t *testing.T) {
	u := &tg.Updates{
		Updates: []tg.UpdateClass{
			&tg.UpdateNewMessage{
				Message: &tg.Message{ID: 1, Message: "test", Date: 1700000000, PeerID: &tg.PeerUser{UserID: 10}},
				Pts:     100,
			},
		},
		Users: []tg.UserClass{&tg.User{ID: 10, FirstName: "Test"}},
		Chats: []tg.ChatClass{},
	}

	updates, users, chats := extractUpdates(u)
	if len(updates) != 1 {
		t.Errorf("期望 1 个 update，实际 %d", len(updates))
	}
	if len(users) != 1 {
		t.Errorf("期望 1 个 user，实际 %d", len(users))
	}
	if len(chats) != 0 {
		t.Errorf("期望 0 个 chat，实际 %d", len(chats))
	}
}

func TestGotdUpdateMapper_ExtractUpdatesFromShort(t *testing.T) {
	u := &tg.UpdateShort{
		Update: &tg.UpdateNewMessage{
			Message: &tg.Message{ID: 1, Message: "test", Date: 1700000000, PeerID: &tg.PeerUser{UserID: 10}},
			Pts:     100,
		},
	}

	updates, users, chats := extractUpdates(u)
	if len(updates) != 1 {
		t.Errorf("期望 1 个 update，实际 %d", len(updates))
	}
	if users != nil {
		t.Error("UpdateShort 应返回 nil users")
	}
	if chats != nil {
		t.Error("UpdateShort 应返回 nil chats")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
