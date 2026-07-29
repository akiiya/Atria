import { apiGet, apiPost } from './http'
import type { DialogsResponse, MessagesResponse, SendMessageResponse } from '@/types/chat'

export function fetchDialogs(limit = 30, forceRefresh = false): Promise<DialogsResponse> {
  let url = `/api/chats/dialogs?limit=${limit}`
  if (forceRefresh) url += '&force_refresh=true'
  return apiGet<DialogsResponse>(url)
}

export function fetchMessages(peerRef: string, limit = 50, beforeId?: number, forceRefresh = false): Promise<MessagesResponse> {
  let url = `/api/chats/${encodeURIComponent(peerRef)}/messages?limit=${limit}`
  if (beforeId) url += `&before_id=${beforeId}`
  if (forceRefresh) url += '&force_refresh=true'
  return apiGet<MessagesResponse>(url)
}

export function sendMessage(peerRef: string, text: string, localId?: string, replyTo?: number): Promise<SendMessageResponse> {
  const body: Record<string, unknown> = { text }
  if (localId) {
    body.local_id = localId
    body.client_pending_id = localId
  }
  if (replyTo) {
    body.reply_to = replyTo
  }
  return apiPost<SendMessageResponse>(
    `/api/chats/${encodeURIComponent(peerRef)}/messages`,
    body
  )
}

export function markRead(peerRef: string, maxId?: number, reason?: string): Promise<{ ok: boolean }> {
  return apiPost<{ ok: boolean }>(
    `/api/chats/${encodeURIComponent(peerRef)}/read`,
    { max_id: maxId || 0, reason: reason || 'open_chat' }
  )
}
