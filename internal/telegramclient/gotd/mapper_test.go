package gotd

import (
	"testing"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/user/atria/internal/telegramclient"
)

func TestMapMessage_TextMessage(t *testing.T) {
	m := &tg.Message{
		ID:      123,
		Date:    1700000000,
		Out:     true,
		Message: "Hello world",
	}

	result := mapMessage(m, "u_1")

	if result.TelegramMessageID != 123 {
		t.Errorf("期望 ID 123，实际 %d", result.TelegramMessageID)
	}
	if result.Text != "Hello world" {
		t.Errorf("期望文本 Hello world，实际 %s", result.Text)
	}
	if result.Kind != telegramclient.MessageKindText {
		t.Errorf("期望 kind text，实际 %s", result.Kind)
	}
	if !result.IsOutgoing {
		t.Error("期望 IsOutgoing=true")
	}
	if result.Direction != telegramclient.MessageDirectionOut {
		t.Errorf("期望方向 out，实际 %s", result.Direction)
	}
	if result.PeerRef != "u_1" {
		t.Errorf("期望 PeerRef u_1，实际 %s", result.PeerRef)
	}
}

func TestMapMessage_IncomingMessage(t *testing.T) {
	m := &tg.Message{
		ID:      456,
		Date:    1700000000,
		Out:     false,
		Message: "Hi there",
	}

	result := mapMessage(m, "u_1")

	if result.IsOutgoing {
		t.Error("期望 IsOutgoing=false")
	}
	if result.Direction != telegramclient.MessageDirectionIn {
		t.Errorf("期望方向 in，实际 %s", result.Direction)
	}
}

func TestMapMessage_EmptyMessage(t *testing.T) {
	m := &tg.Message{
		ID:      789,
		Date:    1700000000,
		Message: "", // 空文本，无 media
	}

	result := mapMessage(m, "u_1")

	// 空消息（无文本、无媒体）在新分类逻辑中视为 text
	if result.Kind != telegramclient.MessageKindText {
		t.Errorf("期望 kind text，实际 %s", result.Kind)
	}
}

func TestMapPeerRef_User(t *testing.T) {
	peer := &tg.PeerUser{UserID: 12345}
	ref := mapPeerRef(peer)
	if ref != "u_12345" {
		t.Errorf("期望 u_12345，实际 %s", ref)
	}
}

func TestMapPeerRef_Chat(t *testing.T) {
	peer := &tg.PeerChat{ChatID: 67890}
	ref := mapPeerRef(peer)
	if ref != "c_67890" {
		t.Errorf("期望 c_67890，实际 %s", ref)
	}
}

func TestMapPeerRef_Channel(t *testing.T) {
	peer := &tg.PeerChannel{ChannelID: 11111}
	ref := mapPeerRef(peer)
	if ref != "ch_11111" {
		t.Errorf("期望 ch_11111，实际 %s", ref)
	}
}

func TestMapDialog_UserDialog(t *testing.T) {
	dialog := &tg.Dialog{
		Peer:        &tg.PeerUser{UserID: 123},
		TopMessage:  456,
		UnreadCount: 5,
		Pinned:      true,
	}

	messages := []tg.MessageClass{
		&tg.Message{ID: 456, Date: 1700000000, Message: "Last message"},
	}
	users := []tg.UserClass{
		&tg.User{ID: 123, FirstName: "Alice", LastName: "Smith", Username: "alice", AccessHash: 99999},
	}
	chats := []tg.ChatClass{}

	result := mapDialog(dialog, messages, users, chats)

	if result == nil {
		t.Fatal("期望非 nil 结果")
	}
	if result.PeerRef != "u_123" {
		t.Errorf("期望 peer_ref u_123，实际 %s", result.PeerRef)
	}
	if result.PeerType != telegramclient.PeerTypeUser {
		t.Errorf("期望 peer_type user，实际 %s", result.PeerType)
	}
	if result.Title != "Alice Smith" {
		t.Errorf("期望标题 Alice Smith，实际 %s", result.Title)
	}
	if result.Username != "alice" {
		t.Errorf("期望 username alice，实际 %s", result.Username)
	}
	if result.UnreadCount != 5 {
		t.Errorf("期望未读数 5，实际 %d", result.UnreadCount)
	}
	if !result.IsPinned {
		t.Error("期望 IsPinned=true")
	}
	if result.LastMessagePreview != "Last message" {
		t.Errorf("期望最后消息预览 'Last message'，实际 '%s'", result.LastMessagePreview)
	}
	if result.AccessHash == 0 {
		t.Error("期望 AccessHash 非零")
	}
	if result.PeerID != 123 {
		t.Errorf("期望 PeerID 123，实际 %d", result.PeerID)
	}
}

