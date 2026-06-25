<script setup lang="ts">
import { ref } from 'vue'
import type { ChatMessage } from '@/types/chat'
import { useI18n } from '@/i18n'
import { downloadMedia, getMediaContentUrl } from '@/api/media'

const props = defineProps<{ message: ChatMessage }>()
const { t } = useI18n()

const mediaStatus = ref<string>('none') // none / cached / downloading / failed
const mediaError = ref<string>('')
const contentUrl = ref<string>('')

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

async function handleDownload() {
  if (mediaStatus.value === 'downloading') return

  mediaStatus.value = 'downloading'
  mediaError.value = ''

  try {
    const messageId = props.message.telegram_message_id || props.message.id
    const result = await downloadMedia(messageId, props.message.peer_ref)
    if (result.ok) {
      mediaStatus.value = 'cached'
      contentUrl.value = getMediaContentUrl(messageId, props.message.peer_ref)
    } else {
      mediaStatus.value = 'failed'
      mediaError.value = result.message || t('media.downloadFailed')
    }
  } catch (e: unknown) {
    mediaStatus.value = 'failed'
    mediaError.value = e instanceof Error ? e.message : t('media.downloadFailed')
  }
}

function openContent() {
  if (contentUrl.value) {
    window.open(contentUrl.value, '_blank')
  }
}
</script>

