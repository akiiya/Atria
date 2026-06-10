<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import Sidebar from './Sidebar.vue'
import Topbar from './Topbar.vue'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const route = useRoute()

// Chat pages need full-width layout (no padding, no scroll)
const isFullWidth = computed(() => route.path.startsWith('/chats'))
</script>

<template>
  <div class="app-layout" :class="{ 'sidebar-collapsed': app.sidebarCollapsed }">
    <Sidebar />
    <div class="app-main">
      <Topbar />
      <div class="app-content" :class="{ 'page-full': isFullWidth }">
        <router-view />
      </div>
    </div>
  </div>
</template>
