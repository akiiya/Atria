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
</script>

<template>
  <div :class="['dialog-item', { selected }]" @click="$emit('click')">
    <AvatarInitials :text="dialog.title || dialog.avatar_placeholder" />
    <div class="dialog-info">
      <div class="dialog-title-row">
        <span class="dialog-title">{{ dialog.title }}</span>
        <span class="dialog-time">{{ formatTime(dialog.last_message_at) }}</span>
      </div>
      <div class="dialog-preview-row">
        <span class="dialog-preview">{{ dialog.last_message_preview || t('chat.noMessages') }}</span>
        <span v-if="dialog.unread_count" class="dialog-unread">{{ dialog.unread_count }}</span>
      </div>
    </div>
  </div>
</template>
