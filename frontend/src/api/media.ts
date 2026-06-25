import { apiGet, apiPost } from './http'

export interface MediaStatus {
  ok: boolean
  status: string // none / cached / downloading / failed
  file_name?: string
  mime_type?: string
  file_size?: number
  available?: boolean
}

export interface MediaDownloadResult {
  ok: boolean
  status?: string
  file_name?: string
  mime_type?: string
  file_size?: number
  message?: string
}

/**
 * 获取媒体文件的缓存状态
 */
export function getMediaStatus(messageId: number, peerRef: string, accountId: number): Promise<MediaStatus> {
  return apiGet<MediaStatus>(`/api/media/${messageId}/status?peer_ref=${encodeURIComponent(peerRef)}&account_id=${accountId}`)
}

/**
 * 触发媒体文件下载到本地缓存
 */
export function downloadMedia(messageId: number, peerRef: string): Promise<MediaDownloadResult> {
  return apiPost<MediaDownloadResult>(`/api/media/${messageId}/download?peer_ref=${encodeURIComponent(peerRef)}`, {})
}

/**
 * 获取已缓存媒体文件的访问 URL
 */
export function getMediaContentUrl(messageId: number, peerRef: string): string {
  return `/api/media/${messageId}/content?peer_ref=${encodeURIComponent(peerRef)}`
}
