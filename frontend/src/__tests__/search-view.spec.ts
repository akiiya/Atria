import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import SearchView from '@/features/search/SearchView.vue'
import ErrorBanner from '@/components/ErrorBanner.vue'
import { createRouter, createWebHashHistory } from 'vue-router'

vi.mock('@/api/search', () => ({
  searchMessages: vi.fn(),
}))

import { searchMessages } from '@/api/search'
const mockSearchMessages = vi.mocked(searchMessages)

function createTestRouter() {
  return createRouter({
    history: createWebHashHistory("/"),
    routes: [
      { path: "/", component: { template: "<div />" } },
      { path: "/chats/:peerRef", component: { template: "<div />" } },
    ],
  })
}

function mountSearchView() {
  const router = createTestRouter()
  return mount(SearchView, {
    global: {
      plugins: [router],
      stubs: {
        EmptyState: true,
        ErrorBanner: true,
      },
    },
  })
}

describe('SearchView', () => {
  beforeEach(() => { vi.clearAllMocks() })

  it('renders search input', () => {
    const w = mountSearchView()
    expect(w.find('.search-input').exists()).toBe(true)
    expect(w.find('.search-input').attributes('type')).toBe('text')
  })

  it('renders search button', () => {
    const w = mountSearchView()
    expect(w.find('.search-btn').exists()).toBe(true)
  })

  it('shows empty state initially', () => {
    const w = mountSearchView()
    expect(w.find('.search-results').exists()).toBe(false)
    expect(w.find('.search-body').exists()).toBe(true)
  })

  it('search button is disabled when query is empty', () => {
    const w = mountSearchView()
    expect(w.find('.search-btn').attributes('disabled')).toBeDefined()
  })

  it('enables search button when query is non-empty', async () => {
    const w = mountSearchView()
    await w.find('.search-input').setValue('hello')
    expect(w.find('.search-btn').attributes('disabled')).toBeUndefined()
  })

  it('shows results after search', async () => {
    mockSearchMessages.mockResolvedValueOnce({
      ok: true,
      results: [
        { peer_ref: "u_123", message_id: 1, sender_name: "Alice", text_snippet: "Hello <b>world</b>", sent_at: "2025-06-29T10:00:00Z", is_outgoing: false },
        { peer_ref: "u_456", message_id: 2, sender_name: "Bob", text_snippet: "Test message", sent_at: "2025-06-29T11:00:00Z", is_outgoing: true },
      ],
      total: 2, limit: 20, offset: 0,
    })

    const w = mountSearchView()
    await w.find('.search-input').setValue('hello')
    await w.find('.search-btn').trigger('click')
    await vi.waitFor(() => expect(w.find('.search-results').exists()).toBe(true))
    expect(w.findAll('.search-result-item').length).toBe(2)
    expect(w.find('.result-sender').text()).toBe('Alice')
  })

  it('displays total count after search', async () => {
    mockSearchMessages.mockResolvedValueOnce({ ok: true, results: [{ peer_ref: "u_123", message_id: 1, sender_name: "Alice", text_snippet: "Found it", sent_at: "2025-06-29T10:00:00Z", is_outgoing: false }], total: 42, limit: 20, offset: 0 })
    const w = mountSearchView()
    await w.find('.search-input').setValue('test')
    await w.find('.search-btn').trigger('click')
    await vi.waitFor(() => expect(w.find('.search-total').exists()).toBe(true))
    expect(w.find('.search-total').text()).toContain('42')
  })

  it('shows no-results empty state when search returns empty', async () => {
    mockSearchMessages.mockResolvedValueOnce({ ok: true, results: [], total: 0, limit: 20, offset: 0 })
    const w = mountSearchView()
    await w.find('.search-input').setValue('nonexistent')
    await w.find('.search-btn').trigger('click')
    await vi.waitFor(() => expect(w.find('.search-results').exists()).toBe(false))
  })

  it('shows error banner when search fails', async () => {
    mockSearchMessages.mockResolvedValueOnce({ ok: false, results: [], total: 0, limit: 20, offset: 0 })
    const w = mountSearchView()
    await w.find('.search-input').setValue('test')
    await w.find('.search-btn').trigger('click')
    await vi.waitFor(() => expect(w.findComponent(ErrorBanner).exists()).toBe(true))
  })

  it('shows error banner when search throws', async () => {
    mockSearchMessages.mockRejectedValueOnce(new Error("Network error"))
    const w = mountSearchView()
    await w.find('.search-input').setValue('test')
    await w.find('.search-btn').trigger('click')
    await vi.waitFor(() => expect(w.findComponent(ErrorBanner).exists()).toBe(true))
  })

  it('triggers search on Enter keydown', async () => {
    mockSearchMessages.mockResolvedValueOnce({ ok: true, results: [], total: 0, limit: 20, offset: 0 })
    const w = mountSearchView()
    await w.find('.search-input').setValue('test')
    await w.find('.search-input').trigger('keydown', { key: 'Enter' })
    expect(mockSearchMessages).toHaveBeenCalledWith('test', undefined, 20, 0)
  })

  it('does not trigger search on other keys', async () => {
    const w = mountSearchView()
    await w.find('.search-input').setValue('test')
    await w.find('.search-input').trigger('keydown', { key: 'a' })
    expect(mockSearchMessages).not.toHaveBeenCalled()
  })

  it('highlightSnippet escapes HTML to prevent XSS', async () => {
    const lt = String.fromCharCode(60)
    const gt = String.fromCharCode(62)
    const xssSnippet = lt + "script" + gt + "alert(1)" + lt + "/script" + gt
    mockSearchMessages.mockResolvedValueOnce({ ok: true, results: [{ peer_ref: "u_123", message_id: 1, sender_name: "Alice", text_snippet: xssSnippet, sent_at: "2025-06-29T10:00:00Z", is_outgoing: false }], total: 1, limit: 20, offset: 0 })
    const w = mountSearchView()
    await w.find('.search-input').setValue('xss')
    await w.find('.search-btn').trigger('click')
    await vi.waitFor(() => expect(w.find('.result-snippet').exists()).toBe(true))
    const html = w.find('.result-snippet').html()
    expect(html).not.toContain(lt + "script" + gt)
    expect(html).toContain('&lt;')
  })

  it('highlightSnippet preserves <b> highlight tags', async () => {
    mockSearchMessages.mockResolvedValueOnce({ ok: true, results: [{ peer_ref: "u_123", message_id: 1, sender_name: "Alice", text_snippet: "Hello <b>world</b>", sent_at: "2025-06-29T10:00:00Z", is_outgoing: false }], total: 1, limit: 20, offset: 0 })
    const w = mountSearchView()
    await w.find('.search-input').setValue('hello')
    await w.find('.search-btn').trigger('click')
    await vi.waitFor(() => expect(w.find('.result-snippet').exists()).toBe(true))
    expect(w.find('.result-snippet').html()).toContain('<b>world</b>')
  })

  it('highlightSnippet escapes non-highlight content', async () => {
    mockSearchMessages.mockResolvedValueOnce({ ok: true, results: [{ peer_ref: "u_123", message_id: 1, sender_name: "Alice", text_snippet: "<b>bold</b> & \"quotes\" <b>more</b>", sent_at: "2025-06-29T10:00:00Z", is_outgoing: false }], total: 1, limit: 20, offset: 0 })
    const w = mountSearchView()
    await w.find('.search-input').setValue('test')
    await w.find('.search-btn').trigger('click')
    await vi.waitFor(() => expect(w.find('.result-snippet').exists()).toBe(true))
    const html = w.find('.result-snippet').html()
    expect(html).toContain('<b>bold</b>')
    expect(html).toContain('<b>more</b>')
    expect(html).toContain('&amp;')
  })

  it('calls searchMessages with correct parameters', async () => {
    mockSearchMessages.mockResolvedValueOnce({ ok: true, results: [], total: 0, limit: 20, offset: 0 })
    const w = mountSearchView()
    await w.find('.search-input').setValue('my query')
    await w.find('.search-btn').trigger('click')
    await vi.waitFor(() => expect(mockSearchMessages).toHaveBeenCalledWith('my query', undefined, 20, 0))
  })

  it('displays outgoing indicator for outgoing messages', async () => {
    mockSearchMessages.mockResolvedValueOnce({ ok: true, results: [{ peer_ref: "u_123", message_id: 1, sender_name: "Me", text_snippet: "My message", sent_at: "2025-06-29T10:00:00Z", is_outgoing: true }], total: 1, limit: 20, offset: 0 })
    const w = mountSearchView()
    await w.find('.search-input').setValue('test')
    await w.find('.search-btn').trigger('click')
    await vi.waitFor(() => expect(w.find('.result-outgoing').exists()).toBe(true))
  })

  it('does not display outgoing indicator for incoming messages', async () => {
    mockSearchMessages.mockResolvedValueOnce({ ok: true, results: [{ peer_ref: "u_123", message_id: 1, sender_name: "Alice", text_snippet: "Their message", sent_at: "2025-06-29T10:00:00Z", is_outgoing: false }], total: 1, limit: 20, offset: 0 })
    const w = mountSearchView()
    await w.find('.search-input').setValue('test')
    await w.find('.search-btn').trigger('click')
    await vi.waitFor(() => expect(w.find('.search-result-item').exists()).toBe(true))
    expect(w.find('.result-outgoing').exists()).toBe(false)
  })
})

