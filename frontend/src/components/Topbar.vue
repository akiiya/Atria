<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAccountStore } from '@/stores/account'
import { useAppStore } from '@/stores/app'
import { useQuery } from '@tanstack/vue-query'
import { fetchMe } from '@/api/me'
import { useI18n } from '@/i18n'

const router = useRouter()
const route = useRoute()

const account = useAccountStore()
const app = useAppStore()
const { t, locale, setLocale, locales } = useI18n()
const showAccountMenu = ref(false)
const showSettingsMenu = ref(false)
const showLangMenu = ref(false)

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

// Close menus on route change
watch(() => route.path, () => {
  showAccountMenu.value = false
  showSettingsMenu.value = false
  showLangMenu.value = false
})

function toggleAccountMenu() {
  showAccountMenu.value = !showAccountMenu.value
  showSettingsMenu.value = false
  showLangMenu.value = false
}

function toggleSettingsMenu() {
  showSettingsMenu.value = !showSettingsMenu.value
  showAccountMenu.value = false
  showLangMenu.value = false
}

function toggleLangMenu() {
  showLangMenu.value = !showLangMenu.value
  showAccountMenu.value = false
  showSettingsMenu.value = false
}

function switchAccount(id: number) {
  showAccountMenu.value = false
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
  if (confirm(t('topbar.logoutConfirm'))) {
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
  if (!target.closest('.lang-switcher')) showLangMenu.value = false
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    showAccountMenu.value = false
    showSettingsMenu.value = false
    showLangMenu.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', closeMenus)
  document.addEventListener('keydown', handleKeydown)
})
onUnmounted(() => {
  document.removeEventListener('click', closeMenus)
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <header class="topbar">
    <div class="topbar-left">
      <div class="account-switcher" v-if="meData?.ok">
        <div v-if="account.currentAccountId" class="account-current" @click="toggleAccountMenu" :title="t('topbar.switchAccount')">
          <span class="account-avatar">👤</span>
          <span class="account-name">{{ account.currentAccountDisplayName }}</span>
          <span v-if="meData?.current_account?.username" class="account-username">@{{ meData.current_account.username }}</span>
          <span class="account-arrow">▾</span>
        </div>
        <a v-else href="#" @click.prevent="router.push('/accounts/login')" class="account-empty" :title="t('topbar.connectAccount')">
          <span class="account-avatar">👤</span>
          <span class="account-name">{{ t('topbar.noAccount') }}</span>
        </a>

        <div :class="['account-dropdown', { show: showAccountMenu }]">
          <a v-for="acc in account.accountList" :key="acc.id"
             :class="['dropdown-item', { active: acc.id === account.currentAccountId }]"
             href="#" @click.prevent="switchAccount(acc.id)">
            <span class="dropdown-item-name">{{ acc.display_name }}</span>
            <span v-if="acc.username" class="dropdown-item-detail">@{{ acc.username }}</span>
          </a>
          <div class="dropdown-divider"></div>
          <a href="#" @click.prevent="showAccountMenu = false; router.push('/accounts/login')" class="dropdown-item">{{ t('topbar.addAccount') }}</a>
        </div>
      </div>
    </div>

    <div class="topbar-right">
      <div class="theme-switcher">
        <button class="theme-btn" :class="{ active: app.theme === 'light' }" @click="app.setTheme('light')" :title="t('topbar.themeLight')">☀️</button>
        <button class="theme-btn" :class="{ active: app.theme === 'dark' }" @click="app.setTheme('dark')" :title="t('topbar.themeDark')">🌙</button>
        <button class="theme-btn" :class="{ active: app.theme === 'system' }" @click="app.setTheme('system')" :title="t('topbar.themeSystem')">💻</button>
      </div>

      <div class="lang-switcher">
        <button class="theme-btn" @click="toggleLangMenu" title="Language">🌐</button>
        <div :class="['settings-dropdown', { show: showLangMenu }]">
          <a v-for="loc in locales" :key="loc.code"
             :class="['dropdown-item', { active: locale === loc.code }]"
             href="#" @click.prevent="setLocale(loc.code); showLangMenu = false">
            {{ loc.label }}
          </a>
        </div>
      </div>

      <div class="topbar-settings">
        <button class="topbar-settings-btn" @click="toggleSettingsMenu" :title="t('topbar.settings')">⚙️</button>
        <div :class="['settings-dropdown', { show: showSettingsMenu }]">
          <a href="#" @click.prevent="router.push('/settings')" class="dropdown-item">{{ t('topbar.settings') }}</a>
          <div class="dropdown-divider"></div>
          <a href="#" class="dropdown-item" @click.prevent="logout()">{{ t('topbar.logout') }}</a>
        </div>
      </div>

      <div class="topbar-user">
        <span class="user-badge">👤 {{ meData?.admin?.username || 'admin' }}</span>
      </div>
    </div>
  </header>
</template>
