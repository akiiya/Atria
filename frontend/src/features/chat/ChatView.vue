<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuery } from '@tanstack/vue-query'
import { fetchDialogs } from '@/api/chat'
import { useChatStore } from '@/stores/chat'
import { useAccountStore } from '@/stores/account'
import DialogList from './DialogList.vue'
import MessagePanel from './MessagePanel.vue'
import EmptyState from '@/components/EmptyState.vue'
import ErrorBanner from '@/components/ErrorBanner.vue'

const route = useRoute()
const router = useRouter()
const chat = useChatStore()
const account = useAccountStore()

const { data: dialogsData, isLoading, error, refetch } = useQuery({
  queryKey: ['dialogs'],
  queryFn: () => fetchDialogs(30),
  retry: 1,
  staleTime: 30_000,
})

// Use computed to reactively derive dialogs from query data
const dialogs = computed(() => dialogsData.value?.dialogs || [])

// Sync selectedPeerRef from route params
const routePeerRef = computed(() => route.params.peerRef as string | undefined)

watch(routePeerRef, (val) => {
  chat.selectPeer(val || null)
}, { immediate: true })

function selectDialog(ref: string) {
  chat.selectPeer(ref)
  router.push(`/chats/${ref}`)
}

const noAccount = computed(() => !account.currentAccountId)
</script>

<template>
  <div class="chat-layout">
    <div class="chat-sidebar" :class="{ 'mobile-hidden': chat.selectedPeerRef }">
      <div class="chat-sidebar-header">
        <h2 class="chat-sidebar-title">会话</h2>
        <button class="btn-icon" @click="refetch()" title="刷新">↻</button>
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
      </div>
      <div v-else-if="error" class="chat-sidebar-body">
        <ErrorBanner :message="(error as Error).message" @dismiss="refetch()" />
      </div>
      <div v-else class="chat-sidebar-body">
        <DialogList :dialogs="dialogs" :selected="chat.selectedPeerRef" @select="selectDialog" />
      </div>
    </div>

    <div class="chat-main" :class="{ 'mobile-hidden': !chat.selectedPeerRef }">
      <MessagePanel v-if="chat.selectedPeerRef" :peer-ref="chat.selectedPeerRef" :key="chat.selectedPeerRef" />
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
