import type { QueryClient } from '@tanstack/vue-query'
import type { RealtimeEvent } from './ws'
import type { ChatMessage, Dialog } from '@/types/chat'

/**
 * 处理 WebSocket 实时事件，局部 patch TanStack Query cache。
 */
export function handleRealtimeEvent(
  event: RealtimeEvent,
  queryClient: QueryClient,
  currentAccountId: number | null,
  currentPeerRef: string | null
): void {
  // 只处理当前 account 的事件
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
      // 状态事件 - 通过 runtime status query 刷新处理
      queryClient.invalidateQueries({ queryKey: ['runtime-status', currentAccountId] })
      break
  }
}

function handleMessageNew(
  event: RealtimeEvent,
  queryClient: QueryClient,
  accountId: number,
  currentPeerRef: string | null
): void {
  const msg = event.payload as ChatMessage | undefined
  if (!msg) return

  // 更新 dialogs query
  queryClient.setQueryData(['dialogs', accountId], (old: unknown) => {
    const data = old as { ok: boolean; dialogs: Dialog[] } | undefined
    if (!data?.ok || !data.dialogs) return old

    const dialogs = data.dialogs.map((d) => {
      if (d.peer_ref === event.peer_ref) {
        return {
          ...d,
          last_message_preview: msg.text?.slice(0, 50) || '',
          last_message_at: msg.sent_at,
          unread_count: currentPeerRef === event.peer_ref ? 0 : (d.unread_count || 0) + 1,
        }
      }
      return d
    })

    // 移动更新的 dialog 到前面（非 pinned 的情况下）
    const updatedIdx = dialogs.findIndex((d) => d.peer_ref === event.peer_ref)
    if (updatedIdx > 0 && !dialogs[updatedIdx].is_pinned) {
      const [updated] = dialogs.splice(updatedIdx, 1)
      // 找到第一个非 pinned 的位置插入
      let insertIdx = 0
      for (let i = 0; i < dialogs.length; i++) {
        if (dialogs[i].is_pinned) insertIdx = i + 1
        else break
      }
      dialogs.splice(insertIdx, 0, updated)
    }

    return { ...data, dialogs }
  })

  // 如果是当前打开的 peer，更新 messages
  if (currentPeerRef && event.peer_ref === currentPeerRef) {
    queryClient.setQueryData(['messages', accountId, currentPeerRef], (old: unknown) => {
      const data = old as { ok: boolean; messages: ChatMessage[] } | undefined
      if (!data?.ok) return old

      // 去重：检查 id
      const exists = data.messages.some(
        (m) => m.id === msg.id
      )
      if (exists) return old

      // 插入并保持时间正序
      const messages = [...data.messages, msg].sort(
        (a, b) => new Date(a.sent_at).getTime() - new Date(b.sent_at).getTime()
      )

      return { ...data, messages }
    })
  }
}

function handleMessageEdited(
  event: RealtimeEvent,
  queryClient: QueryClient,
  accountId: number,
  currentPeerRef: string | null
): void {
  const msg = event.payload as ChatMessage | undefined
  if (!msg) return

  // 更新当前 peer 的 messages
  if (currentPeerRef && event.peer_ref === currentPeerRef) {
    queryClient.setQueryData(['messages', accountId, currentPeerRef], (old: unknown) => {
      const data = old as { ok: boolean; messages: ChatMessage[] } | undefined
      if (!data?.ok) return old

      const messages = data.messages.map((m) => {
        if (m.id === msg.id) {
          return { ...m, text: msg.text, caption: msg.caption }
        }
        return m
      })

      return { ...data, messages }
    })
  }

  // 更新 dialog preview
  queryClient.setQueryData(['dialogs', accountId], (old: unknown) => {
    const data = old as { ok: boolean; dialogs: Dialog[] } | undefined
    if (!data?.ok || !data.dialogs) return old

    const dialogs = data.dialogs.map((d) => {
      if (d.peer_ref === event.peer_ref) {
        return {
          ...d,
          last_message_preview: msg.text?.slice(0, 50) || d.last_message_preview,
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
  const payload = event.payload as { message_ids?: number[] } | undefined
  const messageIds = payload?.message_ids || []
  if (messageIds.length === 0) return

  // 从当前 peer 的 messages 中删除
  if (currentPeerRef && event.peer_ref === currentPeerRef) {
    queryClient.setQueryData(['messages', accountId, currentPeerRef], (old: unknown) => {
      const data = old as { ok: boolean; messages: ChatMessage[] } | undefined
      if (!data?.ok) return old

      const messages = data.messages.filter(
        (m) => !messageIds.includes(m.id)
      )

      return { ...data, messages }
    })
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

    return data
  })
}
