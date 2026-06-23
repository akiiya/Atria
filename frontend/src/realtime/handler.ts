import type { QueryClient } from '@tanstack/vue-query'
import type { RealtimeEvent } from './ws'
import type { ChatMessage, Dialog, MessageKind } from '@/types/chat'
import { useChatStore } from '@/stores/chat'

type MessagesCache = {
  ok?: boolean
  messages?: ChatMessage[]
  older_messages?: ChatMessage[]
  pages?: Array<{ messages?: ChatMessage[]; [key: string]: unknown }>
  [key: string]: unknown
}

type MessagePatchMode = 'upsert' | 'replace-local' | 'mark-failed'

/**
 * safeTruncateText 安全截断文本，不破坏 emoji（surrogate pair / ZWJ / 组合序列）。
 *
 * 优先使用 Intl.Segmenter（按 grapheme cluster 分段），
 * 不支持时 fallback 到 Array.from（按 code point 分段）。
 */
export function safeTruncateText(text: string | undefined | null, maxGraphemes: number): string {
  if (!text) return ''
  // 使用 CSS text-overflow: ellipsis 做视觉截断更安全，
  // 但 preview 文本需要在 JS 层截断以限制数据大小。
  // 优先使用 Intl.Segmenter
  const IntlWithSegmenter = Intl as unknown as { Segmenter?: new (locale: string, opts: { granularity: string }) => { segment: (text: string) => IterableIterator<{ segment: string }> } }
  if (IntlWithSegmenter.Segmenter) {
    const segmenter = new IntlWithSegmenter.Segmenter('zh', { granularity: 'grapheme' })
    const segments = Array.from(segmenter.segment(text))
    if (segments.length <= maxGraphemes) return text
    return segments.slice(0, maxGraphemes).map((s: { segment: string }) => s.segment).join('')
  }
  // fallback: Array.from 按 code point 分段（不拆 surrogate pair）
  const chars = Array.from(text)
  if (chars.length <= maxGraphemes) return text
  return chars.slice(0, maxGraphemes).join('')
}

export function handleRealtimeEvent(
  event: RealtimeEvent,
  queryClient: QueryClient,
  currentAccountId: number | null,
  currentPeerRef: string | null
): void {
  if (!currentAccountId || event.account_id !== currentAccountId) return

  switch (event.type) {
    case 'message.new':
      handleMessageNew(event, queryClient, currentAccountId, currentPeerRef)
      break
    case 'message.edited':
      handleMessageEdited(event, queryClient, currentAccountId, currentPeerRef)
      break
    case 'message.deleted':
      handleMessageDeleted(event, queryClient, currentAccountId, currentPeerRef)
      break
    case 'dialog.upserted':
      handleDialogUpserted(event, queryClient, currentAccountId)
      break
    case 'sync.started':
    case 'sync.done':
    case 'sync.failed':
    case 'account.connected':
    case 'account.disconnected':
      queryClient.invalidateQueries({ queryKey: ['runtime-status', currentAccountId] })
      break
  }
}

export function upsertMessageInMessagesCache(
  queryClient: QueryClient,
  accountId: number,
  peerRef: string,
  message: ChatMessage
): void {
  patchMessagesQuery(queryClient, accountId, peerRef, (old) =>
    patchMessageInCache(old, normalizeMessage(message, peerRef), 'upsert')
  )
}

export function replaceLocalMessageInMessagesCache(
  queryClient: QueryClient,
  accountId: number,
  peerRef: string,
  localId: string,
  message: ChatMessage
): void {
  patchMessagesQuery(queryClient, accountId, peerRef, (old) =>
    patchMessageInCache(old, normalizeMessage({ ...message, local_id: localId }, peerRef), 'replace-local', localId)
  )
}

export function markLocalMessageFailedInMessagesCache(
  queryClient: QueryClient,
  accountId: number,
  peerRef: string,
  localId: string,
  errorText?: string
): void {
  patchMessagesQuery(queryClient, accountId, peerRef, (old) =>
    patchMessageInCache(old, undefined, 'mark-failed', localId, errorText)
  )
}

