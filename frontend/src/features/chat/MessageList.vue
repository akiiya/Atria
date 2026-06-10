<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import type { ChatMessage } from '@/types/chat'
import MessageBubble from './MessageBubble.vue'
import ServiceMessage from './ServiceMessage.vue'
import DateDivider from './DateDivider.vue'

const props = defineProps<{ messages: ChatMessage[] }>()

const scrollParent = ref<HTMLElement | null>(null)

// Auto-scroll to bottom when new messages arrive
watch(() => props.messages.length, async () => {
  await nextTick()
  if (scrollParent.value) {
    const el = scrollParent.value
    const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 200
    if (isNearBottom) {
      el.scrollTop = el.scrollHeight
    }
  }
})

function isNewDay(idx: number): boolean {
  if (idx === 0) return true
  const prev = new Date(props.messages[idx - 1].sent_at).toDateString()
  const curr = new Date(props.messages[idx].sent_at).toDateString()
  return prev !== curr
}
</script>

<template>
  <div ref="scrollParent" class="message-scroll-container">
    <div v-if="messages.length === 0" class="message-empty">
      暂无消息
    </div>
    <template v-for="(msg, idx) in messages" :key="msg.id || idx">
      <DateDivider v-if="isNewDay(idx)" :date="msg.sent_at" />
      <ServiceMessage v-if="msg.message_type === 'service'" :message="msg" />
      <MessageBubble v-else :message="msg" />
    </template>
  </div>
</template>

<style scoped>
.message-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-secondary);
  font-size: 14px;
}
</style>
