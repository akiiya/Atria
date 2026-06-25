import { apiGet, apiPost } from './http'

export interface MaintenanceStatus {
  ok: boolean
  account_count: number
  api_key_count: number
  peer_cache_count: number
  message_cache_count: number
  audit_log_count: number
  orphan_peers: number
  orphan_messages: number
  migration_version: number
  recent_maintenance: Array<{
    id: number
    action: string
    message: string
    created_at: string
  }>
}

export interface CleanupResult {
  ok: boolean
  dry_run: boolean
  peer_count?: number
  msg_count?: number
  orphan_peers?: number
  orphan_messages?: number
  message: string
}

export function fetchMaintenanceStatus(): Promise<MaintenanceStatus> {
  return apiGet<MaintenanceStatus>('/api/maintenance/status')
}

export function cleanupChatCache(body: {
  account_id: number
  peer_ref?: string
  dry_run?: boolean
}): Promise<CleanupResult> {
  return apiPost<CleanupResult>('/api/maintenance/cleanup/chat-cache', body)
}

export function cleanupOrphans(dryRun = true): Promise<CleanupResult> {
  return apiPost<CleanupResult>('/api/maintenance/cleanup/orphans', { dry_run: dryRun })
}
