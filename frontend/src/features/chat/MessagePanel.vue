<script setup lang="ts">
import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { fetchMessages } from '@/api/chat'
import MessageHeader from './MessageHeader.vue'
import MessageList from './MessageList.vue'
import MessageComposer from './MessageComposer.vue'
import ErrorBanner from '@/components/ErrorBanner.vue'

const props = defineProps<{ peerRef: string; accountId: number; dialogTitle?: string }>()

const { data, isLoading, error, refetch } = useQuery({
  queryKey: computed(() => ['messages', props.accountId, props.peerRef]),
  queryFn: () => fetchMessages(props.peerRef, 50),
  enabled: computed(() => !!props.peerRef && !!props.accountId),
  retry: 1,
  staleTime: 30_000,
})

const messages = computed(() => data.value?.messages || [])
const isStale = computed(() => data.value?.stale || false)
</script>

<template>
  <div class="message-panel">
    <MessageHeader :peer-ref="peerRef" :title="dialogTitle || ''" @refresh="refetch()" />

    <div v-if="isLoading && messages.length === 0" class="message-body">
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
    <div v-else-if="error && messages.length === 0" class="message-body">
      <ErrorBanner :message="(error as Error).message" @dismiss="refetch()" />
    </div>
    <div v-else class="message-body">
      <div v-if="isStale && isLoading" class="message-stale-hint">正在刷新...</div>
      <MessageList :messages="messages" />
    </div>

    <MessageComposer :peer-ref="peerRef" :account-id="accountId" @sent="refetch()" />
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
