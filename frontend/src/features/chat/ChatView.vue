<script setup lang="ts">
import { computed, watch, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { fetchDialogs } from '@/api/chat'
import { fetchRuntimeStatus, startRuntime } from '@/api/runtime'
import { useChatStore } from '@/stores/chat'
import { useAccountStore } from '@/stores/account'
import { useI18n } from '@/i18n'
import { RealtimeClient } from '@/realtime/ws'
import { handleRealtimeEvent } from '@/realtime/handler'
import DialogList from './DialogList.vue'
import MessagePanel from './MessagePanel.vue'
import EmptyState from '@/components/EmptyState.vue'
import ErrorBanner from '@/components/ErrorBanner.vue'
import type { RuntimeState } from '@/types/runtime'
import type { WSState } from '@/realtime/ws'

const route = useRoute()
const router = useRouter()
const chat = useChatStore()
const account = useAccountStore()
const queryClient = useQueryClient()
const { t } = useI18n()

// Skeleton 超时提示：loading 超过 10 秒时显示提示
const slowLoading = ref(false)
let slowTimer: ReturnType<typeof setTimeout> | undefined

function startSlowTimer() {
  clearSlowTimer()
  slowLoading.value = false
  slowTimer = setTimeout(() => { slowLoading.value = true }, 10_000)
}
function clearSlowTimer() {
  if (slowTimer) { clearTimeout(slowTimer); slowTimer = undefined }
  slowLoading.value = false
}

const { data: dialogsData, isLoading, error, refetch } = useQuery({
  queryKey: computed(() => ['dialogs', account.currentAccountId]),
  queryFn: () => { startSlowTimer(); return fetchDialogs(30) },
  enabled: computed(() => !!account.currentAccountId),
  retry: 1,
  staleTime: 30_000,
  refetchOnWindowFocus: false,
})

// loading 结束时清理 timer
watch(isLoading, (loading) => {
  if (!loading) clearSlowTimer()
})

// Runtime status query
// refetchOnWindowFocus: true → 切回标签时立即检查状态（防止 stale live）
// refetchInterval: 30_000 → 缩短轮询间隔，更快发现服务断开
const { data: runtimeData, refetch: refetchRuntime, isError: runtimeFetchError } = useQuery({
  queryKey: computed(() => ['runtime-status', account.currentAccountId]),
  queryFn: fetchRuntimeStatus,
  enabled: computed(() => !!account.currentAccountId),
  retry: 1,
  refetchInterval: 30_000,
  refetchOnWindowFocus: true,
  refetchOnReconnect: true,
})

const runtimeState = computed(() => {
  // HTTP fetch 失败 → 服务可能不可达，不保留旧 live 状态
  if (runtimeFetchError.value) return 'offline' as RuntimeState
  return (runtimeData.value?.state || 'stopped') as RuntimeState
})

// Auto-start runtime when stopped
const startMutation = useMutation({
  mutationFn: startRuntime,
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['runtime-status', account.currentAccountId] })
  },
})

// ensureRuntimeStarted：统一的 runtime 自动恢复入口
// 带防抖，防止短时间内重复调用
let lastStartAttempt = 0
const START_DEBOUNCE_MS = 8_000

function ensureRuntimeStarted(_reason: string) {
  const id = account.currentAccountId
  if (!id) return
  if (startMutation.isPending.value) return // 已有 in-flight start

  // 只在 runtime 需要启动时调用
  const state = runtimeState.value
  if (state !== 'stopped' && state !== 'offline') return

  // 防抖：距离上次尝试超过 TTL
  const now = Date.now()
  if (now - lastStartAttempt < START_DEBOUNCE_MS) return
  lastStartAttempt = now

  startMutation.mutate()
}

// 1. 账号变化时启动
watch(() => account.currentAccountId, (id) => {
  if (id) {
    setTimeout(() => ensureRuntimeStarted('account_change'), 500)
  }
}, { immediate: true })

// 2. runtime 状态变化时自动恢复（后端重启后 status 返回 stopped/offline）
watch(runtimeState, (state) => {
  if (state === 'stopped' || state === 'offline') {
    ensureRuntimeStarted('state_change')
  }
})

