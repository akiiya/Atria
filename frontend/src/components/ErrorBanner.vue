<script setup lang="ts">
import { useI18n } from '@/i18n'

const { t } = useI18n()

withDefaults(defineProps<{
  message: string
  /** 是否显示重试按钮 */
  retryable?: boolean
}>(), {
  retryable: false,
})

defineEmits<{ dismiss: []; retry: [] }>()
</script>

<template>
  <div class="alert alert-error" role="alert">
    <span class="alert-message">{{ message }}</span>
    <div class="alert-actions">
      <button
        v-if="retryable"
        type="button"
        class="alert-retry"
        @click="$emit('retry')"
      >{{ t('common.retry') }}</button>
      <button
        type="button"
        class="alert-dismiss"
        :aria-label="t('common.cancel')"
        @click="$emit('dismiss')"
      >✕</button>
    </div>
  </div>
</template>
