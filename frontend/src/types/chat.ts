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

export interface ChatMessage {
  id: number
  peer_ref: string
  direction: 'in' | 'out'
  sender_name: string
  text: string
  sent_at: string
  is_outgoing: boolean
  status: 'sent' | 'failed' | 'unknown'
  message_type: MessageKind
  caption?: string
  media?: MediaInfo
}

export interface SendResult {
  id: number
  sent_at: string
  status: string
  direction: string
  text: string
}

export interface DialogsResponse {
  ok: boolean
  dialogs: Dialog[]
  error?: string
}

export interface MessagesResponse {
  ok: boolean
  messages: ChatMessage[]
  error?: string
}

export interface SendMessageResponse {
  ok: boolean
  message?: SendResult
  code?: string
  error?: string
}
