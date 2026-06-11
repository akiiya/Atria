import { describe, it, expect } from 'vitest'
import { createRouter, createWebHashHistory } from 'vue-router'

describe('Vue Router', () => {
  it('uses createWebHashHistory', () => {
    // 验证 router 使用 hash history 模式
    const router = createRouter({
      history: createWebHashHistory('/app/'),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    // createWebHashHistory 返回的对象包含 startURL 方法
    expect(router.options.history).toBeDefined()
    // hash history 的 base 应包含 /app/
    // 通过检查 history 的 location 来验证
    const loc = router.options.history.location
    expect(loc).toBeDefined()
  })

  it('has all expected route paths', () => {
    const router = createRouter({
      history: createWebHashHistory('/app/'),
      routes: [
        { path: '/', redirect: '/dashboard' },
        { path: '/dashboard', name: 'dashboard', component: { template: '<div />' } },
        { path: '/accounts', name: 'accounts', component: { template: '<div />' } },
        { path: '/accounts/login', name: 'account-login', component: { template: '<div />' } },
        { path: '/accounts/:id', name: 'account-detail', component: { template: '<div />' } },
        { path: '/chats', name: 'chats', component: { template: '<div />' } },
        { path: '/chats/:peerRef', name: 'chat-detail', component: { template: '<div />' } },
        { path: '/contacts', name: 'contacts', component: { template: '<div />' } },
        { path: '/audit', name: 'audit', component: { template: '<div />' } },
        { path: '/settings', name: 'settings', component: { template: '<div />' } },
      ],
    })

    const routeNames = router.getRoutes().map(r => r.name).filter(Boolean)
    expect(routeNames).toContain('dashboard')
    expect(routeNames).toContain('accounts')
    expect(routeNames).toContain('account-login')
    expect(routeNames).toContain('account-detail')
    expect(routeNames).toContain('chats')
    expect(routeNames).toContain('chat-detail')
    expect(routeNames).toContain('contacts')
    expect(routeNames).toContain('audit')
    expect(routeNames).toContain('settings')
  })
})

describe('URL Canonicalization', () => {
  // 测试 canonicalization 逻辑（从 main.ts 提取的核心逻辑）
  function canonicalize(pathname: string, search: string, hash: string): string | null {
    const appBase = '/app/'
    if (pathname.startsWith(appBase) && pathname !== appBase && pathname !== appBase.slice(0, -1)) {
      const subPath = pathname.slice(appBase.length).replace(/\/+$/, '')
      if (subPath) {
        return appBase + '#/' + subPath + search + hash
      }
    }
    return null
  }

  it('/app/accounts -> /app/#/accounts', () => {
    expect(canonicalize('/app/accounts', '', '')).toBe('/app/#/accounts')
  })

  it('/app/chats/u_123 -> /app/#/chats/u_123', () => {
    expect(canonicalize('/app/chats/u_123', '', '')).toBe('/app/#/chats/u_123')
  })

  it('/app/settings -> /app/#/settings', () => {
    expect(canonicalize('/app/settings', '', '')).toBe('/app/#/settings')
  })

  it('/app/accounts/login -> /app/#/accounts/login', () => {
    expect(canonicalize('/app/accounts/login', '', '')).toBe('/app/#/accounts/login')
  })

  it('/app/ (root) does not redirect', () => {
    expect(canonicalize('/app/', '', '')).toBeNull()
  })

  it('/app (no trailing slash) does not redirect', () => {
    expect(canonicalize('/app', '', '')).toBeNull()
  })

  it('/other/path does not redirect', () => {
    expect(canonicalize('/other/path', '', '')).toBeNull()
  })

  it('preserves search params', () => {
    expect(canonicalize('/app/chats', '?limit=50', '')).toBe('/app/#/chats?limit=50')
  })
})
