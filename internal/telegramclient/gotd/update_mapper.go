package gotd

import (
	"fmt"
	"time"

	"github.com/gotd/td/tg"
	"github.com/user/atria/internal/telegramclient"
)

// mapUpdateNewMessage 将 tg.UpdateNewMessage 映射为中立 Message 和 UpdateEvent。
// 不记录 message body 到日志。
func mapUpdateNewMessage(msg *tg.Message, users []tg.UserClass) (telegramclient.Message, telegramclient.UpdateEvent) {
	neutralMsg := mapMessage(msg)

	// 查找发送者名称
	if !msg.Out && msg.FromID != nil {
		if peerUser, ok := msg.FromID.(*tg.PeerUser); ok {
			for _, u := range users {
				if user, ok := u.(*tg.User); ok && user.ID == peerUser.UserID {
					neutralMsg.SenderName = buildDisplayName(user.FirstName, user.LastName)
					break
				}
			}
		}
	}

	peerRef := mapPeerRef(msg.PeerID)
	event := telegramclient.UpdateEvent{
		EventID:   fmt.Sprintf("msg_%s_%d", peerRef, msg.ID),
		Type:      telegramclient.EventMessageNew,
		PeerRef:   peerRef,
		Payload:   neutralMsg,
		CreatedAt: time.Now(),
	}

	return neutralMsg, event
}

// mapUpdateNewChannelMessage 将频道消息映射为中立 Message 和 UpdateEvent。
func mapUpdateNewChannelMessage(msg *tg.Message, users []tg.UserClass) (telegramclient.Message, telegramclient.UpdateEvent) {
	neutralMsg := mapMessage(msg)
	peerRef := mapPeerRef(msg.PeerID)

	event := telegramclient.UpdateEvent{
		EventID:   fmt.Sprintf("chmsg_%s_%d", peerRef, msg.ID),
		Type:      telegramclient.EventMessageNew,
		PeerRef:   peerRef,
		Payload:   neutralMsg,
		CreatedAt: time.Now(),
	}

	return neutralMsg, event
}

// mapUpdateEditMessage 将编辑消息映射为中立 Message 和 UpdateEvent。
func mapUpdateEditMessage(msg *tg.Message) (telegramclient.Message, telegramclient.UpdateEvent) {
	neutralMsg := mapMessage(msg)
	peerRef := mapPeerRef(msg.PeerID)

	now := time.Now()
	neutralMsg.EditedAt = &now

	event := telegramclient.UpdateEvent{
		EventID:   fmt.Sprintf("edit_%s_%d", peerRef, msg.ID),
		Type:      telegramclient.EventMessageEdited,
		PeerRef:   peerRef,
		Payload:   neutralMsg,
		CreatedAt: now,
	}

	return neutralMsg, event
}

// mapUpdateDeleteMessages 将删除消息映射为 UpdateEvent。
func mapUpdateDeleteMessages(peerRef string, msgIDs []int) telegramclient.UpdateEvent {
	return telegramclient.UpdateEvent{
		EventID:   fmt.Sprintf("del_%s_%d", peerRef, time.Now().UnixNano()),
		Type:      telegramclient.EventMessageDeleted,
		PeerRef:   peerRef,
		Payload:   map[string]any{"telegram_message_ids": msgIDs},
		CreatedAt: time.Now(),
	}
}

// extractUpdates 从 tg.UpdatesClass 中提取单个 update 列表。
func extractUpdates(u tg.UpdatesClass) ([]tg.UpdateClass, []tg.UserClass, []tg.ChatClass) {
	switch updates := u.(type) {
	case *tg.Updates:
		return updates.Updates, updates.Users, updates.Chats
	case *tg.UpdatesCombined:
		return updates.Updates, updates.Users, updates.Chats
	case *tg.UpdateShort:
		return []tg.UpdateClass{updates.Update}, nil, nil
	case *tg.UpdateShortMessage:
		// 转换为 UpdateNewMessage
		msg := &tg.Message{
			ID:      updates.ID,
			PeerID:  &tg.PeerUser{UserID: updates.UserID},
			Message: updates.Message,
			Date:    updates.Date,
			Out:     updates.Out,
		}
		return []tg.UpdateClass{&tg.UpdateNewMessage{Message: msg, Pts: updates.Pts}}, nil, nil
	case *tg.UpdateShortChatMessage:
		msg := &tg.Message{
			ID:      updates.ID,
			PeerID:  &tg.PeerChat{ChatID: updates.ChatID},
			Message: updates.Message,
			Date:    updates.Date,
			Out:     false,
			FromID:  &tg.PeerUser{UserID: updates.FromID},
		}
		return []tg.UpdateClass{&tg.UpdateNewMessage{Message: msg, Pts: updates.Pts}}, nil, nil
	default:
		return nil, nil, nil
	}
}
