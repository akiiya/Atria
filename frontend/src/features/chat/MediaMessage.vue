<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import type { ChatMessage } from '@/types/chat'
import { useI18n } from '@/i18n'
import { getMediaStatus, downloadMedia, getMediaContentUrl } from '@/api/media'
import ImageLightbox from './ImageLightbox.vue'

const MEDIA_TYPES = ['photo', 'document', 'video', 'voice', 'audio', 'sticker', 'animation']

const props = defineProps<{ message: ChatMessage }>()
const { t } = useI18n()
const queryClient = useQueryClient()

const messageId = computed(() => props.message.telegram_message_id || props.message.id)
const isMediaType = computed(() => MEDIA_TYPES.includes(props.message.message_type))

// 缓存状态查询。
// 走 TanStack Query 而非裸 fetch，这样滚动进出视口不会重复请求，
// 卸载时会自动取消，同一条消息的多个实例也会去重。
const statusQueryKey = computed(() => ['media-status', props.message.peer_ref, messageId.value])

const { data: statusData } = useQuery({
  queryKey: statusQueryKey,
  queryFn: () => getMediaStatus(messageId.value, props.message.peer_ref),
  enabled: computed(() => isMediaType.value && !!messageId.value),
  staleTime: 5 * 60_000,
  gcTime: 10 * 60_000,
  retry: false,
  refetchOnWindowFocus: false,
})

// 本地覆盖状态：下载过程中的瞬时状态（downloading / failed）
const localStatus = ref<'' | 'downloading' | 'failed'>('')
const mediaError = ref<string>('')
const lightboxVisible = ref(false)

// 切换到另一条消息时重置本地状态
watch(messageId, () => {
  localStatus.value = ''
  mediaError.value = ''
  lightboxVisible.value = false
})

const mediaStatus = computed<string>(() => {
  if (localStatus.value) return localStatus.value
  return statusData.value?.available ? 'cached' : 'none'
})

const contentUrl = computed(() =>
  mediaStatus.value === 'cached' && messageId.value
    ? getMediaContentUrl(messageId.value, props.message.peer_ref)
    : ''
)

// Telegram 随消息内嵌的极小预览图（data URI），无需额外请求即可立即展示。
const thumbnail = computed(() => props.message.media?.thumbnail || '')

// 用 Telegram 提供的原始宽高预留占位尺寸，避免图片加载时布局抖动。
// 占位框与最终图片使用同一 aspect-ratio，因此下载完成后不会跳动。
const PHOTO_MAX_W = 320
const PHOTO_MAX_H = 400

const photoBoxStyle = computed(() => {
  const w = props.message.media?.width
  const h = props.message.media?.height
  if (!w || !h) return undefined

  const scale = Math.min(PHOTO_MAX_W / w, PHOTO_MAX_H / h, 1)
  return {
    width: Math.round(w * scale) + 'px',
    height: Math.round(h * scale) + 'px',
    aspectRatio: `${w} / ${h}`,
  }
})

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

  localStatus.value = 'downloading'
  mediaError.value = ''

  try {
    const id = messageId.value
    const result = await downloadMedia(id, props.message.peer_ref)
    if (result.ok) {
      // 写入查询缓存，使 mediaStatus 变为 cached，并让其他实例共享结果
      queryClient.setQueryData(statusQueryKey.value, {
        ok: true,
        status: 'cached',
        available: true,
        file_name: result.file_name,
        mime_type: result.mime_type,
        file_size: result.file_size,
      })
      localStatus.value = ''
    } else {
      localStatus.value = 'failed'
      mediaError.value = result.message || t('media.downloadFailed')
    }
  } catch (e: unknown) {
    localStatus.value = 'failed'
    mediaError.value = e instanceof Error ? e.message : t('media.downloadFailed')
  }
}

async function handlePhotoClick() {
  if (mediaStatus.value === 'cached' && contentUrl.value) {
    lightboxVisible.value = true
  } else if (mediaStatus.value !== 'downloading') {
    await handleDownload()
    if (mediaStatus.value === 'cached' && contentUrl.value) {
      lightboxVisible.value = true
    }
  }
}

