import { apiGet, apiPost } from './http'
import type { MeResponse, DashboardStats } from '@/types/account'

export function fetchMe(): Promise<MeResponse> {
  return apiGet<MeResponse>('/api/me')
}

export function fetchDashboardStats(): Promise<DashboardStats> {
  return apiGet<DashboardStats>('/api/dashboard/stats')
}

export function selectAccount(accountId: number): Promise<{ ok: boolean }> {
  const form = new FormData()
  form.append('account_id', String(accountId))
  return apiPost('/api/accounts/select', { account_id: accountId })
}
