<script setup lang="ts">
import type { ChatMessage } from '@/types/chat'

defineProps<{ message: ChatMessage }>()

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}
</script>

<template>
  <div class="service-message">
    <span class="service-text">{{ message.text || '服务消息' }}</span>
    <span class="service-time">{{ formatTime(message.sent_at) }}</span>
  </div>
</template>
