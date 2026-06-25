<script setup lang="ts">
import { useI18n } from '@/i18n'
import type { ChatMessage } from '@/types/chat'

const { t } = useI18n()
defineProps<{ message: ChatMessage }>()

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}
</script>

<template>
  <div class="service-message">
    <span class="service-text">{{ message.text || t('chat.serviceMessage') }}</span>
    <span class="service-time">{{ formatTime(message.sent_at) }}</span>
  </div>
</template>
