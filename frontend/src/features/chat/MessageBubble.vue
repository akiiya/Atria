<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import type { ChatMessage, PeerType } from '@/types/chat'
import { renderMessageText } from '@/utils/textRenderer'
import MediaMessage from './MediaMessage.vue'

const MEDIA_KINDS = ['photo', 'document', 'sticker', 'video', 'voice', 'audio']

const { t } = useI18n()
const props = defineProps<{ message: ChatMessage; peerType?: PeerType }>()

// 是否显示 sender label：仅群聊/频道的 incoming 消息显示
const showSenderLabel = computed(() =>
  !props.message.is_outgoing
  && !!props.message.sender_name
  && props.peerType !== 'user'
)

const isMedia = computed(() => MEDIA_KINDS.includes(props.message.message_type))

// 消息正文渲染结果；message.text 或 entities 变化时自动重算
const renderedText = computed(() => renderMessageText(props.message.text, props.message.entities))

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}
</script>

<template>
  <div :class="['message-bubble', message.is_outgoing ? 'outgoing' : 'incoming']">
    <div v-if="showSenderLabel" class="message-sender">
      {{ message.sender_name }}
    </div>
    <MediaMessage v-if="isMedia" :message="message" />
    <div v-else-if="message.message_type === 'text'" class="message-text" v-html="renderedText" />
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
