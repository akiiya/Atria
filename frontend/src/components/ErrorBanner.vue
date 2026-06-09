<script setup lang="ts">
defineProps<{ message: string }>()
defineEmits<{ dismiss: [] }>()
</script>

<template>
  <div class="alert alert-error">
    <span>{{ message }}</span>
    <button class="alert-dismiss" @click="$emit('dismiss')">✕</button>
  </div>
</template>
