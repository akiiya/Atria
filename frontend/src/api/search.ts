import { apiGet } from './http'

export interface SearchResult {
  peer_ref: string
  message_id: number
  sender_name: string
  text_snippet: string
  sent_at: string
  is_outgoing: boolean
}

export interface SearchResponse {
  ok: boolean
  results: SearchResult[]
  total: number
  limit: number
  offset: number
}

export function searchMessages(q: string, peerRef?: string, limit = 20, offset = 0): Promise<SearchResponse> {
  let url = `/api/search/messages?q=${encodeURIComponent(q)}&limit=${limit}&offset=${offset}`
  if (peerRef) url += `&peer_ref=${encodeURIComponent(peerRef)}`
  return apiGet<SearchResponse>(url)
}
