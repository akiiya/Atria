<script setup lang="ts">
import { useAccountStore } from '@/stores/account'
import { useAppStore } from '@/stores/app'

const account = useAccountStore()
const app = useAppStore()
</script>

<template>
  <header class="topbar">
    <div class="topbar-left">
      <button class="topbar-menu-btn" @click="app.toggleSidebar()">☰</button>
      <div class="topbar-title">
        <span v-if="account.currentAccountDisplayName" class="topbar-account">
          {{ account.currentAccountDisplayName }}
        </span>
        <span v-else class="topbar-account muted">未接入账号</span>
      </div>
    </div>
    <div class="topbar-right">
      <span class="topbar-user">👤 admin</span>
    </div>
  </header>
</template>
