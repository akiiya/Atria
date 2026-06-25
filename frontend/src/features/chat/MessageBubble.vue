<script setup lang="ts">
import { useI18n } from '@/i18n'
import type { ChatMessage, PeerType } from '@/types/chat'
import MediaMessage from './MediaMessage.vue'

const { t } = useI18n()
const props = defineProps<{ message: ChatMessage; peerType?: PeerType }>()

// 是否显示 sender label：仅群聊/频道的 incoming 消息显示
const showSenderLabel = !props.message.is_outgoing
  && !!props.message.sender_name
  && props.peerType !== 'user'

function escapeHtml(str: string): string {
  const div = document.createElement('div')
  div.appendChild(document.createTextNode(str))
  return div.innerHTML
}

function linkify(text: string): string {
  const escaped = escapeHtml(text)
  return escaped.replace(
    /(https?:\/\/[^\s<]+)/g,
    '<a href="$1" target="_blank" rel="noopener noreferrer">$1</a>'
  )
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

const isMedia = ['photo', 'document', 'sticker', 'video', 'voice', 'audio'].includes(props.message.message_type)
</script>

<template>
  <div :class="['message-bubble', message.is_outgoing ? 'outgoing' : 'incoming']">
    <div v-if="showSenderLabel" class="message-sender">
      {{ message.sender_name }}
    </div>
    <MediaMessage v-if="isMedia" :message="message" />
    <div v-else-if="message.message_type === 'text'" class="message-text" v-html="linkify(message.text)" />
    <div v-else class="message-unsupported">
      {{ t('chat.unsupportedType').replace('{type}', message.message_type) }}
    </div>
    <div class="message-meta">
      <span class="message-time">{{ formatTime(message.sent_at) }}</span>
      <span v-if="message.is_outgoing" :class="['message-status', message.status]">
        {{ message.status === 'sent' ? '✓' : message.status === 'failed' ? '✕' : '?' }}
      </span>
    </div>
  </div>
</template>
