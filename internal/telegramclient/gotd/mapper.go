package gotd

import (
	"fmt"
	"strings"
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
	var accessHash int64

	switch p := d.Peer.(type) {
	case *tg.PeerUser:
		peerID = p.UserID
		dlg.PeerType = telegramclient.PeerTypeUser
		for _, u := range users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				dlg.Title = buildDisplayName(user.FirstName, user.LastName)
				dlg.Username = user.Username
				dlg.AvatarText = getInitial(dlg.Title)
				accessHash = user.AccessHash
				if user.Bot {
					dlg.PeerType = telegramclient.PeerTypeBot
				}
				break
			}
		}
	case *tg.PeerChat:
		peerID = p.ChatID
		dlg.PeerType = telegramclient.PeerTypeChat
		for _, c := range chats {
			if chat, ok := c.(*tg.Chat); ok && chat.ID == p.ChatID {
				dlg.Title = chat.Title
				dlg.AvatarText = getInitial(dlg.Title)
				dlg.MemberCount = int(chat.ParticipantsCount)
				break
			}
		}
	case *tg.PeerChannel:
		peerID = p.ChannelID
		for _, c := range chats {
			if channel, ok := c.(*tg.Channel); ok && channel.ID == p.ChannelID {
				dlg.Title = channel.Title
				dlg.Username = channel.Username
				dlg.AvatarText = getInitial(dlg.Title)
				accessHash = channel.AccessHash
				dlg.MemberCount = int(channel.ParticipantsCount)
				dlg.Flags = extractChannelFlags(channel)
				if channel.Megagroup {
					dlg.PeerType = telegramclient.PeerTypeSupergroup
				} else {
					dlg.PeerType = telegramclient.PeerTypeChannel
				}
				break
			}
		}
	}

	dlg.AccessHash = accessHash
	dlg.PeerID = peerID

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

	// 提取 caption 和媒体信息
	if m.Media != nil {
		// 在 gotd v0.115.0 中，媒体消息的 caption 存储在 Message.Message 字段中
		if m.Message != "" {
			msg.Caption = m.Message
		}
		msg.Media = extractMediaInfo(m.Media)
	}

	return msg
}

// extractMediaInfo 从消息媒体中提取媒体元信息。
func extractMediaInfo(media tg.MessageMediaClass) *telegramclient.Media {
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		if m.Photo != nil {
			if photo, ok := m.Photo.(*tg.Photo); ok {
				return &telegramclient.Media{
					Width:  getPhotoWidth(photo),
					Height: getPhotoHeight(photo),
				}
			}
		}
	case *tg.MessageMediaDocument:
		if m.Document != nil {
			if doc, ok := m.Document.(*tg.Document); ok {
				return extractDocumentMedia(doc)
			}
		}
	case *tg.MessageMediaWebPage:
		if m.Webpage != nil {
			if wp, ok := m.Webpage.(*tg.WebPage); ok {
				if wp.Photo != nil {
					if photo, ok := wp.Photo.(*tg.Photo); ok {
						return &telegramclient.Media{
							Width:  getPhotoWidth(photo),
							Height: getPhotoHeight(photo),
						}
					}
				}
				if wp.Document != nil {
					if doc, ok := wp.Document.(*tg.Document); ok {
						return extractDocumentMedia(doc)
					}
				}
			}
		}
	}
	return nil
}

// extractDocumentMedia 从 Document 中提取媒体元信息。
func extractDocumentMedia(doc *tg.Document) *telegramclient.Media {
	med := &telegramclient.Media{
		FileName: getDocumentFilename(doc),
		MIMEType: doc.MimeType,
		Size:     doc.Size,
	}
	for _, attr := range doc.Attributes {
		switch a := attr.(type) {
		case *tg.DocumentAttributeVideo:
			med.Width = int(a.W)
			med.Height = int(a.H)
			med.Duration = int(a.Duration)
		case *tg.DocumentAttributeAudio:
			med.Duration = int(a.Duration)
		case *tg.DocumentAttributeSticker:
			med.Emoji = a.Alt
		case *tg.DocumentAttributeFilename:
			med.FileName = a.FileName
		}
	}
	return med
}

// getPhotoWidth 获取照片宽度（取最大尺寸）。
func getPhotoWidth(photo *tg.Photo) int {
	if len(photo.Sizes) > 0 {
		best := photo.Sizes[len(photo.Sizes)-1]
		if s, ok := best.(*tg.PhotoSize); ok {
			return s.W
		}
	}
	return 0
}

// getPhotoHeight 获取照片高度（取最大尺寸）。
func getPhotoHeight(photo *tg.Photo) int {
	if len(photo.Sizes) > 0 {
		best := photo.Sizes[len(photo.Sizes)-1]
		if s, ok := best.(*tg.PhotoSize); ok {
			return s.H
		}
	}
	return 0
}

