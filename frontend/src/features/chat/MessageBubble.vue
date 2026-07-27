<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import type { ChatMessage, PeerType } from '@/types/chat'
import { renderMessageText } from '@/utils/textRenderer'
import MediaMessage from './MediaMessage.vue'

const MEDIA_KINDS = ['photo', 'document', 'sticker', 'video', 'voice', 'audio']

const { t } = useI18n()
// groupFirst/groupLast 用 number 类型避免 Vue 3 对可选 boolean 的 false 默认值。
// 1 = true, 0 = false, undefined = 非分组模式。
const props = defineProps<{
  message: ChatMessage
  peerType?: PeerType
  groupFirst?: number
  groupLast?: number
}>()

// 是否为分组模式（有任一分组 prop 被显式传入，即值为 0 或 1）
const isGrouped = computed(() => props.groupFirst !== undefined || props.groupLast !== undefined)

const groupFirst = computed(() => props.groupFirst === 1)
const groupLast = computed(() => props.groupLast === 1)

// 分组 CSS 类（仅在分组模式下应用，避免非分组消息被误标记）
const groupClasses = computed(() => {
  if (!isGrouped.value) return {}
  const gf = groupFirst.value
  const gl = groupLast.value
  return {
    'group-first': gf,
    'group-last': gl,
    'group-middle': !gf && !gl,
    'group-not-first': !gf,
    'group-not-last': !gl,
  }
})

// 是否显示 sender label：仅群聊/频道的 incoming 消息、且为分组首条时显示
// 默认显示（非分组模式），仅在分组模式下且非首条时隐藏
const showSenderLabel = computed(() => {
  if (props.message.is_outgoing) return false
  if (!props.message.sender_name) return false
  if (props.peerType === 'user') return false
  if (isGrouped.value && !groupFirst.value) return false
  return true
})

// 是否显示时间和状态：默认显示，仅在分组模式下且非末条时隐藏
const showMeta = computed(() => !isGrouped.value || groupLast.value)

const isMedia = computed(() => MEDIA_KINDS.includes(props.message.message_type))

// 消息正文渲染结果；message.text 或 entities 变化时自动重算
const renderedText = computed(() => renderMessageText(props.message.text, props.message.entities))

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}
</script>

<template>
  <div :class="[
    'message-bubble',
    message.is_outgoing ? 'outgoing' : 'incoming',
    groupClasses,
  ]">
    <div v-if="showSenderLabel" class="message-sender">
      {{ message.sender_name }}
    </div>
    <MediaMessage v-if="isMedia" :message="message" />
    <div v-else-if="message.message_type === 'text'" class="message-text" v-html="renderedText" />
    <div v-else class="message-unsupported">
      {{ t('chat.unsupportedType').replace('{type}', message.message_type) }}
    </div>
    <!-- 时间和状态：仅在分组末条或非分组消息时显示 -->
    <div v-if="showMeta" class="message-meta">
      <span class="message-time">{{ formatTime(message.sent_at) }}</span>
      <span v-if="message.is_outgoing" :class="['message-status', message.status]">
        {{ message.status === 'sent' ? '✓' : message.status === 'failed' ? '✕' : '?' }}
      </span>
    </div>
  </div>
</template>
