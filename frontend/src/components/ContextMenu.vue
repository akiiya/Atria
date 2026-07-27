<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

export interface ContextMenuItem {
  label: string
  icon?: string
  action: () => void
  disabled?: boolean
  danger?: boolean
}

const props = defineProps<{
  items: ContextMenuItem[]
  x: number
  y: number
}>()

const emit = defineEmits<{ close: [] }>()

const menuRef = ref<HTMLElement | null>(null)

// 确保菜单不超出视口
const adjustedX = ref(props.x)
const adjustedY = ref(props.y)

onMounted(() => {
  if (menuRef.value) {
    const rect = menuRef.value.getBoundingClientRect()
    const padding = 8
    if (props.x + rect.width > window.innerWidth - padding) {
      adjustedX.value = window.innerWidth - rect.width - padding
    }
    if (props.y + rect.height > window.innerHeight - padding) {
      adjustedY.value = window.innerHeight - rect.height - padding
    }
  }
  // 点击外部关闭
  document.addEventListener('click', handleClickOutside)
  document.addEventListener('contextmenu', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('contextmenu', handleClickOutside)
})

function handleClickOutside(e: Event) {
  if (menuRef.value && !menuRef.value.contains(e.target as Node)) {
    emit('close')
  }
}

function handleItemClick(item: ContextMenuItem) {
  if (!item.disabled) {
    item.action()
    emit('close')
  }
}
</script>

<template>
  <Teleport to="body">
    <div
      ref="menuRef"
      class="context-menu"
      :style="{ left: adjustedX + 'px', top: adjustedY + 'px' }"
      @click.stop
    >
      <button
        v-for="(item, idx) in items"
        :key="idx"
        :class="['context-menu-item', { danger: item.danger, disabled: item.disabled }]"
        @click="handleItemClick(item)"
      >
        <span v-if="item.icon" class="context-menu-icon">{{ item.icon }}</span>
        <span class="context-menu-label">{{ item.label }}</span>
      </button>
    </div>
  </Teleport>
</template>

<style scoped>
.context-menu {
  position: fixed;
  z-index: 10000;
  min-width: 160px;
  max-width: 280px;
  padding: 4px 0;
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
  animation: context-menu-enter 0.12s ease-out;
}

@keyframes context-menu-enter {
  from {
    opacity: 0;
    transform: scale(0.95);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

.context-menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 14px;
  border: none;
  background: transparent;
  color: var(--text-primary);
  font-size: 14px;
  font-family: var(--font-sans);
  cursor: pointer;
  transition: background 0.1s;
  text-align: left;
}

.context-menu-item:hover {
  background: var(--bg-secondary);
}

.context-menu-item.danger {
  color: var(--color-danger);
}

.context-menu-item.danger:hover {
  background: var(--color-danger-light);
}

.context-menu-item.disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.context-menu-item.disabled:hover {
  background: transparent;
}

.context-menu-icon {
  flex-shrink: 0;
  font-size: 16px;
  width: 20px;
  text-align: center;
}

.context-menu-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
