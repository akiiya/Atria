import { describe, it, expect, vi, beforeEach } from 'vitest'
import { handleRealtimeEvent } from '../handler'
import type { RealtimeEvent } from '../ws'
import type { ChatMessage, Dialog } from '@/types/chat'

// Mock QueryClient
function createMockQueryClient() {
  const cache = new Map<string, unknown>()
  return {
    setQueryData: vi.fn((key: unknown[], updater: (old: unknown) => unknown) => {
      const keyStr = JSON.stringify(key)
      const old = cache.get(keyStr)
      cache.set(keyStr, updater(old))
    }),
    invalidateQueries: vi.fn(),
    getQueryData: (key: unknown[]) => cache.get(JSON.stringify(key)),
    _cache: cache,
  }
}

function makeMessage(overrides: Partial<ChatMessage> = {}): ChatMessage {
  return {
    id: 123,
    peer_ref: 'u_456',
    direction: 'out',
    sender_name: 'Test',
    text: 'Hello',
    sent_at: '2026-01-01T12:00:00Z',
    is_outgoing: true,
    status: 'sent',
    message_type: 'text',
    ...overrides,
  }
}

function makeDialog(overrides: Partial<Dialog> = {}): Dialog {
  return {
    peer_ref: 'u_456',
    peer_type: 'user',
    title: 'Alice',
    last_message_preview: 'Hi',
    last_message_at: '2026-01-01T12:00:00Z',
    unread_count: 0,
    ...overrides,
  }
}

