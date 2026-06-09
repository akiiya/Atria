<script setup lang="ts">
import Sidebar from './Sidebar.vue'
import Topbar from './Topbar.vue'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
</script>

<template>
  <div class="app-layout" :class="{ 'sidebar-collapsed': app.sidebarCollapsed }">
    <Sidebar />
    <div class="app-main">
      <Topbar />
      <div class="app-content">
        <router-view />
      </div>
    </div>
  </div>
</template>