function handleMessageNew(
  event: RealtimeEvent,
  queryClient: QueryClient,
  accountId: number,
  currentPeerRef: string | null
): void {
  const msg = normalizeMessage(event.payload as Partial<ChatMessage> | undefined, event.peer_ref)
  if (!msg) return

  const peerRef = event.peer_ref
  if (!peerRef) return

  // 1. 更新 dialogs cache（preview、unread、排序）
  // 如果 dialog 不存在，插入新 dialog
  queryClient.setQueryData(['dialogs', accountId], (old: unknown) => {
    const data = old as { ok: boolean; dialogs: Dialog[] } | undefined
    if (!data?.ok || !data.dialogs) return old

    const idx = data.dialogs.findIndex((d) => d.peer_ref === peerRef)
    if (idx >= 0) {
      // 更新已有 dialog
      const dialogs = [...data.dialogs]
      dialogs[idx] = {
        ...dialogs[idx],
        last_message_preview: safeTruncateText(msg.text, 50),
        last_message_at: msg.sent_at,
        unread_count: currentPeerRef === peerRef ? 0 : (dialogs[idx].unread_count || 0) + 1,
      }
      // 排序：移动到 pinned 之后的顶部
      if (idx > 0 && !dialogs[idx].is_pinned) {
        const [updated] = dialogs.splice(idx, 1)
        let insertIdx = 0
        for (let i = 0; i < dialogs.length; i++) {
          if (dialogs[i].is_pinned) insertIdx = i + 1
          else break
        }
        dialogs.splice(insertIdx, 0, updated)
      }
      return { ...data, dialogs }
    }

    // dialog 不存在，插入新 dialog
    const newDialog: Dialog = {
      peer_ref: peerRef,
      peer_type: peerRef.startsWith('u_') ? 'user' : peerRef.startsWith('ch_') ? 'channel' : 'chat',
      title: msg.sender_name || peerRef,
      last_message_preview: safeTruncateText(msg.text, 50),
      last_message_at: msg.sent_at,
      unread_count: currentPeerRef === peerRef ? 0 : 1,
    }
    // 插入到 pinned 之后的顶部
    let insertIdx = 0
    for (let i = 0; i < data.dialogs.length; i++) {
      if (data.dialogs[i].is_pinned) insertIdx = i + 1
      else break
    }
    const dialogs = [...data.dialogs]
    dialogs.splice(insertIdx, 0, newDialog)
    return { ...data, dialogs }
  })

  // 2. 写入对应 peer 的 messages cache
  upsertMessageInMessagesCache(queryClient, accountId, peerRef, msg)

  // 3. 非当前 peer 标记 stale
  if (currentPeerRef && peerRef !== currentPeerRef) {
    try {
      const chat = useChatStore()
      chat.markPeerStale(peerRef)
    } catch { /* store 未初始化 */ }
  }
}

function handleMessageEdited(
  event: RealtimeEvent,
  queryClient: QueryClient,
  accountId: number,
  currentPeerRef: string | null
): void {
  const msg = normalizeMessage(event.payload as Partial<ChatMessage> | undefined, event.peer_ref)
  if (!msg) return
  const msgID = telegramMessageID(msg)
  if (!msgID) return

  if (currentPeerRef && event.peer_ref === currentPeerRef) {
    patchMessagesQuery(queryClient, accountId, currentPeerRef, (old) =>
      patchMessageCollections(old, (messages) =>
        messages.map((m) => {
          if (telegramMessageID(m) === msgID) {
            return normalizeMessage({ ...m, text: msg.text, caption: msg.caption }, m.peer_ref) || m
          }
          return m
        })
      )
    )
  }

  queryClient.setQueryData(['dialogs', accountId], (old: unknown) => {
    const data = old as { ok: boolean; dialogs: Dialog[] } | undefined
    if (!data?.ok || !data.dialogs) return old

    const dialogs = data.dialogs.map((d) => {
      if (d.peer_ref === event.peer_ref) {
        return {
          ...d,
          last_message_preview: safeTruncateText(msg.text, 50) || d.last_message_preview,
        }
      }
      return d
    })

    return { ...data, dialogs }
  })
}

function handleMessageDeleted(
  event: RealtimeEvent,
  queryClient: QueryClient,
  accountId: number,
  currentPeerRef: string | null
): void {
  const payload = event.payload as { telegram_message_ids?: number[]; message_ids?: number[] } | undefined
  const messageIds = payload?.telegram_message_ids || payload?.message_ids || []
  if (messageIds.length === 0) return

  const peerRef = event.peer_ref

  // peer_ref 为空时（私聊删除无法定位 peer），invalidate dialogs 让前端刷新
  if (!peerRef) {
    queryClient.invalidateQueries({ queryKey: ['dialogs', accountId] })
    if (currentPeerRef) {
      try {
        const chat = useChatStore()
        chat.markPeerStale(currentPeerRef)
      } catch { /* store 未初始化 */ }
    }
    return
  }

  // 从对应 peer 的 messages cache 中移除已删除消息
  patchMessagesQuery(queryClient, accountId, peerRef, (old) =>
    patchMessageCollections(old, (messages) =>
      messages.filter((m) => {
        const id = telegramMessageID(m)
        return !id || !messageIds.includes(id)
      })
    )
  )

  // 非当前 peer 标记 stale，确保切换时 reconcile
  if (currentPeerRef && peerRef !== currentPeerRef) {
    try {
      const chat = useChatStore()
      chat.markPeerStale(peerRef)
    } catch { /* store 未初始化 */ }
  }
}

