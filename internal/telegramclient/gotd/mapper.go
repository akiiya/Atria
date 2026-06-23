package gotd

import (
	"fmt"
	"time"

	"github.com/gotd/td/tg"
	"github.com/user/atria/internal/telegramclient"
)

// mapDialog 将 gotd Dialog 映射为中立 Dialog DTO。
func mapDialog(dialog tg.DialogClass, messages []tg.MessageClass, users []tg.UserClass, chats []tg.ChatClass) *telegramclient.Dialog {
	d, ok := dialog.(*tg.Dialog)
	if !ok {
		return nil
	}

	peerRef := mapPeerRef(d.Peer)
	if peerRef == "" {
		return nil
	}

	dlg := &telegramclient.Dialog{
		PeerRef:     peerRef,
		UnreadCount: d.UnreadCount,
		IsPinned:    d.Pinned,
	}

	var peerID int64
	var peerType telegramclient.PeerType
	var accessHash int64

	switch p := d.Peer.(type) {
	case *tg.PeerUser:
		peerType = telegramclient.PeerTypeUser
		peerID = p.UserID
		dlg.PeerType = telegramclient.PeerTypeUser
		for _, u := range users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				dlg.Title = buildDisplayName(user.FirstName, user.LastName)
				dlg.Username = user.Username
				dlg.AvatarText = getInitial(dlg.Title)
				accessHash = user.AccessHash
				break
			}
		}
	case *tg.PeerChat:
		peerType = telegramclient.PeerTypeChat
		peerID = p.ChatID
		dlg.PeerType = telegramclient.PeerTypeChat
		for _, c := range chats {
			if chat, ok := c.(*tg.Chat); ok && chat.ID == p.ChatID {
				dlg.Title = chat.Title
				dlg.AvatarText = getInitial(dlg.Title)
				break
			}
		}
	case *tg.PeerChannel:
		peerType = telegramclient.PeerTypeChannel
		peerID = p.ChannelID
		dlg.PeerType = telegramclient.PeerTypeChannel
		for _, c := range chats {
			if channel, ok := c.(*tg.Channel); ok && channel.ID == p.ChannelID {
				dlg.Title = channel.Title
				dlg.Username = channel.Username
				dlg.AvatarText = getInitial(dlg.Title)
				accessHash = channel.AccessHash
				break
			}
		}
	}

	dlg.AccessHash = accessHash
	dlg.PeerID = peerID
	_ = peerType // 用于后续扩展

	// 查找最后一条消息
	for _, msg := range messages {
		if m, ok := msg.(*tg.Message); ok && m.ID == d.TopMessage {
			dlg.LastMessagePreview = truncateText(m.Message, 50)
			dlg.LastMessageAt = time.Unix(int64(m.Date), 0)
			dlg.LastMessageKind = classifyMessageKind(m)
			break
		}
	}

	if dlg.Title == "" {
		dlg.Title = "未知会话"
		dlg.AvatarText = "?"
	}

	return dlg
}

// mapMessages 将 gotd 消息结果映射为中立 DTO 列表。
func mapMessages(result tg.MessagesMessagesClass) []telegramclient.Message {
	var messages []telegramclient.Message

	switch m := result.(type) {
	case *tg.MessagesChannelMessages:
		for _, msg := range m.Messages {
			if m2, ok := msg.(*tg.Message); ok {
				messages = append(messages, mapMessage(m2, mapPeerRef(m2.PeerID)))
			}
		}
	case *tg.MessagesMessages:
		for _, msg := range m.Messages {
			if m2, ok := msg.(*tg.Message); ok {
				messages = append(messages, mapMessage(m2, mapPeerRef(m2.PeerID)))
			}
		}
	case *tg.MessagesMessagesSlice:
		for _, msg := range m.Messages {
			if m2, ok := msg.(*tg.Message); ok {
				messages = append(messages, mapMessage(m2, mapPeerRef(m2.PeerID)))
			}
		}
	}

	return messages
}

// mapMessage 将 gotd Message 映射为中立 Message DTO。
// peerRef 由调用方从 msg.PeerID 通过 mapPeerRef 生成后传入。
func mapMessage(m *tg.Message, peerRef string) telegramclient.Message {
	msg := telegramclient.Message{
		ID:                fmt.Sprintf("%d", m.ID),
		TelegramMessageID: m.ID,
		PeerRef:           peerRef,
		SentAt:            time.Unix(int64(m.Date), 0),
		IsOutgoing:        m.Out,
		Status:            telegramclient.MessageStatusSent,
	}

	if m.Out {
		msg.Direction = telegramclient.MessageDirectionOut
	} else {
		msg.Direction = telegramclient.MessageDirectionIn
	}

	msg.Kind = classifyMessageKind(m)

	if m.Message != "" {
		msg.Text = m.Message
	}

	return msg
}

// mapPeerRef 将 gotd Peer 映射为 peer_ref 字符串。
func mapPeerRef(peer tg.PeerClass) string {
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

// classifyMessageKind 根据 gotd Message 内容判断消息类型。
func classifyMessageKind(m *tg.Message) telegramclient.MessageKind {
	if m.Message != "" {
		return telegramclient.MessageKindText
	}
	// TODO: 未来根据 media 类型细化 photo/document/sticker/video/voice/audio
	return telegramclient.MessageKindUnsupported
}

// buildDisplayName 构建显示名。
func buildDisplayName(firstName, lastName string) string {
	name := ""
	if firstName != "" {
		name = firstName
	}
	if lastName != "" {
		if name != "" {
			name += " "
		}
		name += lastName
	}
	if name == "" {
		return "未知用户"
	}
	return name
}

// getInitial 获取名称首字符（grapheme 安全）。
//
// 正确处理国旗 emoji、ZWJ 序列、variation selector 等。
func getInitial(name string) string {
	if name == "" {
		return "?"
	}
	r := []rune(name)
	if len(r) == 0 {
		return "?"
	}

	// Regional Indicator Pair（国旗 emoji）
	if isRegionalIndicator(r[0]) && len(r) > 1 && isRegionalIndicator(r[1]) {
		return string(r[:2])
	}

	// emoji + variation selector / ZWJ 序列 / skin tone
	end := 1
	for end < len(r) {
		if r[end] == 0xFE0E || r[end] == 0xFE0F { // Variation Selector
			end++
			continue
		}
		if r[end] == 0x200D && end+1 < len(r) { // ZWJ
			end += 2
			continue
		}
		if r[end] >= 0x1F3FB && r[end] <= 0x1F3FF { // Skin Tone
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

// truncateText 截断文本（rune 安全，不会截断多字节字符或 emoji）。
func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}
