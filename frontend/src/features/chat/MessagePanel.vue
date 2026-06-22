<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { fetchMessages } from '@/api/chat'
import { useChatStore } from '@/stores/chat'
import MessageHeader from './MessageHeader.vue'
import MessageList from './MessageList.vue'
import MessageComposer from './MessageComposer.vue'
import ErrorBanner from '@/components/ErrorBanner.vue'
import type { ChatMessage } from '@/types/chat'

const props = defineProps<{ peerRef: string; accountId: number; dialogTitle?: string }>()
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

// 切换到此 peer 时，如果标记为 stale，强制刷新最新消息
onMounted(() => {
  if (chat.isPeerStale(props.peerRef)) {
    chat.clearPeerStale(props.peerRef)
    // 使用 force_refresh=true 跳过缓存，直接从 Telegram 拉取最新消息
    queryClient.fetchQuery({
      queryKey: ['messages', props.accountId, props.peerRef],
      queryFn: () => fetchMessages(props.peerRef, 50, undefined, true),
    })
  }
})

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
  return Array.from(map.values()).sort((a, b) =>
    // 直接比较 ISO 字符串，避免创建 Date 对象
    a.sent_at < b.sent_at ? -1 : a.sent_at > b.sent_at ? 1 : 0
  )
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
        :messages="allMessages"
        :has-older="hasOlder"
        :loading-older="loadingOlder"
        :older-error="olderError"
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
