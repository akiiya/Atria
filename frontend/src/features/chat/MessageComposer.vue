<script setup lang="ts">
import { ref } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { sendMessage } from '@/api/chat'

const props = defineProps<{ peerRef: string }>()
const emit = defineEmits<{ sent: [] }>()

const text = ref('')
const error = ref('')
const queryClient = useQueryClient()

const sendMutation = useMutation({
  mutationFn: () => sendMessage(props.peerRef, text.value.trim()),
  onSuccess: (data: { ok: boolean; error?: string; message?: unknown }) => {
    if (data.ok) {
      text.value = ''
      error.value = ''
      queryClient.invalidateQueries({ queryKey: ['messages', props.peerRef] })
      queryClient.invalidateQueries({ queryKey: ['dialogs'] })
      emit('sent')
    } else {
      error.value = data.error || (typeof data.message === 'string' ? data.message : '') || '发送失败'
    }
  },
  onError: (err: Error) => {
    error.value = err.message || '网络请求失败'
  },
})

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

function send() {
  const trimmed = text.value.trim()
  if (!trimmed || sendMutation.isPending.value) return
  if (trimmed.length > 4096) {
    error.value = '消息内容不能超过 4096 个字符'
    return
  }
  error.value = ''
  sendMutation.mutate()
}
</script>

<template>
  <div class="message-composer">
    <div v-if="error" class="composer-error">{{ error }}</div>
    <div class="composer-row">
      <textarea
        v-model="text"
        class="composer-input"
        placeholder="输入消息... (Enter 发送, Shift+Enter 换行)"
        rows="1"
        maxlength="4096"
        @keydown="handleKeydown"
      />
      <button
        class="btn btn-primary composer-send"
        :disabled="!text.trim() || sendMutation.isPending.value"
        @click="send"
      >
        {{ sendMutation.isPending.value ? '发送中...' : '发送' }}
      </button>
    </div>
  </div>
</template>
