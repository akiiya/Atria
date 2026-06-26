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
    <div class="dialog-avatar-wrap">
      <AvatarInitials :text="dialog.title || dialog.avatar_placeholder" />
      <span v-if="dialog.unread_count" class="dialog-unread-badge">{{ dialog.unread_count }}</span>
    </div>
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
      </div>
    </div>
  </div>
</template>

<style scoped>
.dialog-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  cursor: pointer;
  transition: background 0.15s;
  border-radius: 8px;
  margin: 0 8px;
}

.dialog-item:hover {
  background: var(--bg-secondary);
}

.dialog-item.selected {
  background: var(--accent-light, rgba(59, 130, 246, 0.1));
}

.dialog-avatar-wrap {
  position: relative;
  flex-shrink: 0;
}

.dialog-unread-badge {
  position: absolute;
  top: -4px;
  right: -4px;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 10px;
  background: var(--accent-color, #3b82f6);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  line-height: 18px;
  text-align: center;
  box-shadow: 0 0 0 2px var(--bg-primary, #fff);
  pointer-events: none;
}

.dialog-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.dialog-title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.dialog-title {
  font-weight: 600;
  font-size: 14px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.dialog-type-icon {
  font-size: 12px;
  margin-right: 2px;
}

.dialog-time {
  font-size: 12px;
  color: var(--text-tertiary);
  flex-shrink: 0;
}

.dialog-preview-row {
  display: flex;
  align-items: center;
  gap: 4px;
}

.dialog-preview {
  font-size: 13px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.dialog-type-tag {
  font-size: 10px;
  padding: 1px 4px;
  border-radius: 4px;
  background: var(--bg-tertiary, #f3f4f6);
  color: var(--text-tertiary);
  flex-shrink: 0;
}
</style>
