import { apiGet, apiPost } from './http'
import type { RuntimeStatusResponse, RuntimeActionResponse } from '@/types/runtime'

export function fetchRuntimeStatus(): Promise<RuntimeStatusResponse> {
  return apiGet<RuntimeStatusResponse>('/api/chats/runtime/status')
}

export function startRuntime(): Promise<RuntimeActionResponse> {
  return apiPost<RuntimeActionResponse>('/api/chats/runtime/start')
}

export function stopRuntime(): Promise<RuntimeActionResponse> {
  return apiPost<RuntimeActionResponse>('/api/chats/runtime/stop')
}
