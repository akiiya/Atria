import { apiGet, apiPost } from './http'
import type { DialogsResponse, MessagesResponse, SendMessageResponse } from '@/types/chat'

export function fetchDialogs(limit = 30): Promise<DialogsResponse> {
  return apiGet<DialogsResponse>(`/api/chats/dialogs?limit=${limit}`)
}

export function fetchMessages(peerRef: string, limit = 50, beforeId?: number): Promise<MessagesResponse> {
  let url = `/api/chats/${encodeURIComponent(peerRef)}/messages?limit=${limit}`
  if (beforeId) url += `&before_id=${beforeId}`
  return apiGet<MessagesResponse>(url)
}

export function sendMessage(peerRef: string, text: string, localId?: string): Promise<SendMessageResponse> {
  return apiPost<SendMessageResponse>(
    `/api/chats/${encodeURIComponent(peerRef)}/messages`,
    localId ? { text, local_id: localId, client_pending_id: localId } : { text }
  )
}
