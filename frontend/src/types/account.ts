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

export interface DashboardStats {
  ok: boolean
  api_key_count: number
  account_count: number
  session_count: number
  audit_today: number
}
