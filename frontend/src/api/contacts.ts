import { apiGet } from './http'
import type { ContactsResponse } from '@/types/contacts'

export function fetchContacts(forceRefresh = false): Promise<ContactsResponse> {
  let url = '/api/contacts'
  if (forceRefresh) url += '?force_refresh=true'
  return apiGet<ContactsResponse>(url)
}
