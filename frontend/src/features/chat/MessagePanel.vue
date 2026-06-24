<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { fetchMessages } from '@/api/chat'
import { sortMessagesAsc } from '@/realtime/handler'
import { useChatStore } from '@/stores/chat'
import MessageHeader from './MessageHeader.vue'
import MessageList from './MessageList.vue'
import MessageComposer from './MessageComposer.vue'
import ErrorBanner from '@/components/ErrorBanner.vue'
import type { ChatMessage, Dialog, PeerType } from '@/types/chat'

const props = defineProps<{ peerRef: string; accountId: number; dialogTitle?: string; peerType?: PeerType }>()
const queryClient = useQueryClient()
const chat = useChatStore()

// ── Latest Page：只从 API 获取最近 N 条 ──
const INITIAL_LATEST_LIMIT = 20
const LOAD_OLDER_LIMIT = 30

const { data, isLoading, isFetching, error, refetch } = useQuery({
  queryKey: computed(() => ['messages', props.accountId, props.peerRef]),
  queryFn: () => fetchMessages(props.peerRef, INITIAL_LATEST_LIMIT),
  enabled: computed(() => !!props.peerRef && !!props.accountId),
  retry: 1,
  staleTime: 30_000,
  refetchOnWindowFocus: false,
})

// 判断是否需要 latest reconcile
function shouldReconcilePeer(peerRef: string): boolean {
  if (chat.isPeerStale(peerRef)) return true

  const cached = queryClient.getQueryData(['messages', props.accountId, peerRef]) as { ok?: boolean; messages?: ChatMessage[] } | undefined
  if (!cached?.ok || !cached.messages || cached.messages.length === 0) return true

  const dialogsData = queryClient.getQueryData(['dialogs', props.accountId]) as { ok?: boolean; dialogs?: Dialog[] } | undefined
  if (dialogsData?.ok && dialogsData.dialogs) {
    const dialog = dialogsData.dialogs.find(d => d.peer_ref === peerRef)
    if (dialog?.last_message_at && cached.messages.length > 0) {
      const newestMsg = cached.messages[cached.messages.length - 1]
      if (newestMsg?.sent_at && dialog.last_message_at > newestMsg.sent_at) return true
    }
    if (dialog?.unread_count && dialog.unread_count > 0) return true
  }

  return false
}

let reconcilingPeer: string | null = null

async function reconcileLatestForPeer(peerRef: string, _reason: string) {
  if (reconcilingPeer === peerRef) return
  reconcilingPeer = peerRef
  chat.clearPeerStale(peerRef)

  try {
    const result = await fetchMessages(peerRef, INITIAL_LATEST_LIMIT, undefined, true)
    if (result.ok && result.messages && result.messages.length > 0) {
      queryClient.setQueryData(['messages', props.accountId, peerRef], (old: unknown) => {
        const existing = old as { ok?: boolean; messages?: ChatMessage[] } | undefined
        const existingMsgs = existing?.messages || []
        // Replace cache with fresh data, but preserve pending optimistic messages
        const pendingMsgs = existingMsgs.filter(m => m.pending)
        const freshMsgs = result.messages!
        const merged = mergeByTelegramID(pendingMsgs, freshMsgs)
        // Cap cache to prevent unbounded growth
        const capped = merged.length > INITIAL_LATEST_LIMIT
          ? sortMessagesAsc(merged).slice(-INITIAL_LATEST_LIMIT)
          : merged
        return {
          ok: true,
          messages: capped,
          stale: false,
          source: result.source || 'telegram',
          has_older: result.has_older,
          oldest_message_id: result.oldest_message_id,
          newest_message_id: result.newest_message_id,
        }
      })
    }
  } catch {
    // 失败时保留旧 cache
  } finally {
    if (reconcilingPeer === peerRef) reconcilingPeer = null
  }
}

function mergeByTelegramID(existing: ChatMessage[], incoming: ChatMessage[]): ChatMessage[] {
  const map = new Map<string, ChatMessage>()
  for (const msg of existing) {
    const key = msg.telegram_message_id ? `tg:${msg.telegram_message_id}` : (msg.local_id ? `local:${msg.local_id}` : `id:${msg.id}`)
    map.set(key, msg)
  }
  for (const msg of incoming) {
    const key = msg.telegram_message_id ? `tg:${msg.telegram_message_id}` : (msg.local_id ? `local:${msg.local_id}` : `id:${msg.id}`)
    const prev = map.get(key)
    if (!prev || (msg.telegram_message_id && !prev.telegram_message_id)) {
      map.set(key, msg)
    }
  }
  return sortMessagesAsc(Array.from(map.values()))
}

onMounted(() => {
  if (shouldReconcilePeer(props.peerRef)) {
    reconcileLatestForPeer(props.peerRef, 'mount')
  }
})

// ── Visible Window 状态 ──
// recentMessages：最新一页（来自 API / TanStack Query cache）
// olderPages：用户上滑加载的历史页（独立管理，不从 query cache 恢复）
const messageListRef = ref<InstanceType<typeof MessageList> | null>(null)
const olderPages = ref<ChatMessage[]>([])
const hasOlder = ref(true)
const loadingOlder = ref(false)
const olderError = ref<string | null>(null)

const recentMessages = computed(() => data.value?.messages || [])

