<script setup lang="ts">
defineProps<{
  icon?: string
  title: string
  description?: string
  /** 尺寸：small 用于内联空态，large 用于整页空态 */
  size?: 'small' | 'large'
}>()
</script>

<template>
  <div :class="['empty-state', size === 'large' ? 'empty-state-lg' : 'empty-state-sm']">
    <div class="empty-icon">{{ icon || '💬' }}</div>
    <div class="empty-title">{{ title }}</div>
    <div v-if="description" class="empty-desc">{{ description }}</div>
    <slot />
  </div>
</template>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 32px 24px;
  animation: empty-fade-in 0.3s ease-out;
}

@keyframes empty-fade-in {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.empty-state-sm {
  padding: 24px 16px;
}

.empty-state-lg {
  padding: 48px 24px;
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 16px;
  opacity: 0.8;
}

.empty-state-lg .empty-icon {
  font-size: 64px;
  margin-bottom: 20px;
}

.empty-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.empty-state-lg .empty-title {
  font-size: 20px;
}

.empty-desc {
  font-size: 14px;
  color: var(--text-secondary);
  max-width: 320px;
  line-height: 1.5;
}

.empty-state-lg .empty-desc {
  font-size: 15px;
  max-width: 400px;
}
</style>
