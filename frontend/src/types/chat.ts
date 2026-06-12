export type PeerType = 'user' | 'chat' | 'channel'

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