function handleStickerClick() {
  if (mediaStatus.value === 'cached' && contentUrl.value) {
    lightboxVisible.value = true
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
      <div v-if="mediaStatus === 'cached' && contentUrl" class="media-preview" @click="handlePhotoClick">
        <img
          :src="contentUrl"
          :alt="message.caption || t('media.photo')"
          :width="message.media?.width || undefined"
          :height="message.media?.height || undefined"
          :style="photoBoxStyle"
          class="media-img"
          loading="lazy"
          decoding="async"
        />
      </div>
      <!-- 未下载：优先展示 Telegram 内嵌的模糊缩略图，无缩略图时退回灰框 -->
      <div
        v-else-if="thumbnail"
        class="media-preview media-thumb-wrap"
        :style="photoBoxStyle"
        @click="handlePhotoClick"
      >
        <img
          :src="thumbnail"
          :alt="message.caption || t('media.photo')"
          class="media-img media-thumb"
          decoding="async"
        />
        <div class="media-thumb-overlay">
          <span v-if="mediaStatus === 'downloading'" class="media-thumb-spinner" />
          <span v-else class="media-thumb-badge">{{ t('media.view') }}</span>
        </div>
      </div>
      <div v-else class="media-placeholder" :style="photoBoxStyle" @click="handlePhotoClick">
        <span class="media-icon-large">🖼️</span>
        <div v-if="message.media?.width" class="media-meta">{{ message.media.width }}×{{ message.media.height }}</div>
      </div>
      <div v-if="message.caption" class="media-caption">{{ message.caption }}</div>
      <div v-if="mediaStatus === 'none'" class="media-action">
        <button class="btn btn-sm btn-outline" @click.stop="handleDownload">{{ t('media.download') }}</button>
      </div>
      <div v-else-if="mediaStatus === 'downloading'" class="media-action">
        <span class="media-loading">{{ t('media.downloading') }}</span>
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
      <div v-if="mediaStatus === 'cached' && contentUrl" @click="handleStickerClick" style="cursor:pointer;">
        <img
          :src="contentUrl"
          :alt="t('media.sticker')"
          class="media-sticker-img"
          loading="lazy"
          decoding="async"
        />
      </div>
      <div v-else>
        <div class="media-emoji">{{ message.media?.emoji || '🏷️' }}</div>
        <div class="media-note">{{ t('media.sticker') }}</div>
      </div>
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

    <!-- Video -->
    <div v-else-if="message.message_type === 'video'" class="media-video">
      <div v-if="mediaStatus === 'cached' && contentUrl" class="media-preview">
        <video
          :src="contentUrl"
          :width="message.media?.width || undefined"
          :height="message.media?.height || undefined"
          :style="photoBoxStyle"
          controls
          preload="metadata"
          playsinline
          class="media-video-player"
        />
      </div>
      <!-- 未下载：视频同样优先展示内嵌缩略图，叠加播放按钮 -->
      <div
        v-else-if="thumbnail"
        class="media-preview media-thumb-wrap"
        :style="photoBoxStyle"
        @click="handleDownload"
      >
        <img :src="thumbnail" :alt="t('media.video')" class="media-img media-thumb" decoding="async" />
        <div class="media-thumb-overlay">
          <span v-if="mediaStatus === 'downloading'" class="media-thumb-spinner" />
          <span v-else class="media-thumb-play">▶</span>
        </div>
        <div v-if="message.media?.duration" class="media-thumb-duration">
          {{ formatDuration(message.media.duration) }}
        </div>
      </div>
      <div v-else class="media-placeholder" :style="photoBoxStyle" @click="handleDownload">
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

    <!--
      Lightbox：仅在实际打开时挂载。
      此前条件是 v-if="contentUrl"，导致每条已缓存媒体都常驻一个 lightbox 实例，
      每个实例都会 Teleport 到 body 并注册 3 个 document 级监听器。
    -->
    <ImageLightbox
      v-if="lightboxVisible && contentUrl"
      :src="contentUrl"
      :alt="message.caption || t('media.photo')"
      :visible="lightboxVisible"
      @close="lightboxVisible = false"
    />
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

/* ── 内嵌缩略图（未下载时的模糊预览）── */
.media-thumb-wrap {
  position: relative;
  display: inline-block;
  background: var(--bg-tertiary);
}
.media-thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
  /* Telegram 内嵌缩略图只有几十像素，放大后本就模糊；
     轻微 blur 让边缘更柔和，避免看起来像加载失败的低清图 */
  filter: blur(4px);
  transform: scale(1.06); /* 抵消 blur 在边缘产生的透明羽化 */
}
.media-thumb-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.18);
  transition: background 0.15s;
}
.media-thumb-wrap:hover .media-thumb-overlay {
  background: rgba(0, 0, 0, 0.3);
}
.media-thumb-badge {
  padding: 5px 14px;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-size: 12px;
  font-weight: 500;
  backdrop-filter: blur(4px);
}
.media-thumb-play {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  padding-left: 4px; /* 视觉居中三角形 */
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-size: 18px;
  backdrop-filter: blur(4px);
}
.media-thumb-duration {
  position: absolute;
  right: 8px;
  bottom: 8px;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.65);
  color: #fff;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}
.media-thumb-spinner {
  width: 28px;
  height: 28px;
  border: 3px solid rgba(255, 255, 255, 0.35);
  border-top-color: #fff;
  border-radius: 50%;
  animation: media-thumb-spin 0.8s linear infinite;
}
@keyframes media-thumb-spin {
  to { transform: rotate(360deg); }
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
