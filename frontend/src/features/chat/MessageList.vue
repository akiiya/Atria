<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import type { ChatMessage } from '@/types/chat'
import MessageBubble from './MessageBubble.vue'
import ServiceMessage from './ServiceMessage.vue'
import DateDivider from './DateDivider.vue'

const props = defineProps<{
  messages: ChatMessage[]
  hasOlder: boolean
  loadingOlder: boolean
  olderError: string | null
}>()

const emit = defineEmits<{ 'load-older': [] }>()

const scrollParent = ref<HTMLElement | null>(null)
const showNewMessageHint = ref(false)
const isInitialLoad = ref(true)

// 检查是否接近底部（200px 阈值）
function isNearBottom(): boolean {
  if (!scrollParent.value) return true
  const el = scrollParent.value
  return el.scrollHeight - el.scrollTop - el.clientHeight < 200
}

// 滚动到底部
function scrollToBottom() {
  if (!scrollParent.value) return
  scrollParent.value.scrollTop = scrollParent.value.scrollHeight
}

// 监听消息变化
watch(() => props.messages.length, async (newLen, oldLen) => {
  await nextTick()
  if (!scrollParent.value) return

  // 首次加载
  if (isInitialLoad.value && newLen > 0) {
    isInitialLoad.value = false
    scrollToBottom()
    return
  }

  // 消息数增加（非首次）
  if (oldLen !== undefined && newLen > oldLen) {
    // 检查最后一条是否是 outgoing（发送消息触发）
    const lastMsg = props.messages[props.messages.length - 1]
    if (lastMsg?.is_outgoing && isNearBottom()) {
      scrollToBottom()
      showNewMessageHint.value = false
      return
    }

    // 如果用户在底部附近，自动滚到底部
    if (isNearBottom()) {
      scrollToBottom()
      showNewMessageHint.value = false
    } else {
      // 用户不在底部，显示新消息提示
      showNewMessageHint.value = true
    }
  }
})

// 滚动事件处理
function handleScroll() {
  if (!scrollParent.value) return
  const el = scrollParent.value

  // 接近底部时隐藏新消息提示
  if (isNearBottom()) {
    showNewMessageHint.value = false
  }

  // 接近顶部时加载更早消息
  if (el.scrollTop < 300 && props.hasOlder && !props.loadingOlder) {
    emit('load-older')
  }
}

// 点击新消息提示
function handleClickNewMessage() {
  scrollToBottom()
  showNewMessageHint.value = false
}

function isNewDay(idx: number): boolean {
  if (idx === 0) return true
  const prev = new Date(props.messages[idx - 1].sent_at).toDateString()
  const curr = new Date(props.messages[idx].sent_at).toDateString()
  return prev !== curr
}

function messageKey(msg: ChatMessage, idx: number): string {
  if (msg.telegram_message_id) return `tg:${msg.telegram_message_id}`
  if (msg.local_id) return `local:${msg.local_id}`
  return `id:${msg.id}:${idx}`
}
</script>

<template>
  <div ref="scrollParent" class="message-scroll-container" @scroll="handleScroll">
    <!-- 加载更早消息提示 -->
    <div v-if="loadingOlder" class="older-loading">
      <div class="older-loading-spinner"></div>
      <span>加载历史消息...</span>
    </div>
    <div v-else-if="olderError" class="older-error">
      {{ olderError }}
    </div>
    <div v-else-if="!hasOlder && messages.length > 0" class="older-end">
      — 已经到最早消息 —
    </div>

    <div v-if="messages.length === 0" class="message-empty">
      暂无消息
    </div>
    <template v-for="(msg, idx) in messages" :key="messageKey(msg, idx)">
      <DateDivider v-if="isNewDay(idx)" :date="msg.sent_at" />
      <ServiceMessage v-if="msg.message_type === 'service'" :message="msg" />
      <MessageBubble v-else :message="msg" />
    </template>

    <!-- 新消息提示 -->
    <Transition name="fade">
      <button
        v-if="showNewMessageHint"
        class="new-message-hint"
        @click="handleClickNewMessage"
      >
        ↓ 有新消息
      </button>
    </Transition>
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

.older-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px;
  font-size: 13px;
  color: var(--text-secondary);
}

.older-loading-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.older-error {
  text-align: center;
  padding: 8px;
  font-size: 13px;
  color: var(--color-danger);
}

.older-end {
  text-align: center;
  padding: 12px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.new-message-hint {
  position: sticky;
  bottom: 16px;
  left: 50%;
  transform: translateX(-50%);
  display: block;
  margin: 0 auto;
  padding: 6px 16px;
  background: var(--accent-color);
  color: #fff;
  border: none;
  border-radius: 16px;
  font-size: 13px;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  z-index: 10;
  transition: opacity 0.2s;
}

.new-message-hint:hover {
  opacity: 0.9;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
