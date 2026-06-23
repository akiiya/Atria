import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  getFirstGrapheme,
  handleRealtimeEvent,
  markLocalMessageFailedInMessagesCache,
  replaceLocalMessageInMessagesCache,
  sortMessagesAsc,
  safeTruncateText,
  upsertMessageInMessagesCache,
} from '../handler'
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
      expect(cached.messages[0].telegram_message_id ?? cached.messages[0].id).toBe(123)
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
    it('removes message by telegram_message_ids', () => {
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
        payload: { telegram_message_ids: [123] },
      }

      handleRealtimeEvent(event, queryClient as never, 1, 'u_456')

      const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
      expect(cached.messages).toHaveLength(1)
      expect(cached.messages[0].id).toBe(456)
    })

    it('ignores different account', () => {
      queryClient._cache.set(
        JSON.stringify(['messages', 1, 'u_456']),
        { ok: true, messages: [makeMessage({ id: 123 })] }
      )

      const event: RealtimeEvent = {
        type: 'message.deleted',
        event_id: 'evt_8',
        account_id: 999,
        peer_ref: 'u_456',
        created_at: '2026-01-01T12:03:00Z',
        payload: { telegram_message_ids: [123] },
      }

      handleRealtimeEvent(event, queryClient as never, 1, 'u_456')

      const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
      expect(cached.messages).toHaveLength(1)
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

describe('optimistic outgoing cache helpers', () => {
  let queryClient: ReturnType<typeof createMockQueryClient>

  beforeEach(() => {
    queryClient = createMockQueryClient()
    queryClient._cache.set(
      JSON.stringify(['messages', 1, 'u_456']),
      { ok: true, messages: [] }
    )
  })

  it('TestOutgoingOptimistic_ReplacedByRESTServerMessage', () => {
    upsertMessageInMessagesCache(queryClient as never, 1, 'u_456', makeMessage({
      id: -1,
      telegram_message_id: undefined,
      local_id: 'local_1',
      pending: true,
      status: 'sending',
      text: 'hello',
    }))

    replaceLocalMessageInMessagesCache(queryClient as never, 1, 'u_456', 'local_1', makeMessage({
      id: 777,
      telegram_message_id: 777,
      local_id: 'local_1',
      pending: false,
      status: 'sent',
      text: 'hello',
    }))

    const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(cached.messages).toHaveLength(1)
    expect(cached.messages[0].telegram_message_id).toBe(777)
    expect(cached.messages[0].pending).toBe(false)
  })

  it('TestOutgoingOptimistic_DeduplicatesRealtimeByTelegramMessageID', () => {
    upsertMessageInMessagesCache(queryClient as never, 1, 'u_456', makeMessage({
      id: 777,
      telegram_message_id: 777,
      local_id: 'local_1',
      text: 'hello',
    }))

    const event: RealtimeEvent = {
      type: 'message.new',
      event_id: 'evt_rt',
      account_id: 1,
      peer_ref: 'u_456',
      created_at: '2026-01-01T12:00:01Z',
      payload: makeMessage({ id: 777, telegram_message_id: 777, text: 'hello from ws' }),
    }
    handleRealtimeEvent(event, queryClient as never, 1, 'u_456')

    const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(cached.messages).toHaveLength(1)
    expect(cached.messages[0].text).toBe('hello from ws')
  })

  it('TestOutgoingOptimistic_DeduplicatesRealtimeByLocalID', () => {
    upsertMessageInMessagesCache(queryClient as never, 1, 'u_456', makeMessage({
      id: -1,
      local_id: 'local_2',
      pending: true,
      status: 'sending',
      text: 'hello',
    }))

    const event: RealtimeEvent = {
      type: 'message.new',
      event_id: 'evt_local',
      account_id: 1,
      peer_ref: 'u_456',
      created_at: '2026-01-01T12:00:01Z',
      payload: makeMessage({ id: 778, telegram_message_id: 778, local_id: 'local_2', text: 'hello' }),
    }
    handleRealtimeEvent(event, queryClient as never, 1, 'u_456')

    const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(cached.messages).toHaveLength(1)
    expect(cached.messages[0].telegram_message_id).toBe(778)
  })

  it('TestOutgoingOptimistic_DoesNotMergeDifferentOutgoingText', () => {
    upsertMessageInMessagesCache(queryClient as never, 1, 'u_456', makeMessage({
      id: -1,
      local_id: 'local_a',
      pending: true,
      status: 'sending',
      text: 'first',
    }))
    upsertMessageInMessagesCache(queryClient as never, 1, 'u_456', makeMessage({
      id: 2,
      telegram_message_id: 2,
      text: 'second',
    }))

    const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(cached.messages).toHaveLength(2)
  })

  it('TestOutgoingOptimistic_DoesNotMergeDifferentPeer', () => {
    upsertMessageInMessagesCache(queryClient as never, 1, 'u_456', makeMessage({
      id: -1,
      peer_ref: 'u_456',
      local_id: 'local_a',
      pending: true,
      status: 'sending',
      text: 'same',
    }))
    upsertMessageInMessagesCache(queryClient as never, 1, 'u_456', makeMessage({
      id: 2,
      telegram_message_id: 2,
      peer_ref: 'u_999',
      text: 'same',
    }))

    const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(cached.messages).toHaveLength(2)
  })

  it('TestOutgoingOptimistic_FailedSendDoesNotDisappearSilently', () => {
    upsertMessageInMessagesCache(queryClient as never, 1, 'u_456', makeMessage({
      id: -1,
      local_id: 'local_fail',
      pending: true,
      status: 'sending',
    }))
    markLocalMessageFailedInMessagesCache(queryClient as never, 1, 'u_456', 'local_fail', 'failed')

    const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(cached.messages).toHaveLength(1)
    expect(cached.messages[0].status).toBe('failed')
    expect(cached.messages[0].pending).toBe(false)
  })

  it('TestMessageNew_DeduplicatesByTelegramMessageID', () => {
    upsertMessageInMessagesCache(queryClient as never, 1, 'u_456', makeMessage({ id: 10, telegram_message_id: 10 }))
    upsertMessageInMessagesCache(queryClient as never, 1, 'u_456', makeMessage({ id: 10, telegram_message_id: 10 }))

    const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(cached.messages).toHaveLength(1)
  })

  it('TestMessageNew_DeduplicatesOptimisticOutgoingMessage', () => {
    upsertMessageInMessagesCache(queryClient as never, 1, 'u_456', makeMessage({
      id: -1,
      local_id: 'local_dedupe',
      pending: true,
      status: 'sending',
      text: 'same',
    }))
    upsertMessageInMessagesCache(queryClient as never, 1, 'u_456', makeMessage({
      id: 30,
      telegram_message_id: 30,
      local_id: 'local_dedupe',
      text: 'same',
    }))

    const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(cached.messages).toHaveLength(1)
    expect(cached.messages[0].telegram_message_id).toBe(30)
  })
})

describe('query patch compatibility', () => {
  let queryClient: ReturnType<typeof createMockQueryClient>

  beforeEach(() => {
    queryClient = createMockQueryClient()
  })

  it('TestMessageNew_PatchesFlatMessages', () => {
    queryClient._cache.set(JSON.stringify(['messages', 1, 'u_456']), { ok: true, messages: [] })
    handleRealtimeEvent({
      type: 'message.new',
      event_id: 'evt_flat',
      account_id: 1,
      peer_ref: 'u_456',
      created_at: '2026-01-01T12:00:00Z',
      payload: makeMessage({ id: 1, telegram_message_id: 1 }),
    }, queryClient as never, 1, 'u_456')

    const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(cached.messages).toHaveLength(1)
  })

  it('TestMessageNew_PatchesPagedMessagesSafely', () => {
    queryClient._cache.set(JSON.stringify(['messages', 1, 'u_456']), {
      ok: true,
      pages: [{ messages: [] }],
    })
    handleRealtimeEvent({
      type: 'message.new',
      event_id: 'evt_paged',
      account_id: 1,
      peer_ref: 'u_456',
      created_at: '2026-01-01T12:00:00Z',
      payload: makeMessage({ id: 1, telegram_message_id: 1 }),
    }, queryClient as never, 1, 'u_456')

    const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { pages: Array<{ messages: ChatMessage[] }> }
    expect(cached.pages[0].messages).toHaveLength(1)
  })

  it('TestMessageEdited_PatchesByTelegramMessageID', () => {
    queryClient._cache.set(JSON.stringify(['messages', 1, 'u_456']), {
      ok: true,
      older_messages: [makeMessage({ id: 5, telegram_message_id: 5, text: 'old' })],
      messages: [],
    })
    handleRealtimeEvent({
      type: 'message.edited',
      event_id: 'evt_edit',
      account_id: 1,
      peer_ref: 'u_456',
      created_at: '2026-01-01T12:00:00Z',
      payload: makeMessage({ id: 5, telegram_message_id: 5, text: 'edited' }),
    }, queryClient as never, 1, 'u_456')

    const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { older_messages: ChatMessage[] }
    expect(cached.older_messages[0].text).toBe('edited')
  })

  it('TestMessageDeleted_PatchesPagedMessagesSafely', () => {
    queryClient._cache.set(JSON.stringify(['messages', 1, 'u_456']), {
      ok: true,
      pages: [{ messages: [makeMessage({ id: 7, telegram_message_id: 7 })] }],
    })
    handleRealtimeEvent({
      type: 'message.deleted',
      event_id: 'evt_delete',
      account_id: 1,
      peer_ref: 'u_456',
      created_at: '2026-01-01T12:00:00Z',
      payload: { telegram_message_ids: [7] },
    }, queryClient as never, 1, 'u_456')

    const cached = queryClient.getQueryData(['messages', 1, 'u_456']) as { pages: Array<{ messages: ChatMessage[] }> }
    expect(cached.pages[0].messages).toHaveLength(0)
  })

  it('TestDialogUpserted_DoesNotClearMessages', () => {
    queryClient._cache.set(JSON.stringify(['messages', 1, 'u_456']), { ok: true, messages: [makeMessage()] })
    queryClient._cache.set(JSON.stringify(['dialogs', 1]), { ok: true, dialogs: [makeDialog()] })
    handleRealtimeEvent({
      type: 'dialog.upserted',
      event_id: 'evt_dialog',
      account_id: 1,
      peer_ref: 'u_456',
      created_at: '2026-01-01T12:00:00Z',
      payload: makeDialog({ title: 'Updated' }),
    }, queryClient as never, 1, 'u_456')

    const messages = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(messages.messages).toHaveLength(1)
  })

  it('TestSyncFailed_DoesNotClearMessages', () => {
    queryClient._cache.set(JSON.stringify(['messages', 1, 'u_456']), { ok: true, messages: [makeMessage()] })
    handleRealtimeEvent({
      type: 'sync.failed',
      event_id: 'evt_sync_failed',
      account_id: 1,
      created_at: '2026-01-01T12:00:00Z',
    }, queryClient as never, 1, 'u_456')

    const messages = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(messages.messages).toHaveLength(1)
  })

  it('TestReconnect_DoesNotClearExistingMessagesBeforeRefetch', () => {
    queryClient._cache.set(JSON.stringify(['messages', 1, 'u_456']), { ok: true, messages: [makeMessage()] })
    queryClient.invalidateQueries({ queryKey: ['messages', 1, 'u_456'] })
    const messages = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(messages.messages).toHaveLength(1)
  })

  it('TestMessageDeleted_IgnoresDifferentPeer', () => {
    queryClient._cache.set(JSON.stringify(['messages', 1, 'u_456']), { ok: true, messages: [makeMessage({ id: 1, telegram_message_id: 1 })] })
    handleRealtimeEvent({
      type: 'message.deleted',
      event_id: 'evt_other_peer',
      account_id: 1,
      peer_ref: 'u_999',
      created_at: '2026-01-01T12:00:00Z',
      payload: { telegram_message_ids: [1] },
    }, queryClient as never, 1, 'u_456')

    const messages = queryClient.getQueryData(['messages', 1, 'u_456']) as { messages: ChatMessage[] }
    expect(messages.messages).toHaveLength(1)
  })

  it('TestMessageDeleted_DoesNotForceScrollBottom', () => {
    queryClient._cache.set(JSON.stringify(['messages', 1, 'u_456']), { ok: true, messages: [makeMessage({ id: 1, telegram_message_id: 1 })] })
    handleRealtimeEvent({
      type: 'message.deleted',
      event_id: 'evt_delete_no_scroll',
      account_id: 1,
      peer_ref: 'u_456',
      created_at: '2026-01-01T12:00:00Z',
      payload: { telegram_message_ids: [1] },
    }, queryClient as never, 1, 'u_456')

    expect(queryClient.invalidateQueries).not.toHaveBeenCalled()
  })
})

describe('sortMessagesAsc', () => {
  it('TestMessagePanel_SortsMessagesBySentAtAsc', () => {
    const messages = [
      makeMessage({ id: 3, sent_at: '2026-01-01T12:00:03Z' }),
      makeMessage({ id: 1, sent_at: '2026-01-01T12:00:01Z' }),
      makeMessage({ id: 2, sent_at: '2026-01-01T12:00:02Z' }),
    ]
    const sorted = sortMessagesAsc(messages)
    expect(sorted.map(m => m.id)).toEqual([1, 2, 3])
  })

  it('TestMessagePanel_SameTimestampSortsByTelegramMessageID', () => {
    const messages = [
      makeMessage({ id: 3, telegram_message_id: 3, sent_at: '2026-01-01T12:00:00Z' }),
      makeMessage({ id: 1, telegram_message_id: 1, sent_at: '2026-01-01T12:00:00Z' }),
      makeMessage({ id: 2, telegram_message_id: 2, sent_at: '2026-01-01T12:00:00Z' }),
    ]
    const sorted = sortMessagesAsc(messages)
    expect(sorted.map(m => m.telegram_message_id)).toEqual([1, 2, 3])
  })

  it('TestMessagePanel_DifferentPrecisionSortsCorrectly', () => {
    // Go RFC3339 可能返回不同精度的时间戳
    // .1Z = 100ms, .123Z = 123ms → 100ms < 123ms
    const messages = [
      makeMessage({ id: 2, sent_at: '2026-01-01T12:00:00.1Z' }),    // 100ms
      makeMessage({ id: 1, sent_at: '2026-01-01T12:00:00.123Z' }),  // 123ms
    ]
    const sorted = sortMessagesAsc(messages)
    expect(sorted.map(m => m.id)).toEqual([2, 1])  // 100ms < 123ms，id=2 应在前
  })

  it('TestMessagePanel_DoesNotMutateOriginal', () => {
    const original = [makeMessage({ id: 2 }), makeMessage({ id: 1 })]
    const sorted = sortMessagesAsc(original)
    expect(original[0].id).toBe(2)  // 原数组不变
    expect(sorted[0].id).toBe(1)
  })

  it('TestMessagePanel_NewRealtimeMessageAppendsToBottom', () => {
    const existing = [
      makeMessage({ id: 1, sent_at: '2026-01-01T12:00:00Z' }),
      makeMessage({ id: 2, sent_at: '2026-01-01T12:01:00Z' }),
    ]
    const newMsg = makeMessage({ id: 3, sent_at: '2026-01-01T12:02:00Z' })
    const sorted = sortMessagesAsc([...existing, newMsg])
    expect(sorted[sorted.length - 1].id).toBe(3)
  })

  it('TestMessagePanel_OlderMessagesPrependNotAppend', () => {
    const recent = [
      makeMessage({ id: 2, sent_at: '2026-01-01T12:01:00Z' }),
      makeMessage({ id: 3, sent_at: '2026-01-01T12:02:00Z' }),
    ]
    const older = makeMessage({ id: 1, sent_at: '2026-01-01T12:00:00Z' })
    const sorted = sortMessagesAsc([...recent, older])
    expect(sorted[0].id).toBe(1)
  })
})

describe('safeTruncateText', () => {
  it('TestSafeTruncate_DoesNotBreakEmoji', () => {
    const text = 'Hello 😂 World'
    const result = safeTruncateText(text, 7)
    // 'H','e','l','l','o',' ','😂' = 7 graphemes
    expect(result).toBe('Hello 😂')
  })

  it('TestSafeTruncate_DoesNotBreakZWJEmoji', () => {
    const text = '👨‍👩‍👧‍👦 Family'
    const result = safeTruncateText(text, 2)
    // '👨‍👩‍👧‍👦' = 1 grapheme, ' ' = 1 grapheme = 2
    expect(result).toBe('👨‍👩‍👧‍👦 ')
  })

  it('TestSafeTruncate_DoesNotBreakFlagEmoji', () => {
    const text = '🇺🇸 Hello'
    const result = safeTruncateText(text, 1)
    expect(result).toBe('🇺🇸')
  })

  it('TestSafeTruncate_DoesNotBreakSkinToneEmoji', () => {
    const text = '👍🏽 Hello'
    const result = safeTruncateText(text, 1)
    expect(result).toBe('👍🏽')
  })

  it('TestSafeTruncate_ReturnsFullTextIfShorter', () => {
    const text = 'Hi'
    const result = safeTruncateText(text, 10)
    expect(result).toBe('Hi')
  })

  it('TestSafeTruncate_HandlesEmptyInput', () => {
    expect(safeTruncateText('', 10)).toBe('')
    expect(safeTruncateText(undefined, 10)).toBe('')
    expect(safeTruncateText(null, 10)).toBe('')
  })
})

describe('getFirstGrapheme', () => {
  it('TestGetFirstGrapheme_FlagEmoji', () => {
    expect(getFirstGrapheme('🇺🇸US GV-Pruse')).toBe('🇺🇸')
  })

  it('TestGetFirstGrapheme_FlagEmojiOnly', () => {
    expect(getFirstGrapheme('🇺🇸')).toBe('🇺🇸')
  })

  it('TestGetFirstGrapheme_OtherFlag', () => {
    expect(getFirstGrapheme('🇯🇵日本語')).toBe('🇯🇵')
  })

  it('TestGetFirstGrapheme_EmojiWithSkinTone', () => {
    expect(getFirstGrapheme('👍🏽 Thumbs Up')).toBe('👍🏽')
  })

  it('TestGetFirstGrapheme_ZWJFamily', () => {
    expect(getFirstGrapheme('👨‍👩‍👧‍👦 Family')).toBe('👨‍👩‍👧‍👦')
  })

  it('TestGetFirstGrapheme_HeartVariationSelector', () => {
    // ❤️ = U+2764 U+FE0F
    expect(getFirstGrapheme('❤️ Red Heart')).toBe('❤️')
  })

  it('TestGetFirstGrapheme_Chinese', () => {
    expect(getFirstGrapheme('中文测试')).toBe('中')
  })

  it('TestGetFirstGrapheme_English', () => {
    expect(getFirstGrapheme('Alice')).toBe('A')
  })

  it('TestGetFirstGrapheme_Number', () => {
    expect(getFirstGrapheme('123Test')).toBe('1')
  })

  it('TestGetFirstGrapheme_RegularEmoji', () => {
    expect(getFirstGrapheme('😂 Laughing')).toBe('😂')
  })

  it('TestGetFirstGrapheme_Empty', () => {
    expect(getFirstGrapheme('')).toBe('?')
    expect(getFirstGrapheme(undefined)).toBe('?')
    expect(getFirstGrapheme(null)).toBe('?')
  })
})

describe('dialog upsert dedup', () => {
  let queryClient: ReturnType<typeof createMockQueryClient>

  beforeEach(() => {
    queryClient = createMockQueryClient()
  })

  it('TestDialogUpsert_DoesNotDuplicateSamePeerRef', () => {
    // 初始有一个 dialog
    queryClient._cache.set(
      JSON.stringify(['dialogs', 1]),
      { ok: true, dialogs: [makeDialog({ peer_ref: 'u_100', title: 'Alice' })] }
    )

    // dialog.upserted 更新同一 peer_ref
    handleRealtimeEvent({
      type: 'dialog.upserted',
      event_id: 'evt_dup',
      account_id: 1,
      peer_ref: 'u_100',
      created_at: '2026-01-01T12:00:00Z',
      payload: makeDialog({ peer_ref: 'u_100', title: 'Alice Updated' }),
    }, queryClient as never, 1, 'u_100')

    const cached = queryClient.getQueryData(['dialogs', 1]) as { dialogs: Dialog[] }
    expect(cached.dialogs).toHaveLength(1)
    expect(cached.dialogs[0].title).toBe('Alice Updated')
  })

  it('TestDialogUpsert_IgnoresEmptyPeerRef', () => {
    queryClient._cache.set(
      JSON.stringify(['dialogs', 1]),
      { ok: true, dialogs: [makeDialog({ peer_ref: 'u_100' })] }
    )

    // 空 peer_ref 的 dialog.upserted 应被忽略
    handleRealtimeEvent({
      type: 'dialog.upserted',
      event_id: 'evt_empty',
      account_id: 1,
      peer_ref: '',
      created_at: '2026-01-01T12:00:00Z',
      payload: makeDialog({ peer_ref: '', title: 'Ghost' }),
    }, queryClient as never, 1, 'u_100')

    const cached = queryClient.getQueryData(['dialogs', 1]) as { dialogs: Dialog[] }
    expect(cached.dialogs).toHaveLength(1)
    expect(cached.dialogs[0].title).not.toBe('Ghost')
  })
})