// Use computed to reactively derive dialogs from query data
// 防御性去重：按 peer_ref 去重，保留最新条目（防止后端返回重复）
const dialogs = computed(() => {
  const raw = dialogsData.value?.dialogs || []
  const seen = new Map<string, typeof raw[0]>()
  for (const d of raw) {
    if (!d.peer_ref) continue // 跳过无 peer_ref 的幽灵记录
    const existing = seen.get(d.peer_ref)
    if (!existing) {
      seen.set(d.peer_ref, d)
    } else {
      // 保留 last_message_at 更新的条目
      const existingTime = existing.last_message_at || ''
      const newTime = d.last_message_at || ''
      if (newTime > existingTime) {
        seen.set(d.peer_ref, d)
      }
    }
  }
  return Array.from(seen.values())
})

// Find the currently selected dialog for title display
const selectedDialog = computed(() => {
  if (!chat.selectedPeerRef) return null
  return dialogs.value.find(d => d.peer_ref === chat.selectedPeerRef) || null
})

// Sync selectedPeerRef from route params
const routePeerRef = computed(() => route.params.peerRef as string | undefined)

watch(routePeerRef, (val) => {
  chat.selectPeer(val || null)
}, { immediate: true })

function selectDialog(ref: string) {
  chat.selectPeer(ref)
  router.push(`/chats/${ref}`)
}

// 强制刷新：跳过缓存，直接请求 Telegram
function forceRefresh() {
  startSlowTimer()
  ensureRuntimeStarted('force_refresh')
  // 用 force_refresh=true 重写 queryFn 触发后端跳过缓存
  queryClient.fetchQuery({
    queryKey: ['dialogs', account.currentAccountId],
    queryFn: () => fetchDialogs(30, true),
  })
  refetchRuntime()
}

const noAccount = computed(() => !account.currentAccountId)

// WebSocket 实时推送
const wsState = ref<WSState>('disconnected')
let wsClient: RealtimeClient | null = null

function connectWebSocket() {
  if (wsClient) wsClient.close()

  wsClient = new RealtimeClient({
    onEvent: (event) => {
      handleRealtimeEvent(event, queryClient, account.currentAccountId, chat.selectedPeerRef || null)
    },
    onStateChange: (state) => {
      wsState.value = state
    },
  })
  wsClient.connect()
}

function disconnectWebSocket() {
  if (wsClient) {
    wsClient.close()
    wsClient = null
  }
  wsState.value = 'disconnected'
}

// 连接 WebSocket：当 account 变为可用时
watch(() => account.currentAccountId, (id) => {
  if (id) {
    connectWebSocket()
  } else {
    disconnectWebSocket()
  }
}, { immediate: true })

// 断线重连后补状态 + 自动恢复 runtime
watch(wsState, (state, oldState) => {
  if (state === 'connected' && oldState === 'reconnecting') {
    // 重连成功，invalidate 查询以补偿断线期间丢失的事件
    queryClient.invalidateQueries({ queryKey: ['dialogs', account.currentAccountId] })
    if (chat.selectedPeerRef) {
      queryClient.invalidateQueries({ queryKey: ['messages', account.currentAccountId, chat.selectedPeerRef] })
    }
    queryClient.invalidateQueries({ queryKey: ['runtime-status', account.currentAccountId] })
    // WS 重连后自动恢复 runtime（延迟等待 status refetch 完成）
    setTimeout(() => ensureRuntimeStarted('ws_reconnect'), 1000)
  }
})

onUnmounted(() => {
  disconnectWebSocket()
  clearSlowTimer()
  document.removeEventListener('visibilitychange', onVisibilityChange)
  window.removeEventListener('online', onNetworkChange)
  window.removeEventListener('offline', onNetworkChange)
})

// 页面可见性变化时立即检查状态 + 恢复 runtime
function onVisibilityChange() {
  if (document.visibilityState === 'visible' && account.currentAccountId) {
    refetchRuntime()
    if (wsState.value === 'disconnected' || wsState.value === 'error') {
      connectWebSocket()
    }
    // 延迟等待 refetch 完成后 ensure
    setTimeout(() => ensureRuntimeStarted('visibility'), 1000)
  }
}

