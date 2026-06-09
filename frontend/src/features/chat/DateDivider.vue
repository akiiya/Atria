<script setup lang="ts">
defineProps<{ date: string }>()

function formatDate(iso: string): string {
  const d = new Date(iso)
  const now = new Date()
  if (d.toDateString() === now.toDateString()) return '今天'
  const yesterday = new Date(now)
  yesterday.setDate(yesterday.getDate() - 1)
  if (d.toDateString() === yesterday.toDateString()) return '昨天'
  return d.toLocaleDateString([], { year: 'numeric', month: 'long', day: 'numeric' })
}
</script>

<template>
  <div class="date-divider">
    <span class="date-divider-text">{{ formatDate(date) }}</span>
  </div>
</template>
