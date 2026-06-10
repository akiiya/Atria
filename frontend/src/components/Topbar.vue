<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAccountStore } from '@/stores/account'
import { useAppStore } from '@/stores/app'
import { useQuery } from '@tanstack/vue-query'
import { fetchMe } from '@/api/me'

const router = useRouter()

const account = useAccountStore()
const app = useAppStore()
const showAccountMenu = ref(false)
const showSettingsMenu = ref(false)

const { data: meData } = useQuery({
  queryKey: ['me'],
  queryFn: fetchMe,
  retry: 1,
})

// Reactively sync account store when meData changes
watch(meData, (val) => {
  if (val?.ok) {
    if (val.current_account) {
      account.setCurrent(val.current_account)
    } else {
      account.setCurrent(null)
    }
    if (val.accounts) {
      account.setAccounts(val.accounts)
    }
  }
}, { immediate: true })

function toggleAccountMenu() {
  showAccountMenu.value = !showAccountMenu.value
  showSettingsMenu.value = false
}

function toggleSettingsMenu() {
  showSettingsMenu.value = !showSettingsMenu.value
  showAccountMenu.value = false
}

function switchAccount(id: number) {
  const form = document.createElement('form')
  form.method = 'POST'
  form.action = '/accounts/select'
  form.style.display = 'none'

  const csrfInput = document.createElement('input')
  csrfInput.type = 'hidden'
  csrfInput.name = 'csrf_token'
  csrfInput.value = document.querySelector('meta[name="csrf-token"]')?.getAttribute('content') || ''
  form.appendChild(csrfInput)

  const idInput = document.createElement('input')
  idInput.type = 'hidden'
  idInput.name = 'account_id'
  idInput.value = String(id)
  form.appendChild(idInput)

  document.body.appendChild(form)
  form.submit()
}

function logout() {
  if (confirm('确定要退出登录吗？')) {
    const form = document.createElement('form')
    form.method = 'POST'
    form.action = '/logout'
    form.style.display = 'none'

    const csrfInput = document.createElement('input')
    csrfInput.type = 'hidden'
    csrfInput.name = 'csrf_token'
    csrfInput.value = document.querySelector('meta[name="csrf-token"]')?.getAttribute('content') || ''
    form.appendChild(csrfInput)

    document.body.appendChild(form)
    form.submit()
  }
}

function closeMenus(e: Event) {
  const target = e.target as HTMLElement
  if (!target.closest('.account-switcher')) showAccountMenu.value = false
  if (!target.closest('.topbar-settings')) showSettingsMenu.value = false
}

onMounted(() => document.addEventListener('click', closeMenus))
onUnmounted(() => document.removeEventListener('click', closeMenus))
</script>

<template>
  <header class="topbar">
    <div class="topbar-left">
      <div class="account-switcher" v-if="meData?.ok">
        <div v-if="account.currentAccountId" class="account-current" @click="toggleAccountMenu" title="切换账号">
          <span class="account-avatar">👤</span>
          <span class="account-name">{{ account.currentAccountDisplayName }}</span>
          <span v-if="meData?.current_account?.username" class="account-username">@{{ meData.current_account.username }}</span>
          <span class="account-arrow">▾</span>
        </div>
        <a v-else href="#" @click.prevent="router.push('/accounts/login')" class="account-empty" title="接入账号">
          <span class="account-avatar">👤</span>
          <span class="account-name">未接入账号</span>
        </a>

        <div :class="['account-dropdown', { show: showAccountMenu }]">
          <a v-for="acc in account.accountList" :key="acc.id"
             :class="['dropdown-item', { active: acc.id === account.currentAccountId }]"
             href="#" @click.prevent="switchAccount(acc.id)">
            <span class="dropdown-item-name">{{ acc.display_name }}</span>
            <span v-if="acc.username" class="dropdown-item-detail">@{{ acc.username }}</span>
          </a>
          <div class="dropdown-divider"></div>
          <a href="#" @click.prevent="router.push('/accounts/login')" class="dropdown-item">接入新账号</a>
        </div>
      </div>
    </div>

    <div class="topbar-right">
      <div class="theme-switcher">
        <button class="theme-btn" :class="{ active: app.theme === 'light' }" @click="app.setTheme('light')" title="浅色模式">☀️</button>
        <button class="theme-btn" :class="{ active: app.theme === 'dark' }" @click="app.setTheme('dark')" title="深色模式">🌙</button>
        <button class="theme-btn" :class="{ active: app.theme === 'system' }" @click="app.setTheme('system')" title="跟随系统">💻</button>
      </div>

      <div class="topbar-settings">
        <button class="topbar-settings-btn" @click="toggleSettingsMenu" title="设置">⚙️</button>
        <div :class="['settings-dropdown', { show: showSettingsMenu }]">
          <a href="#" @click.prevent="router.push('/settings')" class="dropdown-item">系统设置</a>
          <div class="dropdown-divider"></div>
          <a href="#" class="dropdown-item" @click.prevent="logout()">退出登录</a>
        </div>
      </div>

      <div class="topbar-user">
        <span class="user-badge">👤 {{ meData?.admin?.username || 'admin' }}</span>
      </div>
    </div>
  </header>
</template>
