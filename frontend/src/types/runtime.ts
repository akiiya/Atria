export interface RuntimeStatusResponse {
  ok: boolean
  account_id?: number
  state?: string
  last_sync_at?: string
  last_event_at?: string
  last_error?: string
  active?: boolean
  code?: string
  message?: string
}

export interface RuntimeActionResponse {
  ok: boolean
  account_id?: number
  state?: string
  code?: string
  message?: string
}

export type RuntimeState = 'stopped' | 'connecting' | 'syncing' | 'live' | 'degraded' | 'offline'
