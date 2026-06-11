<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { fetchMessages } from '@/api/chat'
import MessageHeader from './MessageHeader.vue'
import MessageList from './MessageList.vue'
import MessageComposer from './MessageComposer.vue'
import ErrorBanner from '@/components/ErrorBanner.vue'
import type { ChatMessage } from '@/types/chat'

const props = defineProps<{ peerRef: string; accountId: number; dialogTitle?: string }>()
const queryClient = useQueryClient()

// 首屏消息查询
const { data, isLoading, error, refetch } = useQuery({
  queryKey: computed(() => ['messages', props.accountId, props.peerRef]),
  queryFn: () => fetchMessages(props.peerRef, 50),
  enabled: computed(() => !!props.peerRef && !!props.accountId),
  retry: 1,
  staleTime: 30_000,
})

// 分页状态
const olderPages = ref<ChatMessage[]>([])
const hasOlder = ref(true)
const loadingOlder = ref(false)
const olderError = ref<string | null>(null)

// 首屏消息
const recentMessages = computed(() => data.value?.messages || [])

// 合并所有消息并去重，按时间正序
const allMessages = computed(() => {
  const map = new Map<number, ChatMessage>()
  // 先加 older（较早的）
  for (const msg of olderPages.value) {
    map.set(msg.id, msg)
  }
  // 再加 recent（较新的，会覆盖重复的）
  for (const msg of recentMessages.value) {
    map.set(msg.id, msg)
  }
  // 按 sent_at 正序
  return Array.from(map.values()).sort((a, b) =>
    new Date(a.sent_at).getTime() - new Date(b.sent_at).getTime()
  )
})

const isStale = computed(() => data.value?.stale || false)

// 切换 peer 时重置分页状态
watch(() => props.peerRef, () => {
  olderPages.value = []
  hasOlder.value = true
  loadingOlder.value = false
  olderError.value = null
})

// 加载更早消息
async function loadOlder() {
  if (loadingOlder.value || !hasOlder.value) return

  // 找到当前最早消息的 ID
  const oldestMsg = allMessages.value[0]
  if (!oldestMsg) return

  loadingOlder.value = true
  olderError.value = null

  try {
    const result = await fetchMessages(props.peerRef, 50, oldestMsg.id)
    if (result.ok && result.messages) {
      if (result.messages.length === 0) {
        hasOlder.value = false
      } else {
        // 过滤掉已有的消息（去重）
        const existingIds = new Set(allMessages.value.map(m => m.id))
        const newMsgs = result.messages.filter(m => !existingIds.has(m.id))
        if (newMsgs.length === 0) {
          hasOlder.value = false
        } else {
          olderPages.value = [...newMsgs, ...olderPages.value]
          hasOlder.value = result.has_older ?? true
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

// 发送消息后刷新
function handleSent() {
  queryClient.invalidateQueries({ queryKey: ['messages', props.accountId, props.peerRef] })
  queryClient.invalidateQueries({ queryKey: ['dialogs', props.accountId] })
}
</script>

<template>
  <div class="message-panel">
    <MessageHeader :peer-ref="peerRef" :title="dialogTitle || ''" @refresh="refetch()" />

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
