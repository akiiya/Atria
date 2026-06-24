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
// column-reverse 方向映射：
//   scrollTop ≈ 0      → 视口显示最新消息（DOM 底部）= "near bottom"
//   scrollTop ≈ max    → 视口显示最旧消息（DOM 顶部）= "near top"
// "stick-to-bottom": 切换会话/初始加载后，保持在最新消息直到用户手动上滑
// "preserve-position": older pagination 保持位置
// "manual": 用户手动控制
type ScrollIntent = 'stick-to-bottom' | 'preserve-position' | 'manual'
const scrollIntent = ref<ScrollIntent>('stick-to-bottom')

// 取消旧 peer 的异步滚动任务
let scrollTaskToken = 0
// ResizeObserver 用于 stick-to-bottom 补偿
let stickObserver: ResizeObserver | null = null
let stickTimeout: ReturnType<typeof setTimeout> | null = null
// 初始加载标记：peer switch 后首次加载需要 scroll 到最新消息
let needsInitialScroll = false

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
  needsInitialScroll = true
})

// ── column-reverse 方向下的滚动位置检测 ──
// scrollTop ≈ 0 → 视口在最新消息（DOM 底部）
// scrollTop ≈ max → 视口在最旧消息（DOM 顶部）

function getMaxScrollTop(): number {
  if (!scrollParent.value) return 0
  return Math.max(0, scrollParent.value.scrollHeight - scrollParent.value.clientHeight)
}

/** 视口是否在最新消息附近（DOM 底部，scrollTop ≈ 0） */
function isNearBottom(): boolean {
  if (!scrollParent.value) return true
  return scrollParent.value.scrollTop < 160
}

/** 视口是否在最旧消息附近（DOM 顶部，scrollTop ≈ max） */
function isNearTop(): boolean {
  if (!scrollParent.value) return false
  const max = getMaxScrollTop()
  return max > 0 && scrollParent.value.scrollTop > max - 300
}

// ── 核心：column-reverse 下滚动到最新消息 ──
// scrollTop = 0 即可显示 DOM 底部（最新消息）
function scheduleScrollToBottom(_reason: string, _peerRef?: string) {
  const token = ++scrollTaskToken
  const el = scrollParent.value
  if (!el) return

  isProgrammaticScroll = true

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

  // column-reverse: scrollTop = 0 → 显示最新消息
  el.scrollTop = 0
  isProgrammaticScroll = false

  if (scrollIntent.value === 'stick-to-bottom') {
    startStickObserver(token)
  }
}

// ── ResizeObserver：stick-to-bottom 补偿 ──
function startStickObserver(token: number) {
  stopStickObserver()
  const el = scrollParent.value
  if (!el) return

  stickObserver = new ResizeObserver(() => {
    if (scrollTaskToken !== token) { stopStickObserver(); return }
    if (scrollIntent.value !== 'stick-to-bottom') { stopStickObserver(); return }
    // column-reverse: scrollTop = 0 → 最新消息
    el.scrollTop = 0
  })
  stickObserver.observe(el)

  // 稳定后停止 observer
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
// column-reverse 方向：
//   scrollTop ≈ 0 → 最新消息（底部）
//   scrollTop ≈ max → 最旧消息（顶部）
//   向上滑（看旧消息）→ scrollTop 增加
//   向下滑（看新消息）→ scrollTop 减少
function handleScroll() {
  if (!scrollParent.value) return
  const el = scrollParent.value
  const maxScroll = getMaxScrollTop()

  // 接近最新消息（scrollTop ≈ 0）→ 隐藏新消息提示
  if (isNearBottom()) {
    showNewMessageHint.value = false
    if (scrollIntent.value === 'manual') {
      scrollIntent.value = 'stick-to-bottom'
    }
  }

  // 非程序滚动 + 远离最新消息 → 用户在阅读历史
  if (!isProgrammaticScroll && !isNearBottom()) {
    scrollIntent.value = 'manual'
    stopStickObserver()
  }

  // 接近最旧消息（scrollTop ≈ max）→ 加载更早消息
  if (maxScroll > 0 && isNearTop() && props.hasOlder && !props.loadingOlder) {
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

  // 首次加载：peer switch 后第一条消息到达
  if (needsInitialScroll && newLen > 0) {
    needsInitialScroll = false
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
      // column-reverse: prepend 旧消息使 scrollHeight 增加，需要增加 scrollTop 保持位置
      scrollParent.value.scrollTop = oldTop + (newH - oldH)
    })
    return
  }

  // 消息数增加（非首次）
  if (oldLen !== undefined && newLen > oldLen) {
    // outgoing 消息 → 滚到最新
    const lastMsg = props.messages[props.messages.length - 1]
    if (lastMsg?.is_outgoing) {
      scrollIntent.value = 'stick-to-bottom'
      scheduleScrollToBottom('outgoing', props.peerRef)
      showNewMessageHint.value = false
      return
    }

    // stick-to-bottom 模式
    if (scrollIntent.value === 'stick-to-bottom') {
      scheduleScrollToBottom('stick-to-bottom', props.peerRef)
      return
    }

    // 用户在最新消息附近 → 自动滚到最新
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

// ── Wheel fallback：无滚动条时，用户上滑（deltaY<0）仍触发 loadOlder ──
// column-reverse: 向上滚动 = deltaY < 0 = 想看更旧消息
function handleWheel(e: WheelEvent) {
  if (e.deltaY < 0 && props.hasOlder && !props.loadingOlder) {
    const el = scrollParent.value
    if (!el) return
    const max = getMaxScrollTop()
    // 无滚动条（max=0）或已在最旧位置附近 → 触发加载
    if (max === 0 || el.scrollTop > max - 300) {
      // 记录滚动位置
      shouldPreserveOlderPosition.value = true
      olderScrollData = {
        scrollHeight: el.scrollHeight,
        scrollTop: el.scrollTop,
      }
      scrollIntent.value = 'preserve-position'
      emit('load-older')
    }
  }
}

// ── 清理 ──
onBeforeUnmount(() => {
  scrollTaskToken++
  stopStickObserver()
})
</script>

<template>
  <!--
    column-reverse 说明：
    - flex-direction: column-reverse 使 DOM 末尾（最新消息）显示在容器底部
    - scrollTop = 0 → 视口显示最新消息
    - scrollTop = max → 视口显示最旧消息
    - 新消息 append 到数组末尾 → 自动显示在视口底部
    - 旧消息 prepend 到数组头部 → 用户向上滚动可见
  -->
  <div ref="scrollParent" class="message-scroll-container" @scroll="handleScroll" @wheel="handleWheel">
    <div v-if="messages.length === 0" class="message-empty">
      暂无消息
    </div>

    <!-- 消息列表：反向迭代，使视觉顺序为旧→新（上→下） -->
    <template v-for="(msg, idx) in [...messages].reverse()" :key="messageKey(msg, messages.length - 1 - idx)">
      <DateDivider v-if="isNewDay(messages.length - 1 - idx)" :date="msg.sent_at" />
      <ServiceMessage v-if="msg.message_type === 'service'" :message="msg" />
      <MessageBubble v-else :message="msg" :peer-type="peerType" />
    </template>

    <!-- 加载更早消息提示（DOM 顶部 = column-reverse 视觉底部） -->
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