function handleDialogUpserted(
  event: RealtimeEvent,
  queryClient: QueryClient,
  accountId: number
): void {
  const dlg = event.payload as Dialog | undefined
  if (!dlg) return

  queryClient.setQueryData(['dialogs', accountId], (old: unknown) => {
    const data = old as { ok: boolean; dialogs: Dialog[] } | undefined
    if (!data?.ok || !data.dialogs) return old

    const idx = data.dialogs.findIndex((d) => d.peer_ref === dlg.peer_ref)
    if (idx >= 0) {
      const dialogs = [...data.dialogs]
      dialogs[idx] = { ...dialogs[idx], ...dlg }
      return { ...data, dialogs }
    }

    return { ...data, dialogs: [dlg, ...data.dialogs] }
  })

  // dialog.upserted 也标记 stale，确保切换时 reconcile
  if (dlg.peer_ref) {
    try {
      const chat = useChatStore()
      chat.markPeerStale(dlg.peer_ref)
    } catch {
      // store 未初始化时忽略
    }
  }
}

function patchMessagesQuery(
  queryClient: QueryClient,
  accountId: number,
  peerRef: string,
  updater: (old: unknown) => unknown
): void {
  queryClient.setQueryData(['messages', accountId, peerRef], updater)
}

function patchMessageInCache(
  old: unknown,
  incoming: ChatMessage | undefined,
  mode: MessagePatchMode,
  localId?: string,
  errorText?: string
): unknown {
  return patchMessageCollections(old, (messages) => {
    if (mode === 'mark-failed' && localId) {
      return messages.map((m) =>
        m.local_id === localId || m.client_pending_id === localId
          ? { ...m, pending: false, status: 'failed' as const, error: errorText }
          : m
      )
    }
    if (!incoming) return messages
    if (mode === 'replace-local' && localId) {
      const idx = messages.findIndex((m) => m.local_id === localId || m.client_pending_id === localId)
      if (idx >= 0) {
        const next = [...messages]
        next[idx] = mergeMessages(messages[idx], incoming)
        return sortMessagesByTime(dedupeMessages(next))
      }
    }
    return upsertMessageList(messages, incoming)
  })
}

function patchMessageCollections(old: unknown, patchList: (messages: ChatMessage[]) => ChatMessage[]): unknown {
  const data = old as MessagesCache | undefined

  // 如果 cache 不存在或无效，创建初始结构
  if (!data?.ok) {
    const initial = patchList([])
    if (initial.length === 0) return old // 没有消息可写，保持原样
    return { ok: true, messages: initial, stale: true, source: 'realtime' }
  }

  let changed = false
  const next: MessagesCache = { ...data }

  if (Array.isArray(data.messages)) {
    next.messages = patchList(data.messages)
    changed = true
  }
  if (Array.isArray(data.older_messages)) {
    next.older_messages = patchList(data.older_messages)
    changed = true
  }
  if (Array.isArray(data.pages)) {
    next.pages = data.pages.map((page) => {
      if (!Array.isArray(page.messages)) return page
      changed = true
      return { ...page, messages: patchList(page.messages) }
    })
  }

  return changed ? next : old
}

function upsertMessageList(messages: ChatMessage[], incoming: ChatMessage): ChatMessage[] {
  const incomingID = telegramMessageID(incoming)
  const idx = messages.findIndex((m) => {
    const existingID = telegramMessageID(m)
    if (incomingID && existingID && incomingID === existingID) return true
    if (incoming.local_id && m.local_id === incoming.local_id) return true
    if (incoming.client_pending_id && m.client_pending_id === incoming.client_pending_id) return true
    return isConservativeOutgoingMatch(m, incoming)
  })

  if (idx >= 0) {
    const next = [...messages]
    next[idx] = mergeMessages(messages[idx], incoming)
    return sortMessagesByTime(dedupeMessages(next))
  }

  return sortMessagesByTime(dedupeMessages([...messages, incoming]))
}

