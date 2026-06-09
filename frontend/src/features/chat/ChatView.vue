<script setup lang="ts">
import { watch } from 'vue'
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
})

const dialogs = dialogsData.value?.dialogs || []

const peerRef = route.params.peerRef as string | undefined
if (peerRef) {
  chat.selectPeer(peerRef)
}

watch(() => route.params.peerRef, (val) => {
  if (val) chat.selectPeer(val as string)
  else chat.selectPeer(null)
})

function selectDialog(ref: string) {
  chat.selectPeer(ref)
  router.push(`/chats/${ref}`)
}

const noAccount = !account.currentAccountId
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
        <LoadingSkeleton />
      </div>
      <div v-else-if="error" class="chat-sidebar-body">
        <ErrorBanner :message="(error as Error).message" @dismiss="refetch()" />
      </div>
      <div v-else class="chat-sidebar-body">
        <DialogList :dialogs="dialogs" :selected="chat.selectedPeerRef" @select="selectDialog" />
      </div>
    </div>

    <div class="chat-main" :class="{ 'mobile-hidden': !chat.selectedPeerRef }">
      <MessagePanel v-if="chat.selectedPeerRef" :peer-ref="chat.selectedPeerRef" />
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