describe('handleRealtimeEvent', () => {
  let queryClient: ReturnType<typeof createMockQueryClient>

  beforeEach(() => {
    queryClient = createMockQueryClient()
  })

  describe('message.new', () => {
    it('patches current peer messages', () => {
      // Seed messages cache
      queryClient._cache.set(
        JSON.stringify(['messages', 1, 'u_456']),
        { ok: true, messages: [] }
      )

      const event: RealtimeEvent = {
        type: 'message.new',
        event_id: 'evt_1',
        account_id: 1,
        peer_ref: 'u_456',
        created_at: '2026-01-01T12:00:00Z',
        payload: makeMessage({ id: 123, text: 'New message' }),
      }

      handleRealtimeEvent(event, queryClient as never, 1, 'u_456')

      expect(queryClient.setQueryData).toHaveBeenCalled()
    })

    it('deduplicates by telegram_message_id', () => {
      const existingMsg = makeMessage({ id: 123, text: 'Existing' })
      queryClient._cache.set(
        JSON.stringify(['messages', 1, 'u_456']),
        { ok: true, messages: [existingMsg] }
      )

      const event: RealtimeEvent = {
        type: 'message.new',
        event_id: 'evt_2',
        account_id: 1,
        peer_ref: 'u_456',
        created_at: '2026-01-01T12:01:00Z',
        payload: makeMessage({ id: 123, text: 'Duplicate' }),
      }

      handleRealtimeEvent(event, queryClient as never, 1, 'u_456')

      // Should not add duplicate
      const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
      expect(cached.messages).toHaveLength(1)
      expect(cached.messages[0].text).toBe('Existing')
    })

    it('does not patch different peer messages', () => {
      queryClient._cache.set(
        JSON.stringify(['messages', 1, 'u_789']),
        { ok: true, messages: [] }
      )

      const event: RealtimeEvent = {
        type: 'message.new',
        event_id: 'evt_3',
        account_id: 1,
        peer_ref: 'u_456',
        created_at: '2026-01-01T12:00:00Z',
        payload: makeMessage({ id: 123 }),
      }

      handleRealtimeEvent(event, queryClient as never, 1, 'u_789')

      // u_789 messages should not be modified
      const cached = queryClient.getQueryData(['messages', 1, 'u_789']) as { messages: ChatMessage[] }
      expect(cached.messages).toHaveLength(0)
    })

    it('updates dialog preview', () => {
      queryClient._cache.set(
        JSON.stringify(['dialogs', 1]),
        { ok: true, dialogs: [makeDialog({ peer_ref: 'u_456', unread_count: 0 })] }
      )

      const event: RealtimeEvent = {
        type: 'message.new',
        event_id: 'evt_4',
        account_id: 1,
        peer_ref: 'u_456',
        created_at: '2026-01-01T12:00:00Z',
        payload: makeMessage({ id: 123, text: 'New preview text' }),
      }

      handleRealtimeEvent(event, queryClient as never, 1, 'u_789') // different peer

      const cached = queryClient.getQueryData(['dialogs', 1]) as { dialogs: Dialog[] }
      expect(cached.dialogs[0].last_message_preview).toBe('New preview text')
      // unread_count should increase since it's not the current peer
      expect(cached.dialogs[0].unread_count).toBe(1)
    })

    it('ignores events from different account', () => {
      const event: RealtimeEvent = {
        type: 'message.new',
        event_id: 'evt_5',
        account_id: 999, // different account
        peer_ref: 'u_456',
        created_at: '2026-01-01T12:00:00Z',
        payload: makeMessage({ id: 123 }),
      }

      handleRealtimeEvent(event, queryClient as never, 1, 'u_456')

      expect(queryClient.setQueryData).not.toHaveBeenCalled()
    })
  })

  describe('message.edited', () => {
    it('patches existing message', () => {
      queryClient._cache.set(
        JSON.stringify(['messages', 1, 'u_456']),
        { ok: true, messages: [makeMessage({ id: 123, text: 'Original' })] }
      )

      const event: RealtimeEvent = {
        type: 'message.edited',
        event_id: 'evt_6',
        account_id: 1,
        peer_ref: 'u_456',
        created_at: '2026-01-01T12:02:00Z',
        payload: makeMessage({ id: 123, text: 'Edited' }),
      }

      handleRealtimeEvent(event, queryClient as never, 1, 'u_456')

      const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
      expect(cached.messages[0].text).toBe('Edited')
    })
  })

  describe('message.deleted', () => {
    it('removes message from cache', () => {
      queryClient._cache.set(
        JSON.stringify(['messages', 1, 'u_456']),
        { ok: true, messages: [makeMessage({ id: 123 }), makeMessage({ id: 456 })] }
      )

      const event: RealtimeEvent = {
        type: 'message.deleted',
        event_id: 'evt_7',
        account_id: 1,
        peer_ref: 'u_456',
        created_at: '2026-01-01T12:03:00Z',
        payload: { message_ids: [123] },
      }

      handleRealtimeEvent(event, queryClient as never, 1, 'u_456')

      const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
      expect(cached.messages).toHaveLength(1)
      expect(cached.messages[0].id).toBe(456)
    })
  })

  describe('dialog.upserted', () => {
    it('updates dialogs query', () => {
      queryClient._cache.set(
        JSON.stringify(['dialogs', 1]),
        { ok: true, dialogs: [makeDialog({ peer_ref: 'u_456', title: 'Alice' })] }
      )

      const event: RealtimeEvent = {
        type: 'dialog.upserted',
        event_id: 'evt_8',
        account_id: 1,
        peer_ref: 'u_456',
        created_at: '2026-01-01T12:04:00Z',
        payload: makeDialog({ peer_ref: 'u_456', title: 'Alice Updated', unread_count: 5 }),
      }

      handleRealtimeEvent(event, queryClient as never, 1, 'u_456')

      const cached = queryClient.getQueryData(['dialogs', 1]) as { dialogs: Dialog[] }
      expect(cached.dialogs[0].title).toBe('Alice Updated')
      expect(cached.dialogs[0].unread_count).toBe(5)
    })
  })

  describe('sync/status', () => {
    it('invalidates runtime status query', () => {
      const event: RealtimeEvent = {
        type: 'sync.done',
        event_id: 'evt_9',
        account_id: 1,
        created_at: '2026-01-01T12:05:00Z',
      }

      handleRealtimeEvent(event, queryClient as never, 1, null)

      expect(queryClient.invalidateQueries).toHaveBeenCalledWith({
        queryKey: ['runtime-status', 1],
      })
    })
  })
})
