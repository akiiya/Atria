<script setup lang="ts">
import type { ChatMessage } from '@/types/chat'
import { useI18n } from '@/i18n'

defineProps<{ message: ChatMessage }>()

const { t } = useI18n()

function formatSize(bytes: number | undefined): string {
  if (!bytes) return ''
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

function formatDuration(sec: number | undefined): string {
  if (!sec) return ''
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return m + ':' + String(s).padStart(2, '0')
}
</script>

<template>
  <div class="media-card">
    <div v-if="message.message_type === 'photo'" class="media-photo">
      <div class="media-placeholder">🖼️</div>
      <div v-if="message.caption" class="media-caption">{{ message.caption }}</div>
      <div class="media-note">{{ t('media.photoPlaceholder') }}</div>
    </div>

    <div v-else-if="message.message_type === 'document'" class="media-document">
      <div class="media-icon">📄</div>
      <div class="media-info">
        <div class="media-filename">{{ message.media?.file_name || t('media.unknownFile') }}</div>
        <div class="media-meta">
          {{ message.media?.mime_type || '' }}
          {{ message.media?.size ? ' · ' + formatSize(message.media.size) : '' }}
        </div>
      </div>
      <button class="btn-sm" disabled title="下载暂未实现">{{ t('media.download') }}</button>
    </div>

    <div v-else-if="message.message_type === 'sticker'" class="media-sticker">
      <div class="media-emoji">{{ message.media?.emoji || '🏷️' }}</div>
      <div class="media-note">{{ t('media.sticker') }}</div>
    </div>

    <div v-else-if="message.message_type === 'video'" class="media-video">
      <div class="media-placeholder">🎬</div>
      <div class="media-info">
        <span v-if="message.media?.duration">{{ formatDuration(message.media.duration) }}</span>
        <span v-if="message.media?.width"> · {{ message.media.width }}×{{ message.media.height }}</span>
      </div>
      <div v-if="message.caption" class="media-caption">{{ message.caption }}</div>
      <div class="media-note">{{ t('media.videoPlaceholder') }}</div>
    </div>

    <div v-else-if="message.message_type === 'voice'" class="media-voice">
      <div class="media-icon">🎤</div>
      <div class="media-info">
        <span v-if="message.media?.duration">{{ formatDuration(message.media.duration) }}</span>
        <span v-else>{{ t('media.voiceMessage') }}</span>
      </div>
      <div class="media-note">{{ t('media.voicePlaceholder') }}</div>
    </div>

    <div v-else-if="message.message_type === 'audio'" class="media-audio">
      <div class="media-icon">🎵</div>
      <div class="media-info">
        <div class="media-filename">{{ message.media?.file_name || t('media.audio') }}</div>
        <div class="media-meta">
          {{ message.media?.duration ? formatDuration(message.media.duration) : '' }}
          {{ message.media?.size ? ' · ' + formatSize(message.media.size) : '' }}
        </div>
      </div>
    </div>
  </div>
</template>
