<script setup lang="ts">
import { ref, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import type { ChatMessage, PeerType } from '@/types/chat'
import { useI18n } from '@/i18n'
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

const emit = defineEmits<{ 'load-older': []; 'scroll-to-bottom': []; reply: [message: ChatMessage] }>()

const { t } = useI18n()

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
// 用户上滑触发 load-older 时标记，消息变化后跳过自动滚动
const shouldPreserveOlderPosition = ref(false)

// ── 切换会话时重置 ──
watch(() => props.peerRef, () => {
  scrollTaskToken++ // 取消旧任务
  scrollIntent.value = 'stick-to-bottom'
  showNewMessageHint.value = false
  stopStickObserver()
  shouldPreserveOlderPosition.value = false
  needsInitialScroll = true
  // 新会话视为「尚未在底部」，这样首次滚到底时会发出一次 scroll-to-bottom
  wasNearBottom = false
})

// ── column-reverse 方向下的滚动位置检测 ──
// scrollTop ≈ 0 → 视口在最新消息（DOM 底部）
// scrollTop ≈ max → 视口在最旧消息（DOM 顶部）

// column-reverse scrollTop 实测值：
//   scrollTop = 0     → 最新消息（视觉底部）
//   scrollTop < 0     → 向旧消息方向滚动
//   scrollTop = -max  → 最旧消息（视觉顶部）
//   max = scrollHeight - clientHeight（正数）

function getMaxScrollTop(): number {
  if (!scrollParent.value) return 0
  return Math.max(0, scrollParent.value.scrollHeight - scrollParent.value.clientHeight)
}

/** 视口是否在最新消息附近（scrollTop ≈ 0） */
function isNearBottom(): boolean {
  if (!scrollParent.value) return true
  return scrollParent.value.scrollTop > -160
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

  // column-reverse: scrollTop = 0 → 显示最新消息（视觉底部）
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
//
// 通过 rAF 合并到每帧一次，避免每个滚动事件都读取 scrollHeight/clientHeight
// 触发强制重排。同时只在「离开底部后重新回到底部」时才发一次 scroll-to-bottom，
// 而不是停在底部期间每帧都发。
let scrollRafId: number | null = null
let wasNearBottom = true

function handleScroll() {
  if (scrollRafId !== null) return
  scrollRafId = requestAnimationFrame(() => {
    scrollRafId = null
    processScroll()
  })
}

function processScroll() {
  if (!scrollParent.value) return

  // 每帧只读一次布局属性
  const el = scrollParent.value
  const scrollTop = el.scrollTop
  const maxScroll = Math.max(0, el.scrollHeight - el.clientHeight)
  const nearBottom = scrollTop > -160
  const nearTop = maxScroll > 0 && scrollTop < -(maxScroll - 300)

  if (nearBottom) {
    showNewMessageHint.value = false
    if (scrollIntent.value === 'manual') {
      scrollIntent.value = 'stick-to-bottom'
    }
    // 边沿触发：仅在从「非底部」进入「底部」时通知一次
    if (!wasNearBottom) emit('scroll-to-bottom')
  } else if (!isProgrammaticScroll) {
    // 非程序滚动 + 远离最新消息 → 用户在阅读历史
    scrollIntent.value = 'manual'
    stopStickObserver()
  }
  wasNearBottom = nearBottom

  // 接近最旧消息（scrollTop ≈ -max）→ 加载更早消息
  if (nearTop && props.hasOlder && !props.loadingOlder) {
    shouldPreserveOlderPosition.value = true
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

  // Older pagination：加载完成后
  // column-reverse 下浏览器自动处理 scroll anchoring，不需要手动恢复
  if (shouldPreserveOlderPosition.value && oldLen !== undefined && newLen > oldLen) {
    shouldPreserveOlderPosition.value = false
    // 不调整 scrollTop，让浏览器保持用户的视觉位置
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

function messageKey(msg: ChatMessage, idx: number): string {
  if (msg.telegram_message_id) return `tg:${msg.telegram_message_id}`
  if (msg.local_id) return `local:${msg.local_id}`
  return `id:${msg.id}:${idx}`
}

// ── 渲染行预计算 ──
// 模板此前每次渲染都执行 [...messages].reverse()（整数组重新分配），
// 并对每条消息调用 isNewDay()（每条构造两个 Date）。改为在消息变化时算一次。
//
// column-reverse 布局要求 DOM 逆序：数组头部 = 视觉底部（最新消息）。
interface RenderRow {
  key: string
  msg: ChatMessage
  isService: boolean
  showDate: boolean
  /** 是否为分组中的首条消息（需要显示头像和发送者名称） */
  isGroupFirst: boolean
  /** 是否为分组中的末条消息（需要显示时间） */
  isGroupLast: boolean
}

/** 判断两条消息是否属于同一分组（同一发送者、5 分钟内、非服务消息） */
function isSameGroup(a: ChatMessage, b: ChatMessage): boolean {
  if (a.message_type === 'service' || b.message_type === 'service') return false
  if (a.is_outgoing !== b.is_outgoing) return false
  if (!a.is_outgoing && a.sender_name !== b.sender_name) return false
  const timeDiff = Math.abs(new Date(a.sent_at).getTime() - new Date(b.sent_at).getTime())
  return timeDiff < 5 * 60 * 1000 // 5 分钟内
}

const renderRows = computed<RenderRow[]>(() => {
  const list = props.messages
  const rows: RenderRow[] = new Array(list.length)

  // 正序遍历判断日期分隔和分组，同时逆序写入结果
  let prevDay = ''
  let prevMsg: ChatMessage | null = null
  for (let i = 0; i < list.length; i++) {
    const msg = list[i]
    const day = new Date(msg.sent_at).toDateString()
    const showDate = i === 0 || day !== prevDay

    // 分组判断：同一天、同一发送者、5 分钟内
    const inGroup = !showDate && prevMsg && isSameGroup(prevMsg, msg)

    // 判断下一条消息是否同组（用于判断是否为末条）
    const nextMsg = i + 1 < list.length ? list[i + 1] : null
    const nextInGroup = nextMsg && !showDate && isSameGroup(msg, nextMsg) &&
      new Date(msg.sent_at).toDateString() === new Date(nextMsg.sent_at).toDateString()

    rows[list.length - 1 - i] = {
      key: messageKey(msg, i),
      msg,
      isService: msg.message_type === 'service',
      showDate,
      isGroupFirst: !inGroup,
      isGroupLast: !nextInGroup,
    }
    prevDay = day
    prevMsg = msg
  }
  return rows
})

// ── Wheel fallback：无滚动条时，用户上滑（deltaY<0）仍触发 loadOlder ──
// column-reverse: 向上滚动 = deltaY < 0 = 想看更旧消息
function handleWheel(e: WheelEvent) {
  const el = scrollParent.value
  if (!el) return
  const max = getMaxScrollTop()
  // column-reverse: deltaY < 0 = 向旧消息方向滚动（scrollTop 变得更负）
  // 触发条件：无滚动条（max=0）或已在最旧位置附近（scrollTop 接近 -max）
  if (e.deltaY < 0 && props.hasOlder && !props.loadingOlder) {
    if (max === 0 || el.scrollTop < -(max - 300)) {
      shouldPreserveOlderPosition.value = true
      scrollIntent.value = 'preserve-position'
      emit('load-older')
    }
  }
}

// ── 清理 ──
onBeforeUnmount(() => {
  scrollTaskToken++
  stopStickObserver()
  if (scrollRafId !== null) {
    cancelAnimationFrame(scrollRafId)
    scrollRafId = null
  }
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
      {{ t('chat.noMessages') }}
    </div>

    <!-- 消息列表：renderRows 已按 column-reverse 所需的逆序预计算 -->
    <template v-for="row in renderRows" :key="row.key">
      <DateDivider v-if="row.showDate" :date="row.msg.sent_at" />
      <ServiceMessage v-if="row.isService" :message="row.msg" />
      <MessageBubble
        v-else
        :message="row.msg"
        :peer-type="peerType"
        :group-first="row.isGroupFirst ? 1 : 0"
        :group-last="row.isGroupLast ? 1 : 0"
        @reply="emit('reply', $event)"
      />
    </template>

    <!-- 加载更早消息提示（DOM 顶部 = column-reverse 视觉底部） -->
    <div v-if="loadingOlder" class="older-loading">
      <div class="older-loading-spinner"></div>
      <span>{{ t('chat.loadingHistory') }}</span>
    </div>
    <div v-else-if="olderError" class="older-error">
      {{ olderError }}
    </div>
    <div v-else-if="!hasOlder && messages.length > 0" class="older-end">
      — {{ t('chat.reachedOldest') }} —
    </div>

    <!-- 新消息提示 -->
    <Transition name="fade">
      <button
        v-if="showNewMessageHint"
        class="new-message-hint"
        @click="handleClickNewMessage"
      >
        ↓ {{ t('chat.newMessages') }}
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