func TestMapDialog_ChatDialog(t *testing.T) {
	dialog := &tg.Dialog{
		Peer:       &tg.PeerChat{ChatID: 789},
		TopMessage: 100,
	}

	messages := []tg.MessageClass{
		&tg.Message{ID: 100, Date: 1700000000, Message: "Group msg"},
	}
	users := []tg.UserClass{}
	chats := []tg.ChatClass{
		&tg.Chat{ID: 789, Title: "Test Group"},
	}

	result := mapDialog(dialog, messages, users, chats)

	if result == nil {
		t.Fatal("期望非 nil 结果")
	}
	if result.PeerRef != "c_789" {
		t.Errorf("期望 peer_ref c_789，实际 %s", result.PeerRef)
	}
	if result.PeerType != telegramclient.PeerTypeChat {
		t.Errorf("期望 peer_type chat，实际 %s", result.PeerType)
	}
	if result.Title != "Test Group" {
		t.Errorf("期望标题 Test Group，实际 %s", result.Title)
	}
}

func TestMapDialog_ChannelDialog(t *testing.T) {
	dialog := &tg.Dialog{
		Peer:       &tg.PeerChannel{ChannelID: 456},
		TopMessage: 200,
	}

	messages := []tg.MessageClass{
		&tg.Message{ID: 200, Date: 1700000000, Message: "Channel post"},
	}
	users := []tg.UserClass{}
	chats := []tg.ChatClass{
		&tg.Channel{ID: 456, Title: "News Channel", Username: "news"},
	}

	result := mapDialog(dialog, messages, users, chats)

	if result == nil {
		t.Fatal("期望非 nil 结果")
	}
	if result.PeerRef != "ch_456" {
		t.Errorf("期望 peer_ref ch_456，实际 %s", result.PeerRef)
	}
	if result.PeerType != telegramclient.PeerTypeChannel {
		t.Errorf("期望 peer_type channel，实际 %s", result.PeerType)
	}
	if result.Title != "News Channel" {
		t.Errorf("期望标题 News Channel，实际 %s", result.Title)
	}
	if result.Username != "news" {
		t.Errorf("期望 username news，实际 %s", result.Username)
	}
}

func TestMapMessages_MessagesSlice(t *testing.T) {
	result := &tg.MessagesMessagesSlice{
		Messages: []tg.MessageClass{
			&tg.Message{ID: 1, Date: 1700000000, Message: "msg1"},
			&tg.Message{ID: 2, Date: 1700000001, Message: "msg2"},
		},
	}

	messages := mapMessages(result)
	if len(messages) != 2 {
		t.Fatalf("期望 2 条消息，实际 %d", len(messages))
	}
	if messages[0].Text != "msg1" {
		t.Errorf("期望第一条消息文本 msg1，实际 %s", messages[0].Text)
	}
}

func TestMapRPCError_AuthKeyInvalid(t *testing.T) {
	err := mapRPCError(&tgerr.Error{Type: "AUTH_KEY_INVALID", Code: 401})
	if err.Code != telegramclient.ErrorCodeSessionInvalid {
		t.Errorf("期望 session_invalid，实际 %s", err.Code)
	}
}

func TestMapRPCError_APIIdInvalid(t *testing.T) {
	err := mapRPCError(&tgerr.Error{Type: "API_ID_INVALID", Code: 400})
	if err.Code != telegramclient.ErrorCodeAPIKeyInvalid {
		t.Errorf("期望 api_key_invalid，实际 %s", err.Code)
	}
}

func TestMapRPCError_FloodWait(t *testing.T) {
	err := mapRPCError(&tgerr.Error{Type: "FLOOD_WAIT", Code: 420})
	if err.Code != telegramclient.ErrorCodeFloodWait {
		t.Errorf("期望 flood_wait，实际 %s", err.Code)
	}
}

func TestMapRPCError_UnknownTGError(t *testing.T) {
	err := mapRPCError(&tgerr.Error{Type: "UNKNOWN_ERROR", Code: 400})
	if err.Code != telegramclient.ErrorCodeTelegramError {
		t.Errorf("期望 telegram_error，实际 %s", err.Code)
	}
}

func TestClassifyMessageKind_Text(t *testing.T) {
	m := &tg.Message{Message: "hello"}
	if classifyMessageKind(m) != telegramclient.MessageKindText {
		t.Error("期望 text")
	}
}

