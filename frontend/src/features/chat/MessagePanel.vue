<script setup lang="ts">
import { watch } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { fetchMessages } from '@/api/chat'
import MessageHeader from './MessageHeader.vue'
import MessageList from './MessageList.vue'
import MessageComposer from './MessageComposer.vue'
import LoadingSkeleton from '@/components/LoadingSkeleton.vue'
import ErrorBanner from '@/components/ErrorBanner.vue'

const props = defineProps<{ peerRef: string }>()

const { data, isLoading, error, refetch } = useQuery({
  queryKey: ['messages', props.peerRef],
  queryFn: () => fetchMessages(props.peerRef, 50),
  retry: 1,
})

watch(() => props.peerRef, () => refetch())

const messages = data.value?.messages || []
</script>

<template>
  <div class="message-panel">
    <MessageHeader :peer-ref="peerRef" @refresh="refetch()" />

    <div v-if="isLoading" class="message-body">
      <LoadingSkeleton :count="8" />
    </div>
    <div v-else-if="error" class="message-body">
      <ErrorBanner :message="(error as Error).message" @dismiss="refetch()" />
    </div>
    <div v-else class="message-body">
      <MessageList :messages="messages" />
    </div>

    <MessageComposer :peer-ref="peerRef" @sent="refetch()" />
  </div>
</template>
