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

export function sendMessage(peerRef: string, text: string, localId?: string): Promise<SendMessageResponse> {
  return apiPost<SendMessageResponse>(
    `/api/chats/${encodeURIComponent(peerRef)}/messages`,
    localId ? { text, local_id: localId, client_pending_id: localId } : { text }
  )
}
