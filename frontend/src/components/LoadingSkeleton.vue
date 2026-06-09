<script setup lang="ts">
defineProps<{ count?: number }>()
</script>

<template>
  <div class="skeleton-list">
    <div v-for="i in (count || 5)" :key="i" class="skeleton-item">
      <div class="skeleton-avatar"></div>
      <div class="skeleton-lines">
        <div class="skeleton-line long"></div>
        <div class="skeleton-line short"></div>
      </div>
    </div>
  </div>
</template>
