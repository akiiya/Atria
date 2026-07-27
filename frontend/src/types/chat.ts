export type PeerType = 'user' | 'bot' | 'chat' | 'supergroup' | 'channel'

export interface Dialog {
  peer_ref: string
  peer_type: PeerType
  title: string
  username?: string
  avatar_placeholder?: string
  last_message_preview?: string
  last_message_at?: string
  unread_count: number
  is_pinned?: boolean
  is_muted?: boolean
  member_count?: number
  flags?: string
}

export type MessageKind = 'text' | 'photo' | 'document' | 'sticker' | 'video' | 'voice' | 'audio' | 'service' | 'unsupported'

export interface MediaInfo {
  file_name?: string
  mime_type?: string
  size?: number
  emoji?: string
  width?: number
  height?: number
  duration?: number
  /** 内嵌预览图 data URI，随消息一起下发，无需额外请求 */
  thumbnail?: string
}

/**
 * 消息正文格式化实体类型。
 * 与 Telegram 的 MessageEntity 类型一一对应。
 */
export type EntityType =
  | 'bold' | 'italic' | 'underline' | 'strike' | 'spoiler'
  | 'code' | 'pre' | 'blockquote'
  | 'url' | 'text_url' | 'mention' | 'hashtag' | 'email' | 'phone'
  | 'bot_command' | 'custom_emoji'

/**
 * 消息正文格式化区间。
 * offset/length 以 UTF-16 码元计（与 JS 字符串索引一致）。
 */
export interface MessageEntity {
  type: EntityType
  offset: number
  length: number
  url?: string
  language?: string
}

export interface MediaInfoExtended extends MediaInfo {
  download_available?: boolean
  local_status?: string
}

/**
 * 聊天消息类型。
 *
 * id: 后端返回的 telegram_message_id（REST API 中的 id 字段）
 * telegram_message_id: 跨 REST / WebSocket / optimistic 去重的主键
 * local_id: 前端 optimistic message 的临时标识，发送前生成
 * pending: 是否为 optimistic message（尚未收到服务端确认）
 */
export interface ChatMessage {
  id: number
  telegram_message_id?: number
  local_id?: string
  client_pending_id?: string
  pending?: boolean
  peer_ref: string
  direction: 'in' | 'out'
  sender_name: string
  text: string
  sent_at: string
  is_outgoing: boolean
  status: 'sending' | 'sent' | 'failed' | 'unknown'
  message_type: MessageKind
  kind?: MessageKind
  caption?: string
  media?: MediaInfo
  has_media?: boolean
  media_info?: MediaInfoExtended
  /** 正文格式化区间（粗体、代码、链接等），为空表示纯文本 */
  entities?: MessageEntity[]
}

export interface SendResult {
  id: number
  telegram_message_id?: number
  local_id?: string
  sent_at: string
  status: string
  direction: string
  text: string
}

export interface DialogsResponse {
  ok: boolean
  dialogs: Dialog[]
  source?: string  // cache, telegram, mixed
  stale?: boolean   // true 表示数据可能过期
  error?: string
}

export interface MessagesResponse {
  ok: boolean
  messages: ChatMessage[]
  older_messages?: ChatMessage[]
  pages?: Array<{ messages: ChatMessage[] }>
  source?: string
  stale?: boolean
  has_older?: boolean
  oldest_message_id?: number
  newest_message_id?: number
  error?: string
}

export interface SendMessageResponse {
  ok: boolean
  message?: SendResult
  code?: string
  error?: string
}
