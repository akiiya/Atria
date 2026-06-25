<script setup lang="ts">
import { ref } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { sendMessage } from '@/api/chat'
import {
  markLocalMessageFailedInMessagesCache,
  replaceLocalMessageInMessagesCache,
  upsertMessageInMessagesCache,
} from '@/realtime/handler'
import { useI18n } from '@/i18n'
import type { ChatMessage, SendMessageResponse } from '@/types/chat'

const { t } = useI18n()
const props = defineProps<{ peerRef: string; accountId: number }>()
const emit = defineEmits<{ sent: [] }>()

const text = ref('')
const error = ref('')
const queryClient = useQueryClient()

const sendMutation = useMutation({
  mutationFn: (vars: { text: string; localId: string }) =>
    sendMessage(props.peerRef, vars.text, vars.localId),
  onMutate: (vars) => {
    const optimistic: ChatMessage = {
      id: negativeLocalID(vars.localId),
      local_id: vars.localId,
      client_pending_id: vars.localId,
      pending: true,
      peer_ref: props.peerRef,
      direction: 'out',
      sender_name: '',
      text: vars.text,
      sent_at: new Date().toISOString(),
      is_outgoing: true,
      status: 'sending',
      message_type: 'text',
    }
    upsertMessageInMessagesCache(queryClient, props.accountId, props.peerRef, optimistic)
    return vars
  },
  onSuccess: (data: SendMessageResponse, vars) => {
    if (data.ok) {
      if (data.message) {
        const telegramMessageId = data.message.telegram_message_id ?? data.message.id
        replaceLocalMessageInMessagesCache(queryClient, props.accountId, props.peerRef, vars.localId, {
          id: telegramMessageId,
          telegram_message_id: telegramMessageId,
          local_id: vars.localId,
          client_pending_id: vars.localId,
          pending: false,
          peer_ref: props.peerRef,
          direction: data.message.direction === 'in' ? 'in' : 'out',
          sender_name: '',
          text: data.message.text || vars.text,
          sent_at: data.message.sent_at || new Date().toISOString(),
          is_outgoing: true,
          status: data.message.status === 'failed' ? 'failed' : 'sent',
          message_type: 'text',
        })
      }
      text.value = ''
      error.value = ''
      queryClient.invalidateQueries({ queryKey: ['dialogs', props.accountId] })
      emit('sent')
    } else {
      error.value = data.error || (typeof data.message === 'string' ? data.message : '') || t('chat.sendFailed')
      markLocalMessageFailedInMessagesCache(queryClient, props.accountId, props.peerRef, vars.localId, error.value)
    }
  },
  onError: (err: Error, vars) => {
    error.value = err.message || t('chat.networkError')
    markLocalMessageFailedInMessagesCache(queryClient, props.accountId, props.peerRef, vars.localId, error.value)
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
    error.value = t('chat.messageTooLong')
    return
  }
  error.value = ''
  sendMutation.mutate({ text: trimmed, localId: createLocalID() })
}

function createLocalID(): string {
  return `local_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`
}

function negativeLocalID(seed: string): number {
  let hash = 0
  for (let i = 0; i < seed.length; i++) {
    hash = (hash * 31 + seed.charCodeAt(i)) | 0
  }
  return -Math.abs(hash || Date.now())
}
</script>

<template>
  <div class="message-composer">
    <div v-if="error" class="composer-error">{{ error }}</div>
    <div class="composer-row">
      <textarea
        v-model="text"
        class="composer-input"
        :placeholder="t('chat.inputPlaceholder')"
        rows="1"
        maxlength="4096"
        @keydown="handleKeydown"
      />
      <button
        class="btn btn-primary composer-send"
        :disabled="!text.trim() || sendMutation.isPending.value"
        @click="send"
      >
        {{ sendMutation.isPending.value ? t('chat.sending') : t('chat.send') }}
      </button>
    </div>
  </div>
</template>
