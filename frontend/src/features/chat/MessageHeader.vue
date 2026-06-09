<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useChatStore } from '@/stores/chat'

defineProps<{ peerRef: string }>()
defineEmits<{ refresh: [] }>()

const router = useRouter()
const chat = useChatStore()

function goBack() {
  chat.selectPeer(null)
  router.push('/chats')
}
</script>

<template>
  <div class="message-header">
    <button class="btn-back mobile-only" @click="goBack">←</button>
    <div class="message-header-info">
      <span class="message-header-title">{{ peerRef }}</span>
    </div>
    <button class="btn-icon" @click="$emit('refresh')" title="刷新">↻</button>
  </div>
</template>
