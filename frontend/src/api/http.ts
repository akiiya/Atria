function getCSRFToken(): string {
  const meta = document.querySelector('meta[name="csrf-token"]')
  return meta?.getAttribute('content') || ''
}

function getCookie(name: string): string {
  const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'))
  return match ? decodeURIComponent(match[2]) : ''
}

const DEFAULT_TIMEOUT_MS = 30_000

function fetchWithTimeout(url: string, init: RequestInit, timeoutMs = DEFAULT_TIMEOUT_MS): Promise<Response> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)

  return fetch(url, { ...init, signal: controller.signal }).finally(() => {
    clearTimeout(timer)
  })
}

export async function apiGet<T>(url: string): Promise<T> {
  const res = await fetchWithTimeout(url, {
    headers: {
      'Accept': 'application/json',
      'X-CSRF-Token': getCSRFToken() || getCookie('atria_csrf'),
    },
    credentials: 'same-origin',
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}

export async function apiPost<T>(url: string, body?: Record<string, unknown>): Promise<T> {
  const res = await fetchWithTimeout(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
      'X-CSRF-Token': getCSRFToken() || getCookie('atria_csrf'),
    },
    credentials: 'same-origin',
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}

export async function apiPostForm<T>(url: string, body: FormData): Promise<T> {
  const res = await fetchWithTimeout(url, {
    method: 'POST',
    headers: {
      'Accept': 'application/json',
      'X-CSRF-Token': getCSRFToken() || getCookie('atria_csrf'),
    },
    credentials: 'same-origin',
    body,
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}
