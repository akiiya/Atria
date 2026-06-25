<script setup lang="ts">
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from '@/i18n'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const navItems: Array<{ path: string; icon: string; label: string; disabled?: boolean; badge?: string }> = [
  { path: '/dashboard', icon: '🏠', label: 'nav.dashboard' },
  { path: '/accounts', icon: '📱', label: 'nav.accounts' },
  { path: '/chats', icon: '💬', label: 'nav.chats' },
  { path: '/contacts', icon: '👥', label: 'nav.contacts' },
  { path: '/audit', icon: '📋', label: 'nav.audit' },
  { path: '/maintenance', icon: '🔧', label: 'nav.maintenance' },
]

function isActive(path: string): boolean {
  return route.path === path || route.path.startsWith(path + '/')
}

function navigate(path: string, disabled?: boolean) {
  if (!disabled) router.push(path)
}
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-brand">
      <div class="brand-mark" aria-hidden="true">
        <svg width="28" height="28" viewBox="0 0 28 28" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M14 2L24 24H21L18.5 19H9.5L7 24H4L14 2ZM11 17H17L14 8L11 17Z" fill="url(#brand-gradient)"/>
          <circle cx="14" cy="22" r="2.5" fill="url(#brand-gradient)" opacity="0.6"/>
          <defs>
            <linearGradient id="brand-gradient" x1="4" y1="2" x2="24" y2="24" gradientUnits="userSpaceOnUse">
              <stop stop-color="#60a5fa"/>
              <stop offset="1" stop-color="#a78bfa"/>
            </linearGradient>
          </defs>
        </svg>
      </div>
      <div class="brand-name">Atria</div>
    </div>
    <nav class="sidebar-nav">
      <div class="nav-section">
        <div class="nav-section-title">{{ t('nav.features') }}</div>
        <a
          v-for="item in navItems"
          :key="item.path"
          :class="['nav-item', { active: isActive(item.path), disabled: item.disabled }]"
          :title="item.disabled ? '即将支持' : ''"
          @click.prevent="navigate(item.path, item.disabled)"
          href="#"
        >
          <span class="nav-icon">{{ item.icon }}</span>
          <span class="nav-label">{{ t(item.label) }}</span>
          <span v-if="item.badge" class="nav-badge">{{ item.badge }}</span>
        </a>
      </div>
    </nav>
    <div class="sidebar-footer">
      <div class="sidebar-version">v{{ $attrs.version || '0.1.0-dev' }}</div>
    </div>
  </aside>
</template>
