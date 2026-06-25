<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'

const props = defineProps<{
  src: string
  alt?: string
  visible: boolean
}>()

const emit = defineEmits<{ close: [] }>()

const scale = ref(1)
const translateX = ref(0)
const translateY = ref(0)
const isDragging = ref(false)
const dragStartX = ref(0)
const dragStartY = ref(0)
const dragStartTranslateX = ref(0)
const dragStartTranslateY = ref(0)
const show = ref(false)

watch(() => props.visible, (val) => {
  if (val) {
    // 延迟一帧显示，触发 CSS transition
    requestAnimationFrame(() => { show.value = true })
  } else {
    show.value = false
  }
})

function resetTransform() {
  scale.value = 1
  translateX.value = 0
  translateY.value = 0
}

function close() {
  show.value = false
  setTimeout(() => {
    resetTransform()
    emit('close')
  }, 200)
}

function handleWheel(e: WheelEvent) {
  e.preventDefault()
  const delta = e.deltaY > 0 ? -0.15 : 0.15
  const newScale = Math.max(0.2, Math.min(10, scale.value + delta))
  scale.value = newScale
}

function handleMousedown(e: MouseEvent) {
  if (e.button !== 0) return
  isDragging.value = true
  dragStartX.value = e.clientX
  dragStartY.value = e.clientY
  dragStartTranslateX.value = translateX.value
  dragStartTranslateY.value = translateY.value
  e.preventDefault()
}

function handleMousemove(e: MouseEvent) {
  if (!isDragging.value) return
  translateX.value = dragStartTranslateX.value + (e.clientX - dragStartX.value)
  translateY.value = dragStartTranslateY.value + (e.clientY - dragStartY.value)
}

function handleMouseup() {
  isDragging.value = false
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}

function zoomIn() {
  scale.value = Math.min(10, scale.value + 0.3)
}

function zoomOut() {
  scale.value = Math.max(0.2, scale.value - 0.3)
}

function zoomReset() {
  resetTransform()
}

function handleDownload() {
  const a = document.createElement('a')
  a.href = props.src
  a.download = props.alt || 'image'
  a.target = '_blank'
  a.click()
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
  document.addEventListener('mousemove', handleMousemove)
  document.addEventListener('mouseup', handleMouseup)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
  document.removeEventListener('mousemove', handleMousemove)
  document.removeEventListener('mouseup', handleMouseup)
})
</script>

<template>
  <Teleport to="body">
    <Transition name="lightbox">
      <div v-if="visible" class="lightbox-overlay" @click.self="close">
        <div class="lightbox-toolbar">
          <button class="lightbox-btn" title="缩小" @click="zoomOut">➖</button>
          <span class="lightbox-zoom">{{ Math.round(scale * 100) }}%</span>
          <button class="lightbox-btn" title="放大" @click="zoomIn">➕</button>
          <button class="lightbox-btn" title="重置" @click="zoomReset">↺</button>
          <button class="lightbox-btn" title="下载" @click="handleDownload">⬇</button>
          <button class="lightbox-btn lightbox-close" title="关闭 (Esc)" @click="close">✕</button>
        </div>
        <div
          class="lightbox-viewport"
          @wheel.prevent="handleWheel"
          @mousedown="handleMousedown"
        >
          <img
            :src="src"
            :alt="alt || ''"
            class="lightbox-img"
            :style="{
              transform: `translate(${translateX}px, ${translateY}px) scale(${scale})`,
              cursor: isDragging ? 'grabbing' : 'grab',
            }"
            draggable="false"
          />
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.lightbox-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: rgba(0, 0, 0, 0.85);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(4px);
}

.lightbox-toolbar {
  position: absolute;
  top: 16px;
  right: 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  z-index: 10;
  background: rgba(0, 0, 0, 0.5);
  border-radius: 8px;
  padding: 6px 12px;
}

.lightbox-btn {
  background: none;
  border: none;
  color: #fff;
  font-size: 18px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  transition: background 0.15s;
  line-height: 1;
}

.lightbox-btn:hover {
  background: rgba(255, 255, 255, 0.15);
}

.lightbox-close {
  font-size: 20px;
  margin-left: 4px;
}

.lightbox-zoom {
  color: #fff;
  font-size: 13px;
  min-width: 40px;
  text-align: center;
  user-select: none;
}

.lightbox-viewport {
  flex: 1;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  user-select: none;
}

.lightbox-img {
  max-width: 90vw;
  max-height: 85vh;
  object-fit: contain;
  transition: transform 0.1s ease-out;
  will-change: transform;
  pointer-events: auto;
}

/* 动效 */
.lightbox-enter-active {
  transition: opacity 0.2s ease;
}
.lightbox-leave-active {
  transition: opacity 0.15s ease;
}
.lightbox-enter-from,
.lightbox-leave-to {
  opacity: 0;
}
.lightbox-enter-active .lightbox-img {
  animation: lightbox-zoom-in 0.25s ease-out;
}
.lightbox-leave-active .lightbox-img {
  animation: lightbox-zoom-out 0.15s ease-in;
}

@keyframes lightbox-zoom-in {
  from { transform: scale(0.7); opacity: 0; }
  to { transform: scale(1); opacity: 1; }
}

@keyframes lightbox-zoom-out {
  from { transform: scale(1); opacity: 1; }
  to { transform: scale(0.8); opacity: 0; }
}
</style>
