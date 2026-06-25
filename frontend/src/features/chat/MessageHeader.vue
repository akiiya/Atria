<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useChatStore } from '@/stores/chat'
import { useQueryClient } from '@tanstack/vue-query'
import { fetchMessages } from '@/api/chat'
import { useI18n } from '@/i18n'

const props = defineProps<{
  peerRef: string
  title?: string
  accountId?: number
  /** 消息正在加载/刷新中 */
  syncing?: boolean
  /** 数据可能过期（stale cache） */
  stale?: boolean
  /** peer 类型 */
  peerType?: string
}>()
const emit = defineEmits<{ refresh: [] }>()

const router = useRouter()
const chat = useChatStore()
const queryClient = useQueryClient()
const { t } = useI18n()

function goBack() {
  chat.selectPeer(null)
  router.push('/chats')
}

function handleClick() {
  // 点击仍可手动刷新（作为 fallback）
  if (!props.accountId || !props.peerRef) return
  queryClient.fetchQuery({
    queryKey: ['messages', props.accountId, props.peerRef],
    queryFn: () => fetchMessages(props.peerRef, 50, undefined, true),
  })
  emit('refresh')
}

function peerTypeLabel(type: string | undefined): string {
  switch (type) {
    case 'user': return ''
    case 'bot': return t('peerType.bot')
    case 'chat': return t('peerType.group')
    case 'supergroup': return t('peerType.supergroup')
    case 'channel': return t('peerType.channel')
    default: return ''
  }
}
</script>

<template>
  <div class="message-header">
    <button class="btn-back mobile-only" @click="goBack">&larr;</button>
    <div class="message-header-info">
      <span class="message-header-title">{{ title || peerRef }}</span>
      <span v-if="peerTypeLabel(peerType)" class="message-header-type">{{ peerTypeLabel(peerType) }}</span>
    </div>
    <span
      :class="['sync-icon', syncing ? 'sync-loading' : stale ? 'sync-connecting' : 'sync-idle']"
      :title="syncing ? t('chat.syncing') : stale ? t('chat.dataStale') : t('chat.synced')"
      @click="handleClick"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M21.5 2v6h-6M2.5 22v-6h6M2 11.5a10 10 0 0 1 18.8-4.3M22 12.5a10 10 0 0 1-18.8 4.3"/>
      </svg>
    </span>
  </div>
</template>
