<script setup lang="ts">
import { computed, watch, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { fetchDialogs } from '@/api/chat'
import { fetchRuntimeStatus, startRuntime } from '@/api/runtime'
import { useChatStore } from '@/stores/chat'
import { useAccountStore } from '@/stores/account'
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
const { data: runtimeData, refetch: refetchRuntime } = useQuery({
  queryKey: computed(() => ['runtime-status', account.currentAccountId]),
  queryFn: fetchRuntimeStatus,
  enabled: computed(() => !!account.currentAccountId),
  retry: 1,
  refetchInterval: 60_000, // 每 60 秒刷新一次
  refetchOnWindowFocus: false,
})

const runtimeState = computed(() => (runtimeData.value?.state || 'stopped') as RuntimeState)

// Auto-start runtime when stopped
const startMutation = useMutation({
  mutationFn: startRuntime,
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['runtime-status', account.currentAccountId] })
  },
})

// Watch for account changes and check runtime
watch(() => account.currentAccountId, (id) => {
  if (id) {
    // 延迟检查 runtime 状态，等待 status query 完成
    setTimeout(() => {
      if (runtimeState.value === 'stopped' && !startMutation.isPending.value) {
        startMutation.mutate()
      }
    }, 500)
  }
}, { immediate: true })

// Use computed to reactively derive dialogs from query data
const dialogs = computed(() => dialogsData.value?.dialogs || [])

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

// 断线重连后补状态
watch(wsState, (state, oldState) => {
  if (state === 'connected' && oldState === 'reconnecting') {
    // 重连成功，invalidate 查询以补偿断线期间丢失的事件
    queryClient.invalidateQueries({ queryKey: ['dialogs', account.currentAccountId] })
    if (chat.selectedPeerRef) {
      queryClient.invalidateQueries({ queryKey: ['messages', account.currentAccountId, chat.selectedPeerRef] })
    }
    queryClient.invalidateQueries({ queryKey: ['runtime-status', account.currentAccountId] })
  }
})

onUnmounted(() => {
  disconnectWebSocket()
  clearSlowTimer()
})

// Runtime state display
const runtimeLabel = computed(() => {
  switch (runtimeState.value) {
    case 'connecting': return '正在连接'
    case 'syncing': return '正在同步'
    case 'live': return '实时更新中'
    case 'degraded': return '同步异常'
    case 'offline': return '连接断开'
    case 'stopped': return '未启动'
    default: return ''
  }
})

const runtimeTooltip = computed(() => {
  const parts: string[] = []
  const lastErr = runtimeData.value?.last_error as string | undefined
  if (lastErr) parts.push(lastErr)
  const execReady = runtimeData.value?.executor_ready as boolean | undefined
  if (execReady === false && runtimeState.value !== 'stopped') {
    parts.push('执行器未就绪')
  }
  return parts.join(' · ') || ''
})

const runtimeClass = computed(() => {
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
        <h2 class="chat-sidebar-title">会话</h2>
        <div class="chat-sidebar-actions">
          <span v-if="account.currentAccountId && runtimeLabel" :class="['runtime-badge', runtimeClass]" :title="runtimeTooltip || runtimeLabel">
            <span class="runtime-dot"></span>
            {{ runtimeLabel }}
          </span>
          <button class="btn-icon" @click="forceRefresh()" title="刷新">↻</button>
        </div>
      </div>

      <div v-if="noAccount" class="chat-sidebar-body">
        <EmptyState
          icon="🔑"
          title="请先接入 Telegram 账号"
          description="聊天功能需要先接入一个 Telegram 账号。"
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
          <span>加载时间较长，请检查网络或 <button class="btn-link" @click="forceRefresh()">强制刷新</button></span>
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
        :key="chat.selectedPeerRef"
      />
      <div v-else class="chat-main-empty">
        <EmptyState
          icon="💬"
          title="选择一个会话"
          description="从左侧列表中选择一个会话开始聊天。"
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