// getDocumentFilename 从文档属性中提取文件名。
func getDocumentFilename(doc *tg.Document) string {
	for _, attr := range doc.Attributes {
		if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
			return fn.FileName
		}
	}
	return ""
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
	// 优先检查媒体类型（媒体消息的 caption 存储在 m.Message 中）
	if m.Media != nil {
		switch m.Media.(type) {
		case *tg.MessageMediaPhoto:
			return telegramclient.MessageKindPhoto
		case *tg.MessageMediaDocument:
			doc := m.Media.(*tg.MessageMediaDocument)
			if doc.Document != nil {
				if d, ok := doc.Document.(*tg.Document); ok {
					return classifyDocument(d)
				}
			}
			return telegramclient.MessageKindDocument
		case *tg.MessageMediaGeo, *tg.MessageMediaGeoLive:
			return "geo"
		case *tg.MessageMediaContact:
			return "contact"
		case *tg.MessageMediaPoll:
			return "poll"
		case *tg.MessageMediaWebPage:
			// 网页预览中可能包含图片或视频
			wp, ok := m.Media.(*tg.MessageMediaWebPage)
			if ok && wp.Webpage != nil {
				if webPage, ok := wp.Webpage.(*tg.WebPage); ok {
					if webPage.Photo != nil {
						return telegramclient.MessageKindPhoto
					}
					if webPage.Document != nil {
						if d, ok := webPage.Document.(*tg.Document); ok {
							return classifyDocument(d)
						}
						return telegramclient.MessageKindDocument
					}
				}
			}
			return "webpage"
		default:
			return telegramclient.MessageKindUnsupported
		}
	}
	// 无媒体：纯文本
	return telegramclient.MessageKindText
}

// classifyDocument 根据文档属性判断具体消息类型。
func classifyDocument(d *tg.Document) telegramclient.MessageKind {
	for _, attr := range d.Attributes {
		switch a := attr.(type) {
		case *tg.DocumentAttributeVideo:
			_ = a
			return telegramclient.MessageKindVideo
		case *tg.DocumentAttributeAudio:
			if a.Voice {
				return telegramclient.MessageKindVoice
			}
			return telegramclient.MessageKindAudio
		case *tg.DocumentAttributeSticker:
			return telegramclient.MessageKindSticker
		case *tg.DocumentAttributeAnimated:
			return "animation"
		}
	}
	// MIME type fallback
	mime := d.MimeType
	switch {
	case strings.HasPrefix(mime, "image/"):
		return telegramclient.MessageKindPhoto
	case strings.HasPrefix(mime, "video/"):
		return telegramclient.MessageKindVideo
	case strings.HasPrefix(mime, "audio/"):
		return telegramclient.MessageKindAudio
	}
	return telegramclient.MessageKindDocument
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

// mapContacts 将 gotd User 列表映射为中立 Contact DTO 列表。
// 只映射非 bot、非 deleted 的用户。
func mapContacts(users []tg.UserClass) []telegramclient.Contact {
	var contacts []telegramclient.Contact
	for _, u := range users {
		user, ok := u.(*tg.User)
		if !ok || user.Deleted || user.Bot {
			continue
		}
		contacts = append(contacts, telegramclient.Contact{
			PeerRef:     fmt.Sprintf("u_%d", user.ID),
			PeerType:    telegramclient.PeerTypeUser,
			DisplayName: buildDisplayName(user.FirstName, user.LastName),
			Username:    user.Username,
			Phone:       maskPhone(user.Phone),
			AvatarText:  getInitial(buildDisplayName(user.FirstName, user.LastName)),
			AccessHash:  user.AccessHash,
			PeerID:      user.ID,
		})
	}
	return contacts
}

// maskPhone 对手机号进行脱敏处理。
// 保留前 3 位和后 2 位，中间用 * 替代。
// 例如：13800138000 → 138******00
func maskPhone(phone string) string {
	if len(phone) <= 5 {
		return phone
	}
	return phone[:3] + strings.Repeat("*", len(phone)-5) + phone[len(phone)-2:]
}

// truncateText 截断文本（rune 安全，不会截断多字节字符或 emoji）。
func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}

// extractChannelFlags 从 channel 提取标志位，返回逗号分隔的字符串。
func extractChannelFlags(channel *tg.Channel) string {
	var flags []string
	if channel.Verified {
		flags = append(flags, "verified")
	}
	if channel.Scam {
		flags = append(flags, "scam")
	}
	if channel.Fake {
		flags = append(flags, "fake")
	}
	if channel.Restricted {
		flags = append(flags, "restricted")
	}
	if channel.Broadcast {
		flags = append(flags, "broadcast")
	}
	if channel.Megagroup {
		flags = append(flags, "megagroup")
	}
	return strings.Join(flags, ",")
}
