<script setup lang="ts">
import type { Dialog } from '@/types/chat'
import AvatarInitials from '@/components/AvatarInitials.vue'
import { useI18n } from '@/i18n'

defineProps<{
  dialog: Dialog
  selected: boolean
}>()

const { t } = useI18n()

function formatTime(iso: string | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  const now = new Date()
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString([], { month: '2-digit', day: '2-digit' })
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

function peerTypeIcon(type: string | undefined): string {
  switch (type) {
    case 'bot': return '\u{1F916}'
    case 'chat': return '\u{1F465}'
    case 'supergroup': return '\u{1F465}'
    case 'channel': return '\u{1F4E2}'
    default: return ''
  }
}
</script>

<template>
  <div :class="['dialog-item', { selected }]" @click="$emit('click')">
    <AvatarInitials :text="dialog.title || dialog.avatar_placeholder" />
    <div class="dialog-info">
      <div class="dialog-title-row">
        <span class="dialog-title">
          <span v-if="peerTypeIcon(dialog.peer_type)" class="dialog-type-icon">{{ peerTypeIcon(dialog.peer_type) }}</span>
          {{ dialog.title }}
        </span>
        <span class="dialog-time">{{ formatTime(dialog.last_message_at) }}</span>
      </div>
      <div class="dialog-preview-row">
        <span class="dialog-preview">
          <span v-if="peerTypeLabel(dialog.peer_type)" class="dialog-type-tag">{{ peerTypeLabel(dialog.peer_type) }}</span>
          {{ dialog.last_message_preview || t('chat.noMessages') }}
        </span>
        <span v-if="dialog.unread_count" class="dialog-unread">{{ dialog.unread_count }}</span>
      </div>
    </div>
  </div>
</template>
