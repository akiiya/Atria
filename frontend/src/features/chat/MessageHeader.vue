<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useChatStore } from '@/stores/chat'
import { useQueryClient } from '@tanstack/vue-query'
import { fetchMessages } from '@/api/chat'

const props = defineProps<{ peerRef: string; title?: string; accountId?: number }>()
const emit = defineEmits<{ refresh: [] }>()

const router = useRouter()
const chat = useChatStore()
const queryClient = useQueryClient()

function goBack() {
  chat.selectPeer(null)
  router.push('/chats')
}

function forceRefreshMessages() {
  if (!props.accountId || !props.peerRef) return
  queryClient.fetchQuery({
    queryKey: ['messages', props.accountId, props.peerRef],
    queryFn: () => fetchMessages(props.peerRef, 50, undefined, true),
  })
  emit('refresh')
}
</script>

<template>
  <div class="message-header">
    <button class="btn-back mobile-only" @click="goBack">←</button>
    <div class="message-header-info">
      <span class="message-header-title">{{ title || peerRef }}</span>
    </div>
    <button class="btn-icon" @click="forceRefreshMessages" title="刷新">↻</button>
  </div>
</template>
