<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import type { ChatMessage } from '@/types/chat'
import MessageBubble from './MessageBubble.vue'
import ServiceMessage from './ServiceMessage.vue'
import DateDivider from './DateDivider.vue'

const props = defineProps<{ messages: ChatMessage[] }>()

const scrollParent = ref<HTMLElement | null>(null)

const virtualizer = useVirtualizer({
  count: props.messages.length,
  getScrollElement: () => scrollParent.value,
  estimateSize: () => 60,
  overscan: 10,
})

watch(() => props.messages.length, async () => {
  await nextTick()
  if (scrollParent.value) {
    const el = scrollParent.value
    const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 150
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
    <div :style="{ height: virtualizer.getTotalSize() + 'px', position: 'relative' }">
      <div
        v-for="row in virtualizer.getVirtualItems()"
        :key="String(row.key)"
        :style="{
          position: 'absolute',
          top: row.start + 'px',
          width: '100%',
        }"
      >
        <DateDivider v-if="isNewDay(row.index)" :date="messages[row.index].sent_at" />
        <ServiceMessage v-if="messages[row.index].message_type === 'service'" :message="messages[row.index]" />
        <MessageBubble v-else :message="messages[row.index]" />
      </div>
    </div>
  </div>
</template>