// 网络状态变化时立即检查 + 恢复 runtime
function onNetworkChange() {
  if (account.currentAccountId) {
    refetchRuntime()
    if (navigator.onLine && (wsState.value === 'disconnected' || wsState.value === 'error')) {
      connectWebSocket()
    }
    if (navigator.onLine) {
      setTimeout(() => ensureRuntimeStarted('network_online'), 1000)
    }
  }
}

document.addEventListener('visibilitychange', onVisibilityChange)
window.addEventListener('online', onNetworkChange)
window.addEventListener('offline', onNetworkChange)

// Runtime state display
// 综合 WebSocket 连接状态 + runtime 状态 + HTTP fetch 可达性
const runtimeLabel = computed(() => {
  // WebSocket 断开/重连中 → 不能显示"实时更新中"
  if (wsState.value === 'reconnecting') return t('chat.runtimeReconnecting')
  if (wsState.value === 'connecting') return t('chat.runtimeConnecting')
  if (wsState.value === 'disconnected' || wsState.value === 'error') {
    // WS 断开 + runtime fetch 也失败 → 服务不可达
    if (runtimeFetchError.value || runtimeState.value === 'offline') return t('chat.runtimeServiceDown')
    // WS 断开但 runtime 状态已知 → 连接断开
    return t('chat.runtimeDisconnected')
  }

  // runtime start 正在进行中 → 正在恢复
  if (startMutation.isPending.value) return t('chat.runtimeRestoring')

  // WebSocket 已连接 → 看 runtime 状态
  switch (runtimeState.value) {
    case 'connecting': return t('chat.runtimeConnecting')
    case 'syncing': return t('chat.runtimeSyncing')
    case 'live': return t('chat.runtimeLive')
    case 'degraded': return t('chat.runtimeDegraded')
    case 'offline': return t('chat.runtimeOffline')
    case 'stopped': return t('chat.runtimeStopped')
    default: return ''
  }
})

const runtimeTooltip = computed(() => {
  const parts: string[] = []
  const lastErr = runtimeData.value?.last_error as string | undefined
  if (lastErr) parts.push(lastErr)
  const execReady = runtimeData.value?.executor_ready as boolean | undefined
  if (execReady === false && runtimeState.value !== 'stopped') {
    parts.push(t('chat.executorNotReady'))
  }
  // WS 断开时附加提示
  if (wsState.value === 'reconnecting') parts.push(t('chat.wsReconnecting'))
  if (wsState.value === 'error') parts.push(t('chat.wsError'))
  return parts.join(' · ') || ''
})

// 同步图标样式：根据 runtime + WS 状态决定动画
const syncIconClass = computed(() => {
  if (wsState.value === 'reconnecting') return 'sync-reconnecting'
  if (wsState.value === 'connecting') return 'sync-connecting'
  if (wsState.value === 'disconnected' || wsState.value === 'error') {
    if (runtimeFetchError.value) return 'sync-error'
    return 'sync-stopped'
  }
  if (startMutation.isPending.value) return 'sync-connecting'
  switch (runtimeState.value) {
    case 'live': return 'sync-live'
    case 'connecting':
    case 'syncing': return 'sync-connecting'
    case 'degraded':
    case 'offline': return 'sync-error'
    default: return 'sync-stopped'
  }
})

const runtimeClass = computed(() => {
  // WebSocket 重连/连接中 → 黄色
  if (wsState.value === 'reconnecting' || wsState.value === 'connecting') return 'runtime-connecting'
  // WebSocket 断开/错误 → 红色或灰色
  if (wsState.value === 'disconnected' || wsState.value === 'error') {
    if (runtimeFetchError.value) return 'runtime-error'
    return 'runtime-stopped'
  }

  // runtime start 正在进行中 → 黄色
  if (startMutation.isPending.value) return 'runtime-connecting'

  // WebSocket 已连接 → 看 runtime 状态
  switch (runtimeState.value) {
    case 'live': return 'runtime-live'
    case 'connecting':
    case 'syncing': return 'runtime-connecting'
    case 'degraded':
    case 'offline': return 'runtime-error'
    default: return 'runtime-stopped'
  }
})
</script>

