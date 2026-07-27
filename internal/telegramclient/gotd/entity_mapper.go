package gotd

import (
	"github.com/gotd/td/tg"
	"github.com/user/atria/internal/telegramclient"
)

// mapEntities 将 gotd 的消息实体映射为中立 DTO。
//
// Telegram 的 offset/length 以 UTF-16 码元计，前端 JS 字符串同为 UTF-16，
// 因此数值可直接透传，无需转换。
//
// 未识别的实体类型会被丢弃，前端按纯文本渲染该区间。
func mapEntities(entities []tg.MessageEntityClass) []telegramclient.MessageEntity {
	if len(entities) == 0 {
		return nil
	}

	out := make([]telegramclient.MessageEntity, 0, len(entities))
	for _, e := range entities {
		mapped, ok := mapEntity(e)
		if ok {
			out = append(out, mapped)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mapEntity(e tg.MessageEntityClass) (telegramclient.MessageEntity, bool) {
	switch v := e.(type) {
	case *tg.MessageEntityBold:
		return entity(telegramclient.EntityBold, v.Offset, v.Length), true
	case *tg.MessageEntityItalic:
		return entity(telegramclient.EntityItalic, v.Offset, v.Length), true
	case *tg.MessageEntityUnderline:
		return entity(telegramclient.EntityUnderline, v.Offset, v.Length), true
	case *tg.MessageEntityStrike:
		return entity(telegramclient.EntityStrike, v.Offset, v.Length), true
	case *tg.MessageEntitySpoiler:
		return entity(telegramclient.EntitySpoiler, v.Offset, v.Length), true
	case *tg.MessageEntityCode:
		return entity(telegramclient.EntityCode, v.Offset, v.Length), true
	case *tg.MessageEntityBlockquote:
		return entity(telegramclient.EntityBlockquote, v.Offset, v.Length), true
	case *tg.MessageEntityURL:
		return entity(telegramclient.EntityURL, v.Offset, v.Length), true
	case *tg.MessageEntityMention:
		return entity(telegramclient.EntityMention, v.Offset, v.Length), true
	case *tg.MessageEntityHashtag:
		return entity(telegramclient.EntityHashtag, v.Offset, v.Length), true
	case *tg.MessageEntityEmail:
		return entity(telegramclient.EntityEmail, v.Offset, v.Length), true
	case *tg.MessageEntityPhone:
		return entity(telegramclient.EntityPhone, v.Offset, v.Length), true
	case *tg.MessageEntityBotCommand:
		return entity(telegramclient.EntityBotCommand, v.Offset, v.Length), true

	case *tg.MessageEntityPre:
		ent := entity(telegramclient.EntityPre, v.Offset, v.Length)
		ent.Language = v.Language
		return ent, true

	case *tg.MessageEntityTextURL:
		ent := entity(telegramclient.EntityTextURL, v.Offset, v.Length)
		ent.URL = v.URL
		return ent, true

	case *tg.MessageEntityCustomEmoji:
		// 自定义 emoji 需要额外下载贴纸资源，当前按普通文本渲染，
		// 但保留实体信息以便后续支持。
		return entity(telegramclient.EntityCustomEmoji, v.Offset, v.Length), true

	case *tg.MessageEntityMentionName:
		// 提及具体用户（带 user_id）。当前与普通 mention 同等对待。
		return entity(telegramclient.EntityMention, v.Offset, v.Length), true
	}

	return telegramclient.MessageEntity{}, false
}

func entity(t telegramclient.EntityType, offset, length int) telegramclient.MessageEntity {
	return telegramclient.MessageEntity{
		Type:   t,
		Offset: offset,
		Length: length,
	}
}
