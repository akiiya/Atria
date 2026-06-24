<script setup lang="ts">
import { ref, watch, nextTick, onBeforeUnmount } from 'vue'
import type { ChatMessage, PeerType } from '@/types/chat'
import MessageBubble from './MessageBubble.vue'
import ServiceMessage from './ServiceMessage.vue'
import DateDivider from './DateDivider.vue'

const props = defineProps<{
  messages: ChatMessage[]
  hasOlder: boolean
  loadingOlder: boolean
  olderError: string | null
  peerType?: PeerType
  peerRef?: string
}>()

const emit = defineEmits<{ 'load-older': [] }>()

const scrollParent = ref<HTMLElement | null>(null)
const showNewMessageHint = ref(false)

// ── Scroll Intent 状态机 ──
// "stick-to-bottom": 切换会话/初始加载后，保持到底部直到布局稳定
// "preserve-position": older pagination 保持位置
// "manual": 用户手动控制
type ScrollIntent = 'stick-to-bottom' | 'preserve-position' | 'manual'
const scrollIntent = ref<ScrollIntent>('stick-to-bottom')

// 取消旧 peer 的异步滚动任务
let scrollTaskToken = 0
// ResizeObserver 用于 stick-to-bottom 补偿
let stickObserver: ResizeObserver | null = null
let stickTimeout: ReturnType<typeof setTimeout> | null = null

// ── Older Pagination Anchor（内部管理）──
// 用户上滑触发 load-older 时标记，消息变化后恢复滚动位置
const shouldPreserveOlderPosition = ref(false)
let olderScrollData: { scrollHeight: number; scrollTop: number } | null = null

// ── 切换会话时重置 ──
watch(() => props.peerRef, () => {
  scrollTaskToken++ // 取消旧任务
  scrollIntent.value = 'stick-to-bottom'
  showNewMessageHint.value = false
  stopStickObserver()
  shouldPreserveOlderPosition.value = false
  olderScrollData = null
})

// ── 检查是否接近底部 ──
function isNearBottom(): boolean {
  if (!scrollParent.value) return true
  const el = scrollParent.value
  return el.scrollHeight - el.scrollTop - el.clientHeight < 160
}

// ── 核心：可靠地滚动到底部 ──
// 使用递增 token 确保旧 peer 的任务失效
// 使用 ResizeObserver 补偿 scrollHeight 二次变化
function scheduleScrollToBottom(_reason: string, _peerRef?: string) {
  const token = ++scrollTaskToken
  const el = scrollParent.value
  if (!el) return

  // 标记程序滚动，避免 onScroll 误判为用户操作
  isProgrammaticScroll = true

  // nextTick → rAF → rAF → 设置 scrollTop
  // 双 rAF 确保浏览器完成布局和绘制
  nextTick().then(() => {
    if (scrollTaskToken !== token) return
    requestAnimationFrame(() => {
      if (scrollTaskToken !== token) return
      requestAnimationFrame(() => {
        if (scrollTaskToken !== token) return
        doScrollToBottom(token, _reason)
      })
    })
  })
}

function doScrollToBottom(token: number, _reason: string) {
  const el = scrollParent.value
  if (!el || scrollTaskToken !== token) return

  el.scrollTop = el.scrollHeight
  isProgrammaticScroll = false

  // 启动 stick-to-bottom observer：内容高度变化时继续保持底部
  if (scrollIntent.value === 'stick-to-bottom') {
    startStickObserver(token)
  }
}

// ── ResizeObserver：stick-to-bottom 补偿 ──
// 当消息列表高度变化（字体加载、emoji 渲染、reconcile merge）时，
// 如果仍在 stick-to-bottom 模式，自动保持底部
function startStickObserver(token: number) {
  stopStickObserver()
  const el = scrollParent.value
  if (!el) return

  stickObserver = new ResizeObserver(() => {
    if (scrollTaskToken !== token) { stopStickObserver(); return }
    if (scrollIntent.value !== 'stick-to-bottom') { stopStickObserver(); return }
    el.scrollTop = el.scrollHeight
  })
  stickObserver.observe(el)

  // 稳定后停止 observer（避免无限观察）
  stickTimeout = setTimeout(() => {
    if (scrollTaskToken === token) stopStickObserver()
  }, 2000)
}

