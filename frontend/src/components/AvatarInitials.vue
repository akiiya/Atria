<script setup lang="ts">
defineProps<{ text: string; size?: number }>()
</script>

<template>
  <div class="avatar-initials" :style="{ width: (size || 40) + 'px', height: (size || 40) + 'px' }">
    {{ (text || '?').charAt(0).toUpperCase() }}
  </div>
</template>
