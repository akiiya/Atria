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

const { data, isLoading, error, refetch } = useQuery({
  queryKey: computed(() => ['messages', props.accountId, props.peerRef]),
  queryFn: () => fetchMessages(props.peerRef, 50),
  enabled: computed(() => !!props.peerRef && !!props.accountId),
  retry: 1,
  staleTime: 30_000,
  refetchOnWindowFocus: false,
})

// 判断是否需要 latest reconcile
function shouldReconcilePeer(peerRef: string): boolean {
  // 1. peer 被标记为 stale
  if (chat.isPeerStale(peerRef)) return true

  // 2. messages cache 不存在或为空
  const cached = queryClient.getQueryData(['messages', props.accountId, peerRef]) as { ok?: boolean; messages?: ChatMessage[] } | undefined
  if (!cached?.ok || !cached.messages || cached.messages.length === 0) return true

  // 3. dialog 的 last_message_at 比 messages newest 更新
  const dialogsData = queryClient.getQueryData(['dialogs', props.accountId]) as { ok?: boolean; dialogs?: Dialog[] } | undefined
  if (dialogsData?.ok && dialogsData.dialogs) {
    const dialog = dialogsData.dialogs.find(d => d.peer_ref === peerRef)
    if (dialog?.last_message_at && cached.messages.length > 0) {
      const newestMsg = cached.messages[cached.messages.length - 1]
      if (newestMsg?.sent_at && dialog.last_message_at > newestMsg.sent_at) return true
    }
    // 4. dialog unread_count > 0
    if (dialog?.unread_count && dialog.unread_count > 0) return true
  }

  return false
}

// 防止并发 reconcile：同一 peer 同一时间只跑一个
let reconcilingPeer: string | null = null

// latest reconcile：拉取最新消息并 merge
async function reconcileLatestForPeer(peerRef: string, _reason: string) {
  if (reconcilingPeer === peerRef) return // 已在 reconcile
  reconcilingPeer = peerRef
  chat.clearPeerStale(peerRef)

  try {
    const result = await fetchMessages(peerRef, 50, undefined, true)
    if (result.ok && result.messages && result.messages.length > 0) {
      // merge 到当前 messages cache
      queryClient.setQueryData(['messages', props.accountId, peerRef], (old: unknown) => {
        const existing = old as { ok?: boolean; messages?: ChatMessage[]; older_messages?: ChatMessage[] } | undefined
        const existingMsgs = existing?.messages || []
        const merged = mergeByTelegramID(existingMsgs, result.messages!)
        return {
          ok: true,
          messages: merged,
          stale: false,
          source: result.source || 'telegram',
          has_older: result.has_older,
          oldest_message_id: result.oldest_message_id,
          newest_message_id: result.newest_message_id,
        }
      })
    }
  } catch {
    // 失败时保留旧 cache，不清理
  } finally {
    if (reconcilingPeer === peerRef) reconcilingPeer = null
  }
}

// 按 telegram_message_id 合并去重
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

// 监听 peerRef 变化，触发 reconcile 判断
// 不能只依赖 onMounted，因为 peerRef 变化时组件可能不重建
onMounted(() => {
  if (shouldReconcilePeer(props.peerRef)) {
    reconcileLatestForPeer(props.peerRef, 'mount')
  }
})

const messageListRef = ref<InstanceType<typeof MessageList> | null>(null)
const olderPages = ref<ChatMessage[]>([])
const hasOlder = ref(true)
const loadingOlder = ref(false)
const olderError = ref<string | null>(null)

function messageKey(msg: ChatMessage): string {
  if (msg.telegram_message_id) return `tg:${msg.telegram_message_id}`
  if (msg.local_id) return `local:${msg.local_id}`
  return `id:${msg.id}`
}

function messagePaginationID(msg: ChatMessage): number {
  return msg.telegram_message_id || msg.id
}

const recentMessages = computed(() => data.value?.messages || [])
const olderMessages = computed(() => data.value?.older_messages || olderPages.value)

const allMessages = computed(() => {
  const map = new Map<string, ChatMessage>()
  for (const msg of olderMessages.value) {
    map.set(messageKey(msg), msg)
  }
  for (const msg of recentMessages.value) {
    map.set(messageKey(msg), msg)
  }
  return sortMessagesAsc(Array.from(map.values()))
})

const isStale = computed(() => data.value?.stale || false)

watch(() => props.peerRef, () => {
  olderPages.value = []
  hasOlder.value = true
  loadingOlder.value = false
  olderError.value = null
})

async function loadOlder() {
  if (loadingOlder.value || !hasOlder.value) return

  const oldestMsg = allMessages.value[0]
  if (!oldestMsg) return

  // 记录滚动位置，用于 older pagination anchor
  messageListRef.value?.prepareOlderAnchor()

  loadingOlder.value = true
  olderError.value = null

  try {
    const result = await fetchMessages(props.peerRef, 50, messagePaginationID(oldestMsg))
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
          hasOlder.value = result.has_older ?? true
          queryClient.setQueryData(['messages', props.accountId, props.peerRef], (old: unknown) => {
            const cached = old as Record<string, unknown> | undefined
            if (!cached) return old
            return { ...cached, older_messages: olderPages.value }
          })
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

  // 恢复滚动位置
  messageListRef.value?.restoreOlderAnchor()
}

function handleSent() {
  queryClient.invalidateQueries({ queryKey: ['dialogs', props.accountId] })
}
</script>

<template>
  <div class="message-panel">
    <MessageHeader :peer-ref="peerRef" :title="dialogTitle || ''" :account-id="accountId" @refresh="refetch()" />

    <div v-if="isLoading && allMessages.length === 0" class="message-body">
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
    <div v-else-if="error && allMessages.length === 0" class="message-body">
      <ErrorBanner :message="(error as Error).message" @dismiss="refetch()" />
    </div>
    <div v-else class="message-body">
      <div v-if="isStale && isLoading" class="message-stale-hint">正在刷新...</div>
      <MessageList
        ref="messageListRef"
        :messages="allMessages"
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