function stopStickObserver() {
  if (stickObserver) { stickObserver.disconnect(); stickObserver = null }
  if (stickTimeout) { clearTimeout(stickTimeout); stickTimeout = null }
}

// ── 程序滚动标记 ──
let isProgrammaticScroll = false

// ── 滚动事件处理 ──
function handleScroll() {
  if (!scrollParent.value) return
  const el = scrollParent.value

  // 接近底部时隐藏新消息提示
  if (isNearBottom()) {
    showNewMessageHint.value = false
    // 用户滚回底部，恢复 stick-to-bottom
    if (scrollIntent.value === 'manual') {
      scrollIntent.value = 'stick-to-bottom'
    }
  }

  // 非程序滚动 + 离底超过阈值 → 用户在阅读历史
  if (!isProgrammaticScroll && !isNearBottom()) {
    scrollIntent.value = 'manual'
    stopStickObserver()
  }

  // 接近顶部时加载更早消息
  if (el.scrollTop < 300 && props.hasOlder && !props.loadingOlder) {
    // 记录滚动位置，供消息变化后恢复
    shouldPreserveOlderPosition.value = true
    olderScrollData = {
      scrollHeight: el.scrollHeight,
      scrollTop: el.scrollTop,
    }
    scrollIntent.value = 'preserve-position'
    emit('load-older')
  }
}

// ── 消息变化监听 ──
watch(() => props.messages.length, async (newLen, oldLen) => {
  await nextTick()
  if (!scrollParent.value) return

  // 首次加载（peer switch 后 isInitialLoad 已通过 peerRef watcher 重置）
  if (scrollIntent.value === 'stick-to-bottom' && newLen > 0 && (oldLen === undefined || oldLen === 0)) {
    scheduleScrollToBottom('initial-load', props.peerRef)
    return
  }

  // Older pagination：恢复滚动位置
  if (shouldPreserveOlderPosition.value && olderScrollData && oldLen !== undefined && newLen > oldLen) {
    shouldPreserveOlderPosition.value = false
    const { scrollHeight: oldH, scrollTop: oldTop } = olderScrollData
    olderScrollData = null
    requestAnimationFrame(() => {
      if (!scrollParent.value) return
      const newH = scrollParent.value.scrollHeight
      scrollParent.value.scrollTop = oldTop + (newH - oldH)
    })
    return
  }

  // 消息数增加（非首次）
  if (oldLen !== undefined && newLen > oldLen) {
    // outgoing 消息 → 滚到底
    const lastMsg = props.messages[props.messages.length - 1]
    if (lastMsg?.is_outgoing) {
      scrollIntent.value = 'stick-to-bottom'
      scheduleScrollToBottom('outgoing', props.peerRef)
      showNewMessageHint.value = false
      return
    }

    // stick-to-bottom 模式（reconcile/force_refresh 后）
    if (scrollIntent.value === 'stick-to-bottom') {
      scheduleScrollToBottom('stick-to-bottom', props.peerRef)
      return
    }

    // 用户在底部附近 → 自动滚底
    if (isNearBottom()) {
      scheduleScrollToBottom('near-bottom', props.peerRef)
      showNewMessageHint.value = false
    } else {
      // 用户在看历史 → 显示新消息提示
      showNewMessageHint.value = true
    }
  }
})

// ── 点击新消息提示 ──
function handleClickNewMessage() {
  scrollIntent.value = 'stick-to-bottom'
  scheduleScrollToBottom('manual-jump', props.peerRef)
  showNewMessageHint.value = false
}

// ── 日期分隔 ──
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

// ── 清理 ──
onBeforeUnmount(() => {
  scrollTaskToken++
  stopStickObserver()
})
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
    <!-- 消息列表锚定：margin-top: auto 使消息不足一屏时靠底部显示 -->
    <div v-if="messages.length > 0" class="message-list-anchor">
      <template v-for="(msg, idx) in messages" :key="messageKey(msg, idx)">
        <DateDivider v-if="isNewDay(idx)" :date="msg.sent_at" />
        <ServiceMessage v-if="msg.message_type === 'service'" :message="msg" />
        <MessageBubble v-else :message="msg" :peer-type="peerType" />
      </template>
    </div>

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