// peer switch 时清空 olderPages，重置 visibleCap，确保只显示 latest page
watch(() => props.peerRef, () => {
  olderPages.value = []
  hasOlder.value = true
  loadingOlder.value = false
  olderError.value = null
  visibleCap.value = INITIAL_LATEST_LIMIT
})

function messageKey(msg: ChatMessage): string {
  if (msg.telegram_message_id) return `tg:${msg.telegram_message_id}`
  if (msg.local_id) return `local:${msg.local_id}`
  return `id:${msg.id}`
}

function messagePaginationID(msg: ChatMessage): number {
  return msg.telegram_message_id || msg.id
}

// allMessages = olderPages + recentMessages，按 sent_at ASC，去重
const allMessages = computed(() => {
  const map = new Map<string, ChatMessage>()
  for (const msg of olderPages.value) {
    map.set(messageKey(msg), msg)
  }
  for (const msg of recentMessages.value) {
    map.set(messageKey(msg), msg)
  }
  return sortMessagesAsc(Array.from(map.values()))
})

// visibleMessages：实际渲染的消息列表
// 动态 cap：初始显示最近 INITIAL_LATEST_LIMIT 条，
// 用户每次上滑加载历史后 cap 自动增长，形成瀑布流分页效果
// 防止 TanStack Query cache 累积过多消息时一次性全部渲染
const visibleCap = ref(INITIAL_LATEST_LIMIT)

const visibleMessages = computed(() => {
  const all = allMessages.value
  const cap = visibleCap.value
  if (all.length <= cap) {
    console.info('[visibleMessages]', { peer: props.peerRef, allCount: all.length, cap, rendered: all.length })
    return all
  }
  const sliced = all.slice(-cap)
  console.info('[visibleMessages]', { peer: props.peerRef, allCount: all.length, cap, rendered: sliced.length })
  return sliced
})

const isStale = computed(() => data.value?.stale || false)

// ── Older Pagination ──
async function loadOlder() {
  if (loadingOlder.value || !hasOlder.value) {
    console.info('[loadOlder] blocked', { loading: loadingOlder.value, hasOlder: hasOlder.value })
    return
  }

  const oldestMsg = allMessages.value[0]
  if (!oldestMsg) {
    console.info('[loadOlder] no oldest message')
    return
  }

  console.info('[loadOlder] starting', { peer: props.peerRef, beforeId: messagePaginationID(oldestMsg), allCount: allMessages.value.length })
  loadingOlder.value = true
  olderError.value = null

  try {
    const result = await fetchMessages(props.peerRef, LOAD_OLDER_LIMIT, messagePaginationID(oldestMsg))
    console.info('[loadOlder] result', { ok: result.ok, count: result.messages?.length, hasOlder: result.has_older })
    if (result.ok && result.messages) {
      if (result.messages.length === 0) {
        hasOlder.value = false
      } else {
        const existingIds = new Set(allMessages.value.map(messageKey))
        const newMsgs = result.messages.filter((m) => !existingIds.has(messageKey(m)))
        if (newMsgs.length === 0) {
          hasOlder.value = false
        } else {
          olderPages.value = [...newMsgs, ...olderPages.value]
          // 动态扩大 visibleCap，让新加载的旧消息进入可见区域
          visibleCap.value += newMsgs.length
          hasOlder.value = result.has_older ?? (result.messages.length >= LOAD_OLDER_LIMIT)
        }
      }
    } else {
      hasOlder.value = result.has_older ?? false
    }
  } catch (e: unknown) {
    olderError.value = e instanceof Error ? e.message : '加载历史消息失败'
  } finally {
    loadingOlder.value = false
  }
}

function handleSent() {
  queryClient.invalidateQueries({ queryKey: ['dialogs', props.accountId] })
}
</script>

<template>
  <div class="message-panel">
    <MessageHeader
      :peer-ref="peerRef"
      :title="dialogTitle || ''"
      :account-id="accountId"
      :syncing="isFetching"
      :stale="isStale && isFetching"
      @refresh="refetch()"
    />

    <div v-if="isLoading && visibleMessages.length === 0" class="message-body">
      <div class="message-loading">
        <div class="skeleton-list">
          <div v-for="i in 6" :key="i" class="skeleton-item">
            <div class="skeleton-avatar"></div>
            <div class="skeleton-lines">
              <div class="skeleton-line long"></div>
              <div class="skeleton-line short"></div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div v-else-if="error && visibleMessages.length === 0" class="message-body">
      <ErrorBanner :message="(error as Error).message" @dismiss="refetch()" />
    </div>
    <div v-else class="message-body">
      <div v-if="isStale && isFetching" class="message-stale-hint">正在刷新...</div>
      <MessageList
        ref="messageListRef"
        :messages="visibleMessages"
        :has-older="hasOlder"
        :loading-older="loadingOlder"
        :older-error="olderError"
        :peer-type="peerType"
        :peer-ref="peerRef"
        @load-older="loadOlder"
      />
    </div>

    <MessageComposer :peer-ref="peerRef" :account-id="accountId" @sent="handleSent" />
  </div>
</template>

<style scoped>
.message-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 24px;
}

.message-stale-hint {
  text-align: center;
  padding: 4px;
  font-size: 12px;
  color: var(--text-secondary);
  background: var(--bg-tertiary);
}
</style>
