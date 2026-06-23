<script setup lang="ts">
import { computed, ref } from 'vue'

const props = defineProps<{
  text?: string
  size?: number
  avatarUrl?: string
}>()

const imgError = ref(false)

const showImage = computed(() => props.avatarUrl && !imgError.value)

/**
 * 从文本中安全提取首个 grapheme cluster（不拆 emoji / surrogate pair / ZWJ）。
 */
const initialChar = computed(() => {
  const t = props.text || ''
  if (!t) return '?'
  // 优先使用 Intl.Segmenter
  const IntlWithSegmenter = Intl as unknown as { Segmenter?: new (locale: string, opts: { granularity: string }) => { segment: (text: string) => IterableIterator<{ segment: string }> } }
  if (IntlWithSegmenter.Segmenter) {
    const segmenter = new IntlWithSegmenter.Segmenter('zh', { granularity: 'grapheme' })
    const first = segmenter.segment(t)[Symbol.iterator]().next()
    if (!first.done && first.value) return first.value.segment
  }
  // fallback: Array.from 按 code point
  const chars = Array.from(t)
  return chars[0] || '?'
})

const pixelSize = computed(() => props.size || 40)
const fontSize = computed(() => Math.max(12, pixelSize.value * 0.4))
</script>

<template>
  <div
    class="avatar-initials"
    :style="{ width: pixelSize + 'px', height: pixelSize + 'px', fontSize: fontSize + 'px' }"
  >
    <img
      v-if="showImage"
      :src="avatarUrl"
      :alt="text || ''"
      class="avatar-img"
      @error="imgError = true"
    />
    <span v-else class="avatar-char">{{ initialChar }}</span>
  </div>
</template>

<style scoped>
.avatar-initials {
  border-radius: 50%;
  background: var(--accent-color);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  flex-shrink: 0;
  overflow: hidden;
  position: relative;
}
.avatar-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 50%;
}
.avatar-char {
  line-height: 1;
  /* 防止 emoji 宽度撑开头像 */
  max-width: 100%;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
