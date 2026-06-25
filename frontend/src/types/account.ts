export interface AdminInfo {
  username: string
}

export interface AccountInfo {
  id: number
  display_name: string
  username: string
  avatar_text: string
}

export interface MeResponse {
  ok: boolean
  admin: AdminInfo
  current_account: AccountInfo | null
  accounts: AccountInfo[]
}

export interface DashboardAuditLog {
  id: number
  action: string
  risk_level: string
  created_at: string
}

export interface DashboardStats {
  ok: boolean
  api_key_count: number
  account_count: number
  session_count: number
  audit_today: number
  runtime_live?: number
  runtime_stopped?: number
  recent_errors?: number
  recent_audit?: number
  recent_logs?: DashboardAuditLog[]
  version?: string
  db_driver?: string
  data_dir?: string
  listen_addr?: string
}