func TestClassifyMessageKind_EmptyMessage(t *testing.T) {
	m := &tg.Message{Message: ""}
	// 空消息（无文本、无媒体）在新分类逻辑中视为 text
	if classifyMessageKind(m) != telegramclient.MessageKindText {
		t.Error("期望 text")
	}
}

func TestBuildDisplayName(t *testing.T) {
	if buildDisplayName("Alice", "Smith") != "Alice Smith" {
		t.Error("期望 Alice Smith")
	}
	if buildDisplayName("Alice", "") != "Alice" {
		t.Error("期望 Alice")
	}
	if buildDisplayName("", "Smith") != "Smith" {
		t.Error("期望 Smith")
	}
	if buildDisplayName("", "") != "未知用户" {
		t.Error("期望 未知用户")
	}
}

func TestGetInitial(t *testing.T) {
	if getInitial("Alice") != "A" {
		t.Error("期望 A")
	}
	if getInitial("") != "?" {
		t.Error("期望 ?")
	}
}

func TestGetInitial_FlagEmoji(t *testing.T) {
	// 🇺🇸 是两个 regional indicator (U+1F1FA U+1F1F8)
	result := getInitial("🇺🇸US GV-Pruse")
	if result != "🇺🇸" {
		t.Errorf("期望 🇺🇸，实际 %q (len=%d)", result, len([]rune(result)))
	}
}

func TestGetInitial_FlagEmojiNoText(t *testing.T) {
	result := getInitial("🇺🇸")
	if result != "🇺🇸" {
		t.Errorf("期望 🇺🇸，实际 %q", result)
	}
}

func TestGetInitial_OtherFlag(t *testing.T) {
	result := getInitial("🇯🇵日本語")
	if result != "🇯🇵" {
		t.Errorf("期望 🇯🇵，实际 %q", result)
	}
}

func TestGetInitial_EmojiWithVariationSelector(t *testing.T) {
	// ❤️ = U+2764 U+FE0F
	result := getInitial("❤️ Red Heart")
	if result != "❤️" {
		t.Errorf("期望 ❤️，实际 %q (runes=%v)", result, []rune(result))
	}
}

func TestGetInitial_ZWJFamily(t *testing.T) {
	// 👨‍👩‍👧‍👦 = U+1F468 U+200D U+1F469 U+200D U+1F466 U+200D U+1F466
	result := getInitial("👨‍👩‍👧‍👦 Family")
	if result != "👨‍👩‍👧‍👦" {
		t.Errorf("期望 👨‍👩‍👧‍👦，实际 %q (runes=%v)", result, []rune(result))
	}
}

func TestGetInitial_EmojiWithSkinTone(t *testing.T) {
	// 👍🏽 = U+1F44D U+1F3FD
	result := getInitial("👍🏽 Thumbs Up")
	if result != "👍🏽" {
		t.Errorf("期望 👍🏽，实际 %q (runes=%v)", result, []rune(result))
	}
}

func TestGetInitial_Chinese(t *testing.T) {
	result := getInitial("中文测试")
	if result != "中" {
		t.Errorf("期望 中，实际 %q", result)
	}
}

func TestGetInitial_Number(t *testing.T) {
	result := getInitial("123Test")
	if result != "1" {
		t.Errorf("期望 1，实际 %q", result)
	}
}

func TestGetInitial_RegularEmoji(t *testing.T) {
	result := getInitial("😂 Laughing")
	if result != "😂" {
		t.Errorf("期望 😂，实际 %q", result)
	}
}

func TestTruncateText(t *testing.T) {
	if truncateText("short", 10) != "short" {
		t.Error("短文本不应截断")
	}
	if truncateText("longtext", 4) != "long..." {
		t.Error("长文本应截断")
	}
}

func TestTruncateText_CJK(t *testing.T) {
	// CJK 字符每个占 3 字节，旧实现按字节截断会乱码
	result := truncateText("你好世界测试", 4)
	if result != "你好世界..." {
		t.Errorf("期望 '你好世界...'，实际 '%s'", result)
	}
}

func TestTruncateText_Emoji(t *testing.T) {
	// emoji 可能占多个字节，旧实现可能截断 surrogate pair
	result := truncateText("😂🥹👍❤️", 2)
	if result != "😂🥹..." {
		t.Errorf("期望 '😂🥹...'，实际 '%s'", result)
	}
}

func TestTruncateText_Mixed(t *testing.T) {
	result := truncateText("Hello你好😂世界", 5)
	if result != "Hello..." {
		t.Errorf("期望 'Hello...'，实际 '%s'", result)
	}
}
