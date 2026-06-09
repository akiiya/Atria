<script setup lang="ts">
import { useRouter, useRoute } from 'vue-router'

const router = useRouter()
const route = useRoute()

const navItems = [
  { path: '/dashboard', icon: '🏠', label: '仪表盘' },
  { path: '/chats', icon: '💬', label: '聊天' },
  { path: '/accounts', icon: '📱', label: '账号会话' },
]

function navigate(path: string) {
  router.push(path)
}
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-brand">
      <div class="brand-mark" aria-hidden="true">▲</div>
      <div class="brand-name">Atria</div>
    </div>
    <nav class="sidebar-nav">
      <div class="nav-section">
        <div class="nav-section-title">功能</div>
        <a
          v-for="item in navItems"
          :key="item.path"
          :class="['nav-item', { active: route.path.startsWith(item.path) }]"
          @click.prevent="navigate(item.path)"
          href="#"
        >
          <span class="nav-icon">{{ item.icon }}</span>
          <span class="nav-label">{{ item.label }}</span>
        </a>
      </div>
    </nav>
  </aside>
</template>