function dedupeMessages(messages: ChatMessage[]): ChatMessage[] {
  const seenTelegramIDs = new Set<number>()
  const seenLocalIDs = new Set<string>()
  const result: ChatMessage[] = []

  for (const msg of messages) {
    const id = telegramMessageID(msg)
    if (id) {
      if (seenTelegramIDs.has(id)) continue
      seenTelegramIDs.add(id)
    }
    const localID = msg.local_id || msg.client_pending_id
    if (localID) {
      if (seenLocalIDs.has(localID)) continue
      seenLocalIDs.add(localID)
    }
    result.push(msg)
  }
  return result
}

function mergeMessages(existing: ChatMessage, incoming: ChatMessage): ChatMessage {
  return {
    ...existing,
    ...incoming,
    id: incoming.id || existing.id,
    telegram_message_id: incoming.telegram_message_id || existing.telegram_message_id,
    local_id: existing.local_id || incoming.local_id,
    client_pending_id: existing.client_pending_id || incoming.client_pending_id,
    pending: incoming.pending ?? false,
    status: incoming.status || 'sent',
  }
}

function normalizeMessage(raw: Partial<ChatMessage> | undefined, fallbackPeerRef?: string): ChatMessage | undefined {
  if (!raw) return undefined
  const rawID = Number(raw.telegram_message_id ?? raw.id)
  const telegramID = Number.isFinite(rawID) && rawID > 0 ? rawID : undefined
  const messageType = (raw.message_type || raw.kind || 'text') as MessageKind
  const numericID = Number(raw.id)
  const id = telegramID ?? (Number.isFinite(numericID) && numericID !== 0
    ? numericID
    : negativeLocalID(raw.local_id || raw.client_pending_id))

  return {
    id,
    telegram_message_id: telegramID,
    local_id: raw.local_id,
    client_pending_id: raw.client_pending_id,
    pending: raw.pending ?? false,
    peer_ref: raw.peer_ref || fallbackPeerRef || '',
    direction: raw.direction || (raw.is_outgoing ? 'out' : 'in'),
    sender_name: raw.sender_name || '',
    text: raw.text || '',
    sent_at: raw.sent_at || new Date().toISOString(),
    is_outgoing: raw.is_outgoing ?? raw.direction === 'out',
    status: raw.status || 'sent',
    message_type: messageType,
    kind: raw.kind,
    caption: raw.caption,
    media: raw.media,
  }
}

function negativeLocalID(seed?: string): number {
  if (!seed) return -Date.now()
  let hash = 0
  for (let i = 0; i < seed.length; i++) {
    hash = (hash * 31 + seed.charCodeAt(i)) | 0
  }
  return -Math.abs(hash || Date.now())
}

function telegramMessageID(msg: ChatMessage): number | undefined {
  if (msg.telegram_message_id && msg.telegram_message_id > 0) return msg.telegram_message_id
  if (typeof msg.id === 'number' && msg.id > 0) return msg.id
  return undefined
}

function isConservativeOutgoingMatch(existing: ChatMessage, incoming: ChatMessage): boolean {
  const onePending = Boolean(existing.pending) !== Boolean(incoming.pending)
  if (!onePending || !existing.is_outgoing || !incoming.is_outgoing) return false
  if (existing.peer_ref !== incoming.peer_ref) return false
  if (existing.text.trim() !== incoming.text.trim()) return false
  const delta = Math.abs(new Date(existing.sent_at).getTime() - new Date(incoming.sent_at).getTime())
  return Number.isFinite(delta) && delta <= 30_000
}

/**
 * sortMessagesAsc 按 sent_at 正序排列消息。
 *
 * 排序规则：
 * 1. sent_at 升序（使用 Date.getTime() 比较，避免 ISO 字符串精度差异导致错序）
 * 2. sent_at 相同时，telegram_message_id 升序
 * 3. telegram_message_id 缺失时，id 升序
 *
 * 不改变原数组引用。
 */
export function sortMessagesAsc(messages: ChatMessage[]): ChatMessage[] {
  return [...messages].sort((a, b) => {
    const aTime = new Date(a.sent_at).getTime()
    const bTime = new Date(b.sent_at).getTime()
    if (aTime !== bTime) return aTime - bTime

    // sent_at 相同，用 telegram_message_id 兜底
    const aID = a.telegram_message_id || a.id || 0
    const bID = b.telegram_message_id || b.id || 0
    return aID - bID
  })
}

// 向后兼容别名
const sortMessagesByTime = sortMessagesAsc
