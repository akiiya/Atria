import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAppStore = defineStore('app', () => {
  const theme = ref<'light' | 'dark' | 'system'>('system')
  const sidebarCollapsed = ref(false)
  const mobilePanelMode = ref<'list' | 'chat'>('list')

  function setTheme(t: 'light' | 'dark' | 'system') {
    theme.value = t
    document.documentElement.setAttribute('data-theme', t)
  }

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  return { theme, sidebarCollapsed, mobilePanelMode, setTheme, toggleSidebar }
})