<template>
  <div class="chat-layout">
    <div class="chat-sidebar" :class="{ 'mobile-hidden': chat.selectedPeerRef }">
      <div class="chat-sidebar-header">
        <h2 class="chat-sidebar-title">{{ t('chat.title') }}</h2>
        <div class="chat-sidebar-actions">
          <span v-if="account.currentAccountId && runtimeLabel" :class="['runtime-badge', runtimeClass]" :title="runtimeTooltip || runtimeLabel">
            <span class="runtime-dot"></span>
            {{ runtimeLabel }}
          </span>
          <span :class="['sync-icon', syncIconClass]" :title="runtimeTooltip || runtimeLabel || t('chat.title')">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21.5 2v6h-6M2.5 22v-6h6M2 11.5a10 10 0 0 1 18.8-4.3M22 12.5a10 10 0 0 1-18.8 4.3"/>
            </svg>
          </span>
        </div>
      </div>

      <div v-if="noAccount" class="chat-sidebar-body">
        <EmptyState
          icon="🔑"
          :title="t('chat.noAccount')"
          :description="t('chat.noAccountDesc')"
        />
      </div>
      <div v-else-if="isLoading" class="chat-sidebar-body">
        <div class="skeleton-list">
          <div v-for="i in 8" :key="i" class="skeleton-item">
            <div class="skeleton-avatar"></div>
            <div class="skeleton-lines">
              <div class="skeleton-line long"></div>
              <div class="skeleton-line short"></div>
            </div>
          </div>
        </div>
        <div v-if="slowLoading" class="slow-hint">
          <span>{{ t('chat.staleHint') }} <button class="btn-link" @click="forceRefresh()">{{ t('common.refresh') }}</button></span>
        </div>
      </div>
      <div v-else-if="error" class="chat-sidebar-body">
        <ErrorBanner :message="(error as Error).message" @dismiss="refetch()" />
      </div>
      <div v-else class="chat-sidebar-body">
        <DialogList :dialogs="dialogs" :selected="chat.selectedPeerRef" @select="selectDialog" />
      </div>
    </div>

    <div class="chat-main" :class="{ 'mobile-hidden': !chat.selectedPeerRef }">
      <MessagePanel
        v-if="chat.selectedPeerRef && account.currentAccountId"
        :peer-ref="chat.selectedPeerRef"
        :account-id="account.currentAccountId"
        :dialog-title="selectedDialog?.title || ''"
        :peer-type="selectedDialog?.peer_type"
        :key="chat.selectedPeerRef"
      />
      <div v-else class="chat-main-empty">
        <EmptyState
          icon="💬"
          :title="t('chat.selectChat')"
          :description="t('chat.selectChatDesc')"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.chat-sidebar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.runtime-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 10px;
  white-space: nowrap;
}

.runtime-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;
}

.runtime-live {
  background: var(--color-success-light, rgba(16, 185, 129, 0.1));
  color: var(--color-success, #10b981);
}
.runtime-live .runtime-dot {
  background: var(--color-success, #10b981);
}

.runtime-connecting {
  background: transparent;
  color: var(--text-secondary, #888);
  opacity: 0.7;
}
.runtime-connecting .runtime-dot {
  background: var(--text-secondary, #888);
}

.runtime-error {
  background: var(--color-danger-light, rgba(239, 68, 68, 0.1));
  color: var(--color-danger, #ef4444);
}
.runtime-error .runtime-dot {
  background: var(--color-danger, #ef4444);
}

.runtime-stopped {
  background: var(--bg-tertiary, rgba(128, 128, 128, 0.1));
  color: var(--text-secondary, #888);
}
.runtime-stopped .runtime-dot {
  background: var(--text-secondary, #888);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.slow-hint {
  text-align: center;
  padding: 12px 16px;
  font-size: 12px;
  color: var(--text-secondary, #888);
}

.btn-link {
  background: none;
  border: none;
  color: var(--color-primary, #3b82f6);
  cursor: pointer;
  font-size: inherit;
  text-decoration: underline;
  padding: 0;
}
</style>
