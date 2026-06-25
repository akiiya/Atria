<script setup lang="ts">
import { useI18n } from '@/i18n'

defineProps<{ date: string }>()

const { t } = useI18n()

function formatDate(iso: string): string {
  const d = new Date(iso)
  const now = new Date()
  if (d.toDateString() === now.toDateString()) return t('chat.today')
  const yesterday = new Date(now)
  yesterday.setDate(yesterday.getDate() - 1)
  if (d.toDateString() === yesterday.toDateString()) return t('chat.yesterday')
  return d.toLocaleDateString([], { year: 'numeric', month: 'long', day: 'numeric' })
}
</script>

<template>
  <div class="date-divider">
    <span class="date-divider-text">{{ formatDate(date) }}</span>
  </div>
</template>