<template>
  <div class="media-card">
    <!-- Photo -->
    <div v-if="message.message_type === 'photo'" class="media-photo">
      <div v-if="mediaStatus === 'cached' && contentUrl" class="media-preview" @click="openContent">
        <img :src="contentUrl" :alt="message.caption || t('media.photo')" class="media-img" />
      </div>
      <div v-else class="media-placeholder" @click="handleDownload">
        <span class="media-icon-large">🖼️</span>
        <div v-if="message.media?.width" class="media-meta">{{ message.media.width }}×{{ message.media.height }}</div>
      </div>
      <div v-if="message.caption" class="media-caption">{{ message.caption }}</div>
      <div v-if="mediaStatus === 'none'" class="media-action">
        <button class="btn btn-sm btn-outline" @click="handleDownload">{{ t('media.download') }}</button>
      </div>
      <div v-else-if="mediaStatus === 'downloading'" class="media-action">
        <span class="media-loading">{{ t('media.downloading') }}</span>
      </div>
      <div v-else-if="mediaStatus === 'cached'" class="media-action">
        <button class="btn btn-sm btn-outline" @click="openContent">{{ t('media.view') }}</button>
      </div>
      <div v-else-if="mediaStatus === 'failed'" class="media-error">
        {{ mediaError || t('media.downloadFailed') }}
      </div>
    </div>

    <!-- Document -->
    <div v-else-if="message.message_type === 'document'" class="media-document">
      <div class="media-icon">📄</div>
      <div class="media-info">
        <div class="media-filename">{{ message.media?.file_name || t('media.unknownFile') }}</div>
        <div class="media-meta">
          {{ message.media?.mime_type || '' }}
          {{ message.media?.size ? ' · ' + formatSize(message.media.size) : '' }}
        </div>
      </div>
      <div v-if="mediaStatus === 'none'">
        <button class="btn btn-sm btn-outline" @click="handleDownload">{{ t('media.download') }}</button>
      </div>
      <div v-else-if="mediaStatus === 'downloading'">
        <span class="media-loading">{{ t('media.downloading') }}</span>
      </div>
      <div v-else-if="mediaStatus === 'cached'">
        <button class="btn btn-sm btn-primary" @click="openContent">{{ t('media.open') }}</button>
      </div>
      <div v-else-if="mediaStatus === 'failed'" class="media-error">
        {{ mediaError || t('media.downloadFailed') }}
      </div>
    </div>

    <!-- Sticker -->
    <div v-else-if="message.message_type === 'sticker'" class="media-sticker">
      <div class="media-emoji">{{ message.media?.emoji || '🏷️' }}</div>
      <div class="media-note">{{ t('media.sticker') }}</div>
      <div v-if="mediaStatus === 'none'" class="media-action">
        <button class="btn btn-sm btn-outline" @click="handleDownload">{{ t('media.download') }}</button>
      </div>
      <div v-else-if="mediaStatus === 'downloading'" class="media-action">
        <span class="media-loading">{{ t('media.downloading') }}</span>
      </div>
      <div v-else-if="mediaStatus === 'cached' && contentUrl" class="media-action">
        <img :src="contentUrl" :alt="t('media.sticker')" class="media-sticker-img" @click="openContent" />
      </div>
      <div v-else-if="mediaStatus === 'failed'" class="media-error">
        {{ mediaError || t('media.downloadFailed') }}
      </div>
    </div>

    <!-- Video -->
    <div v-else-if="message.message_type === 'video'" class="media-video">
      <div v-if="mediaStatus === 'cached' && contentUrl" class="media-preview">
        <video :src="contentUrl" controls class="media-video-player" />
      </div>
      <div v-else class="media-placeholder" @click="handleDownload">
        <span class="media-icon-large">🎬</span>
        <div class="media-info">
          <span v-if="message.media?.duration">{{ formatDuration(message.media.duration) }}</span>
          <span v-if="message.media?.width"> · {{ message.media.width }}×{{ message.media.height }}</span>
        </div>
      </div>
      <div v-if="message.caption" class="media-caption">{{ message.caption }}</div>
      <div v-if="mediaStatus === 'none'" class="media-action">
        <button class="btn btn-sm btn-outline" @click="handleDownload">{{ t('media.download') }}</button>
      </div>
      <div v-else-if="mediaStatus === 'downloading'" class="media-action">
        <span class="media-loading">{{ t('media.downloading') }}</span>
      </div>
      <div v-else-if="mediaStatus === 'failed'" class="media-error">
        {{ mediaError || t('media.downloadFailed') }}
      </div>
    </div>

    <!-- Voice -->
    <div v-else-if="message.message_type === 'voice'" class="media-voice">
      <div class="media-icon">🎤</div>
      <div class="media-info">
        <span v-if="message.media?.duration">{{ formatDuration(message.media.duration) }}</span>
        <span v-else>{{ t('media.voiceMessage') }}</span>
      </div>
      <div v-if="mediaStatus === 'cached' && contentUrl" class="media-audio-player">
        <audio :src="contentUrl" controls />
      </div>
      <div v-else-if="mediaStatus === 'none'">
        <button class="btn btn-sm btn-outline" @click="handleDownload">{{ t('media.download') }}</button>
      </div>
      <div v-else-if="mediaStatus === 'downloading'">
        <span class="media-loading">{{ t('media.downloading') }}</span>
      </div>
      <div v-else-if="mediaStatus === 'failed'" class="media-error">
        {{ mediaError || t('media.downloadFailed') }}
      </div>
    </div>

    <!-- Audio -->
    <div v-else-if="message.message_type === 'audio'" class="media-audio">
      <div class="media-icon">🎵</div>
      <div class="media-info">
        <div class="media-filename">{{ message.media?.file_name || t('media.audio') }}</div>
        <div class="media-meta">
          {{ message.media?.duration ? formatDuration(message.media.duration) : '' }}
          {{ message.media?.size ? ' · ' + formatSize(message.media.size) : '' }}
        </div>
      </div>
      <div v-if="mediaStatus === 'cached' && contentUrl" class="media-audio-player">
        <audio :src="contentUrl" controls />
      </div>
      <div v-else-if="mediaStatus === 'none'">
        <button class="btn btn-sm btn-outline" @click="handleDownload">{{ t('media.download') }}</button>
      </div>
      <div v-else-if="mediaStatus === 'downloading'">
        <span class="media-loading">{{ t('media.downloading') }}</span>
      </div>
      <div v-else-if="mediaStatus === 'failed'" class="media-error">
        {{ mediaError || t('media.downloadFailed') }}
      </div>
    </div>

    <!-- Animation / Unsupported -->
    <div v-else class="media-unsupported">
      <div class="media-icon">📎</div>
      <div class="media-info">
        <div>{{ message.media?.file_name || t('media.unsupported') }}</div>
        <div v-if="message.media?.size" class="media-meta">{{ formatSize(message.media.size) }}</div>
      </div>
      <div v-if="mediaStatus === 'none'">
        <button class="btn btn-sm btn-outline" @click="handleDownload">{{ t('media.download') }}</button>
      </div>
      <div v-else-if="mediaStatus === 'downloading'">
        <span class="media-loading">{{ t('media.downloading') }}</span>
      </div>
      <div v-else-if="mediaStatus === 'cached' && contentUrl">
        <button class="btn btn-sm btn-primary" @click="openContent">{{ t('media.open') }}</button>
      </div>
      <div v-else-if="mediaStatus === 'failed'" class="media-error">
        {{ mediaError || t('media.downloadFailed') }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.media-preview {
  cursor: pointer;
  border-radius: 8px;
  overflow: hidden;
  max-width: 320px;
}
.media-img {
  max-width: 100%;
  max-height: 300px;
  display: block;
}
.media-video-player {
  max-width: 100%;
  max-height: 300px;
  border-radius: 8px;
}
.media-sticker-img {
  max-width: 120px;
  max-height: 120px;
  cursor: pointer;
}
.media-audio-player {
  margin-top: 8px;
}
.media-audio-player audio {
  width: 100%;
  max-width: 280px;
}
.media-placeholder {
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 16px;
}
.media-icon-large {
  font-size: 48px;
}
.media-action {
  margin-top: 8px;
}
.media-loading {
  font-size: 12px;
  color: var(--text-secondary);
  animation: pulse 1.5s infinite;
}
.media-error {
  font-size: 12px;
  color: var(--color-danger);
  margin-top: 4px;
}
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}
</style>
